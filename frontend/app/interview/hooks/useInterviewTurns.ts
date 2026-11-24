'use client'

import { useRef, type Dispatch, type RefObject, type SetStateAction } from 'react'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService, User } from '@/lib/auth'
import { interviewLimits, InterviewSession } from '@/lib/interview'
import { extractApiErrorMessage, parseMediaError, parseMultipartResponse } from '@/lib/interview/utils'
import { extFromMimeType, type RecorderFormat } from '../lib/recorderFormat'
import { buildCompanyInfo } from '../utils'
import { applyCompanyReadingResponse } from '../companyReadingContext'
import { nextQuestionIndex } from '../questionEvents'
import { fetchAndReadWithTimeout } from '@/lib/fetch-timeout'
import type { Utterance, InterviewCompany, Position } from '../types'

/**
 * ターン(/turn, /start-turn)のタイムアウト(#1476)。
 * STT→LLM→TTS を通すので一覧系より長く取るが、無期限にはしない。
 * 半開きのまま固まると、ターンが終わらないので面接が進まず、
 * 終了時に「応答待ちのターン」を待つ処理（handleStop）も戻らなくなる。
 * ヘッダー受信後に本文（multipart の音声）で止まる場合も同じなので、
 * fetchWithTimeout ではなく本文の読み込みまで見る fetchAndReadWithTimeout を使う。
 *
 * ターンはこの通信の前に authService.ensureFreshUserToken() を待つ。そちらにも
 * AUTH_REFRESH_TIMEOUT_MS（15秒）が掛かっているので、ターン1回の上限は
 * 15 + 90 = 105秒。どこで固まっても有限時間で戻る（#1501）。
 */
const TURN_FETCH_TIMEOUT_MS = 90_000

type UseInterviewTurnsArgs = {
  user: User | null
  session: InterviewSession | null
  interviewCompany: InterviewCompany | null
  companyQuestionHints: string[]
  selectedPosition: Position
  remainingSeconds: number
  currentQuestionIndex: number
  totalQuestionCount: number
  questionElapsedSeconds: number
  setCurrentQuestionIndex: Dispatch<SetStateAction<number>>
  setTotalQuestionCount: Dispatch<SetStateAction<number>>
  setQuestionElapsedSeconds: Dispatch<SetStateAction<number>>
  setIsDeepeningQuestion: Dispatch<SetStateAction<boolean>>
  setQuestionCategory: Dispatch<SetStateAction<string | null>>
  questionStartedAtRef: RefObject<number | null>
  audioChunksRef: RefObject<Blob[]>
  recorderFormatRef: RefObject<RecorderFormat>
  setTurnPending: (v: boolean) => void
  audioGenerationRef: RefObject<number>
  playAudioBlob: (blob: Blob, gen: number) => Promise<void>
  setUtterances: Dispatch<SetStateAction<Utterance[]>>
  setErrorMessage: Dispatch<SetStateAction<string | null>>
  setUtteranceSaveFailed: Dispatch<SetStateAction<boolean>>
}

/**
 * 面接のターン（冒頭の /start-turn と回答の /turn）の送信・応答反映・音声再生。
 * 応答待ちのターンは inFlightTurnRef に束ね、終了処理（handleStop）が待つ。
 */
export function useInterviewTurns({
  user,
  session,
  interviewCompany,
  companyQuestionHints,
  selectedPosition,
  remainingSeconds,
  currentQuestionIndex,
  totalQuestionCount,
  questionElapsedSeconds,
  setCurrentQuestionIndex,
  setTotalQuestionCount,
  setQuestionElapsedSeconds,
  setIsDeepeningQuestion,
  setQuestionCategory,
  questionStartedAtRef,
  audioChunksRef,
  recorderFormatRef,
  setTurnPending,
  audioGenerationRef,
  playAudioBlob,
  setUtterances,
  setErrorMessage,
  setUtteranceSaveFailed,
}: UseInterviewTurnsArgs) {
  /**
   * 応答待ちのターン(#1476)。
   * 「完了する」は turnPending 中でも押せるし、時間切れの強制終了も割り込む。
   * 終了処理が保存チェーンを読んだ後に /turn の応答が届くと、その発話は
   * finishSession の後に保存され、レポートが最後のやり取り抜きで作られる。
   * そこで終了処理はまずこれを待ってからチェーンを読む。
   * 解決するのは「応答を保存チェーンに積み終えた時点」までで、音声再生は含めない
   * （終了時は cleanupConnection が再生を止めるため、再生まで待つと解決しない）。
   */
  const inFlightTurnRef = useRef<Promise<void>>(Promise.resolve())
  /**
   * /start-turn の応答(#1969)。サーバーは冒頭の質問を応答の直前に保存するので、/turn はこれを
   * 待ってから送る。先に送ると、回答と次の質問が冒頭の質問より前に保存され、面接官の履歴からも
   * 冒頭の質問が抜ける。/start-turn はタイムアウト付きなので、待ちは必ず終わる。
   */
  const startTurnSettledRef = useRef<Promise<void>>(Promise.resolve())
  const historyRef = useRef<{ role: string; content: string }[]>([])
  const companyContextRef = useRef({ reading: '', readingResolved: false })
  const companyContextGenerationRef = useRef(0)

  /**
   * ターンの応答処理（発話を保存チェーンへ積み終えるまで）を「実行中のターン」として記録する(#1476)。
   * handleStop はこれを待ってから保存チェーンを読む。
   * 失敗しても終了処理を止めないよう、記録する側の Promise は必ず解決させる。
   *
   * 代入で上書きせず、実行中の全ターンを束ねる。handleJoin は /start-turn の完了前に
   * 状態を connected にするので、その応答待ち中にユーザーが録音して /turn を始められる。
   * 上書きすると handleStop は最後の1件しか待たず、遅れて返った /start-turn の質問が
   * finishSession の後に保存され、質問を欠いた書き起こしでレポートが確定する。
   */
  const trackTurn = (receiving: Promise<Blob>): Promise<Blob> => {
    const settled = receiving.then(() => {}, () => {})
    inFlightTurnRef.current = Promise.all([inFlightTurnRef.current, settled]).then(() => {})
    return receiving
  }

  /**
   * ターンの応答を待って音声を再生する(#1501)。
   * 音声世代はリクエスト開始時点（＝この関数に入った時点。receive() は既に呼ばれているが、
   * 間に cleanupConnection が割り込む隙は無い）で取り、playAudioBlob へ引き継ぐ。
   * 応答待ちの間に面接が終了していれば再生しない。/turn と /start-turn の両方をここに通す。
   */
  const playTurnAudio = async (receiving: Promise<Blob>): Promise<void> => {
    const gen = audioGenerationRef.current
    await playAudioBlob(await trackTurn(receiving), gen)
  }

  const doStartTurn = async (sessionId: number, userId: number) => {
    const companyContextGeneration = companyContextGenerationRef.current
    const receive = async (): Promise<Blob> => {
      await authService.ensureFreshUserToken()
      const { meta, audio } = await fetchAndReadWithTimeout(`${BACKEND_URL}/api/interviews/${sessionId}/start-turn`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({
          user_id: userId,
          company_name: interviewCompany?.name || '',
          company_reading: interviewCompany?.name_reading || '',
          position: selectedPosition?.title || '',
          company_info: buildCompanyInfo(interviewCompany),
          company_id: interviewCompany?.id || 0,
          company_type: selectedPosition?.category || 'general',
          question_index: 1,
          total_questions: totalQuestionCount,
          question_elapsed_seconds: 0,
          question_duration_seconds: Math.max(60, interviewLimits.questionDurationSeconds || 180),
          question_hints: companyQuestionHints.slice(0, 5).map((question) => question.slice(0, 300)),
        }),
      }, TURN_FETCH_TIMEOUT_MS, async res => {
        if (!res.ok) throw new Error(extractApiErrorMessage(await res.text()))
        return parseMultipartResponse(res)
      })
      if (companyContextGeneration !== companyContextGenerationRef.current) return audio
      companyContextRef.current = applyCompanyReadingResponse(
        companyContextRef.current,
        meta,
        companyContextGeneration,
        companyContextGenerationRef.current,
      )
      const aiText: string = meta.ai_text || ''
      const responseQuestionTotal = Number(meta.total_questions)
      if (Number.isSafeInteger(responseQuestionTotal) && responseQuestionTotal > 0) {
        setTotalQuestionCount(responseQuestionTotal)
      }
      if (typeof meta.question_source === 'string' && meta.question_source.trim() !== '') {
        setCurrentQuestionIndex(1)
        questionStartedAtRef.current = Date.now()
        setQuestionElapsedSeconds(0)
      }
      setIsDeepeningQuestion(Boolean(meta.is_deepening))
      setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
      // 書き起こしはサーバーが保存済み(#1969)。ここでは表示だけを更新する。
      if (String(meta.utterance_save_failed) === 'true') setUtteranceSaveFailed(true)
      if (aiText) {
        historyRef.current.push({ role: 'assistant', content: aiText })
        setUtterances(p => [...p, { role: 'ai', text: aiText }])
      }
      return audio
    }
    const receiving = receive()
    startTurnSettledRef.current = receiving.then(() => {}, () => {})
    await playTurnAudio(receiving)
  }

  const sendTurn = async () => {
    if (!user || !session) { setTurnPending(false); return }
    const chunks = audioChunksRef.current
    if (chunks.length === 0) { setTurnPending(false); return }
    // 実際に録れた形式で送る。Blob の type と拡張子が実体とずれると、
    // 受け側の形式判定とファイル名が食い違う。
    const format = recorderFormatRef.current
    const blobType = chunks[0]?.type || format.mimeType || 'audio/webm'
    const ext = extFromMimeType(blobType) || format.ext
    const audioBlob = new Blob(chunks, { type: blobType })
    const formData = new FormData()
    formData.append('audio', audioBlob, `audio.${ext}`)
    formData.append('user_id', String(user.user_id))
    formData.append('turn_count', String(Math.floor(historyRef.current.length / 2) + 1))
    formData.append('remaining_seconds', String(remainingSeconds))
    formData.append('question_index', String(currentQuestionIndex))
    formData.append('total_questions', String(totalQuestionCount))
    formData.append('question_elapsed_seconds', String(questionElapsedSeconds))
    formData.append('question_duration_seconds', String(Math.max(60, interviewLimits.questionDurationSeconds || 180)))
    formData.append('company_name', interviewCompany?.name || '')
    formData.append('company_reading', companyContextRef.current.reading || interviewCompany?.name_reading || '')
    formData.append('company_reading_resolved', String(
      companyContextRef.current.readingResolved || Boolean(interviewCompany?.name_reading),
    ))
    formData.append('position', selectedPosition?.title || '')
    formData.append('company_info', buildCompanyInfo(interviewCompany))
    formData.append('company_type', selectedPosition?.category || 'general')
    formData.append('company_id', String(interviewCompany?.id || 0))
    formData.append(
      'question_hints',
      JSON.stringify(companyQuestionHints.slice(0, 5).map((question) => question.slice(0, 300))),
    )
    const companyContextGeneration = companyContextGenerationRef.current
    const receive = async (): Promise<Blob> => {
      // Backend は保存済みの発話から履歴を組み直す（#1636）。発話はサーバーがターンの中で
      // 保存し終えてから応答する。冒頭の質問より先に回答が保存されないよう、/start-turn の
      // 応答を待ってから送る(#1969)。
      await startTurnSettledRef.current
      await authService.ensureFreshUserToken()
      const { meta, audio } = await fetchAndReadWithTimeout(`${BACKEND_URL}/api/interviews/${session.id}/turn`, {
        method: 'POST',
        headers: { ...authService.getUserFetchHeaders() },
        body: formData,
      }, TURN_FETCH_TIMEOUT_MS, async res => {
        if (!res.ok) throw new Error(extractApiErrorMessage(await res.text()))
        return parseMultipartResponse(res)
      })
      if (companyContextGeneration !== companyContextGenerationRef.current) return audio
      companyContextRef.current = applyCompanyReadingResponse(
        companyContextRef.current,
        meta,
        companyContextGeneration,
        companyContextGenerationRef.current,
      )
      const userText: string = meta.user_text || ''
      const aiText: string = meta.ai_text || ''
      const responseQuestionTotal = Number(meta.total_questions)
      if (Number.isSafeInteger(responseQuestionTotal) && responseQuestionTotal > 0) {
        setTotalQuestionCount(responseQuestionTotal)
      }
      const questionEvent = typeof meta.question_source === 'string' && meta.question_source.trim() !== ''
      if (questionEvent) {
        setCurrentQuestionIndex((index) => nextQuestionIndex(index, meta.question_source))
        questionStartedAtRef.current = Date.now()
        setQuestionElapsedSeconds(0)
      }
      setIsDeepeningQuestion(Boolean(meta.is_deepening))
      setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
      // 書き起こしはサーバーが user → ai の順に保存済み(#1969)。ここでは表示だけを更新する。
      if (String(meta.utterance_save_failed) === 'true') setUtteranceSaveFailed(true)
      if (userText) {
        historyRef.current.push({ role: 'user', content: userText })
        setUtterances(p => [...p, { role: 'user', text: userText }])
      }
      if (aiText) {
        historyRef.current.push({ role: 'assistant', content: aiText })
        setUtterances(p => [...p, { role: 'ai', text: aiText }])
      }
      return audio
    }
    try {
      await playTurnAudio(receive())
    } catch (e: unknown) {
      if (companyContextGeneration === companyContextGenerationRef.current) {
        setErrorMessage(parseMediaError(e))
      }
    } finally {
      if (companyContextGeneration === companyContextGenerationRef.current) {
        setTurnPending(false)
      }
    }
  }

  return {
    inFlightTurnRef,
    startTurnSettledRef,
    historyRef,
    companyContextRef,
    companyContextGenerationRef,
    doStartTurn,
    sendTurn,
  }
}
