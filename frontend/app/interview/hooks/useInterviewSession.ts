'use client'

import { useEffect, useRef, useState } from 'react'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService, User } from '@/lib/auth'
import { interviewApi, interviewLimits, InterviewReport, InterviewSession } from '@/lib/interview'
import { extractApiErrorMessage, parseMediaError, parseMultipartResponse } from '@/lib/interview/utils'
import { WeightScore } from '@/components/ScoreUpdateBanner'
import type { InterviewMedia } from './useInterviewMedia'
import { useHandsFreeVad } from './useHandsFreeVad'
import { pickRecorderFormat, extFromMimeType, type RecorderFormat } from '../lib/recorderFormat'
import { buildCompanyInfo, getNextAvatarGender } from '../utils'
import {
  evaluateReportPollTick,
  REPORT_POLL_INTERVAL_MS,
  REPORT_POLL_TIMEOUT_MS,
} from '../reportPolling'
import { resolveFinishOutcomeMessage } from '../finishOutcome'
import { saveUtteranceWithRetry, newClientUtteranceId, flushThenFinish } from '../utteranceSave'
import { fetchAndReadWithTimeout } from '@/lib/fetch-timeout'
import type { Utterance, InterviewCompany, Position, InterviewStatus } from '../types'

export type ReportStatus = 'idle' | 'pending' | 'ready' | 'error' | 'timeout'

/** レポート生成の再投入API自体が失敗したときの文言(#1476) */
export const REPORT_RETRY_FAILED_MESSAGE =
  'レポート生成の再試行リクエストに失敗しました。通信状況を確認して、もう一度お試しください。'

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

type UseInterviewSessionArgs = {
  user: User | null
  interviewCompany: InterviewCompany | null
  selectedPosition: Position
  media: InterviewMedia
  status: InterviewStatus
  setStatus: (status: InterviewStatus) => void
}

/**
 * 面接セッションのライフサイクル（参加・ターン・終了・レポート・録音）。
 * メディア refs は media 側が所有し、ここでは利用のみ。
 */
export function useInterviewSession({
  user,
  interviewCompany,
  selectedPosition,
  media,
  status,
  setStatus,
}: UseInterviewSessionArgs) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [utterances, setUtterances] = useState<Utterance[]>([])
  const [partialUser, setPartialUser] = useState('')
  const [partialAi, setPartialAi] = useState('')
  const [remainingSeconds, setRemainingSeconds] = useState(interviewLimits.maxMinutes * 60)
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(1)
  const [questionElapsedSeconds, setQuestionElapsedSeconds] = useState(0)
  const [isDeepeningQuestion, setIsDeepeningQuestion] = useState(false)
  const [questionCategory, setQuestionCategory] = useState<string | null>(null)
  const [sessionWarningShown, setSessionWarningShown] = useState(false)
  const [session, setSession] = useState<InterviewSession | null>(null)
  const [report, setReport] = useState<InterviewReport | null>(null)
  const [reportStatus, setReportStatus] = useState<ReportStatus>('idle')
  /** レポート生成の再投入API(regenerateReport)が失敗したときのメッセージ(#1476) */
  const [reportRetryError, setReportRetryError] = useState('')
  const [emailSending, setEmailSending] = useState(false)
  const [emailSent, setEmailSent] = useState(false)
  // メール送信失敗をUIへ伝えるためのメッセージ（#1056）
  const [emailError, setEmailError] = useState('')
  const [aiLevel, setAiLevel] = useState(0)
  const [aiSpeaking, _setAiSpeaking] = useState(false)
  const [avatarGender, setAvatarGender] = useState<'male' | 'female'>('male')
  const [captionsVisible, setCaptionsVisible] = useState(true)
  const [handsFreeMode, setHandsFreeMode] = useState(false)
  const [consentDialogOpen, setConsentDialogOpen] = useState(false)
  const [consentGiven, setConsentGiven] = useState(false)
  const [isRecording, _setIsRecording] = useState(false)
  const [turnPending, _setTurnPending] = useState(false)
  const [videoUploadStatus, setVideoUploadStatus] = useState<'idle' | 'uploading' | 'done' | 'error'>('idle')
  const [videoUploadProgress, setVideoUploadProgress] = useState(0)
  const [videoSizeWarning, setVideoSizeWarning] = useState<string | null>(null)
  const [scoresBefore, setScoresBefore] = useState<WeightScore[] | null>(null)
  const [scoresAfter, setScoresAfter] = useState<WeightScore[] | null>(null)
  /** 面接終了API(finishSession)が失敗したかどうか。true の間はレポート画面に再試行UIを出す(#1015) */
  const [finishFailed, setFinishFailed] = useState(false)
  /** 発話保存が再試行しても失敗したか。true の間は面接画面・レポート画面に警告を出す(#1476) */
  const [utteranceSaveFailed, setUtteranceSaveFailed] = useState(false)

  /**
   * 発話保存を直列につなぐチェーン(#1476)。
   * 再試行込みの保存をターン処理の中で待つと、失敗時に音声再生が数秒止まる。
   * かといって投げっぱなしにすると user/ai の保存順が入れ替わり、レポートの書き起こしが崩れる。
   * 1本のチェーンに積むことで、順序を保ったままターン処理をブロックしない。
   */
  const utteranceSaveChainRef = useRef<Promise<void>>(Promise.resolve())
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
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const recorderFormatRef = useRef<RecorderFormat>({ mimeType: '', ext: 'webm' })
  // 発話が確定しなかった録音を送らずに捨てるためのフラグ。
  // MediaRecorder.stop() は非同期で onstop を呼ぶため、
  // 停止要求の時点でどちらの意図か記録しておく必要がある。
  const discardTurnRef = useRef(false)
  const audioChunksRef = useRef<Blob[]>([])
  const historyRef = useRef<{ role: string; content: string }[]>([])
  const companyContextRef = useRef({ reading: '', info: '' })
  const aiAudioRef = useRef<HTMLAudioElement | null>(null)
  const aiAudioCtxRef = useRef<AudioContext | null>(null)
  const aiLevelRafRef = useRef<number | null>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollStartedAtRef = useRef<number | null>(null)
  const pollSessionRef = useRef<{ sessionId: number; userId: number } | null>(null)
  /** 再試行時に古い tick の結果を破棄するための世代カウンタ */
  const pollGenerationRef = useRef(0)
  // レポートポーリングの連続fetch失敗数。成功したら0に戻す（#1057）
  const pollFailureCountRef = useRef(0)
  /** playAudioBlob の世代。cleanupConnection で increment し、古い再生を破棄する */
  const audioGenerationRef = useRef(0)
  const sessionStartRef = useRef<number | null>(null)
  const transcriptEndRef = useRef<HTMLDivElement | null>(null)
  const isRecordingRef = useRef(false)
  const turnPendingRef = useRef(false)
  const aiSpeakingRef = useRef(false)
  // タイマーのsetIntervalコールバックがstale closureでhandleStop実行時点の
  // session/userを読んでしまう(#926)のを防ぐため、常に最新値を反映するrefを使う
  const sessionRef = useRef<InterviewSession | null>(null)
  const userRef = useRef<User | null>(null)
  // VAD effect は hooks 順序のため先に登録し、実装は ref 経由で呼ぶ
  const startRecordingRef = useRef<() => void>(() => {})
  const stopRecordingRef = useRef<() => void>(() => {})
  const discardRecordingRef = useRef<() => void>(() => {})

  // ref と state を常に同期（VAD の stale closure 対策）
  const setIsRecording = (v: boolean) => { isRecordingRef.current = v; _setIsRecording(v) }
  const setTurnPending = (v: boolean) => { turnPendingRef.current = v; _setTurnPending(v) }
  const setAiSpeaking = (v: boolean) => { aiSpeakingRef.current = v; _setAiSpeaking(v) }

  useHandsFreeVad({
    enabled: handsFreeMode,
    status,
    streamRef: media.streamRef,
    isRecordingRef,
    turnPendingRef,
    aiSpeakingRef,
    startRecording: () => startRecordingRef.current(),
    stopRecording: () => stopRecordingRef.current(),
    discardRecording: () => discardRecordingRef.current(),
  })

  useEffect(() => { userRef.current = user }, [user])

  // Cleanup on unmount
  useEffect(() => () => cleanupConnection(), [])

  // Auto-scroll transcript
  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [utterances, partialAi])

  const cleanupConnection = () => {
    audioGenerationRef.current++
    ;[timerRef, pollRef].forEach(r => { if (r.current) { clearInterval(r.current); r.current = null } })
    if (mediaRecorderRef.current) {
      // 録音中に終了した場合、stop() の onstop で新しいターンを送ると
      // 終了API(=レポート生成のキュー投入)より後に発話が積まれる(#1476)。
      // まだ確定していない録音なので送らずに捨てる（VADの空振りと同じ扱い）。
      //
      // state は見ない。MediaRecorder.stop() は state を即座に 'inactive' にする一方で
      // onstop は後のタスクで発火するため、「送信ボタンを押した直後に終了した」場合の
      // 未処理の録音が state では 'inactive' と区別できない。この録音を見逃すと、
      // onstop がまだ trackTurn を呼んでいない＝待つべき Promise も無いまま
      // finishSession の後に /turn が飛び、最後の回答がレポートから落ちる。
      discardTurnRef.current = true
      if (mediaRecorderRef.current.state !== 'inactive') mediaRecorderRef.current.stop()
      mediaRecorderRef.current = null
    }
    if (aiAudioRef.current) { aiAudioRef.current.pause(); aiAudioRef.current.src = '' }
    if (aiLevelRafRef.current !== null) { cancelAnimationFrame(aiLevelRafRef.current); aiLevelRafRef.current = null }
    if (aiAudioCtxRef.current) { aiAudioCtxRef.current.close().catch(() => {}); aiAudioCtxRef.current = null }
    setAiLevel(0)
    media.stopStream()
    setIsRecording(false); setTurnPending(false); setAiSpeaking(false)
  }

  const startTimer = (totalQuestions: number) => {
    sessionStartRef.current = Date.now()
    timerRef.current = setInterval(() => {
      if (!sessionStartRef.current) return
      const elapsed = Math.floor((Date.now() - sessionStartRef.current) / 1000)
      const remaining = Math.max(0, interviewLimits.maxMinutes * 60 - elapsed)
      const safeQuestionDuration = Math.max(60, interviewLimits.questionDurationSeconds || 180)
      const safeTotalQuestions = Math.max(1, totalQuestions)
      const nextQuestionIndex = Math.min(
        safeTotalQuestions,
        Math.floor(elapsed / safeQuestionDuration) + 1,
      )
      const questionElapsed = Math.min(
        safeQuestionDuration,
        Math.max(0, elapsed - (nextQuestionIndex - 1) * safeQuestionDuration),
      )

      setElapsedSeconds(elapsed)
      setRemainingSeconds(remaining)
      setCurrentQuestionIndex(nextQuestionIndex)
      setQuestionElapsedSeconds(questionElapsed)
      if (remaining <= 120 && !sessionWarningShown) setSessionWarningShown(true)
      if (remaining <= 0) handleStop(true)
    }, 1000)
  }

  /**
   * ターンの応答音声を再生する。
   * gen は「リクエストを投げた時点」の音声世代で、呼び出し側から引き継ぐ(#1501)。
   * ここで audioGenerationRef を読むと、応答待ちの間に「完了する」で
   * cleanupConnection が世代を進めていても更新後の値を読むため stale 判定を通過し、
   * レポート画面へ遷移した後に面接官の音声が鳴り出す。
   */
  const playAudioBlob = async (blob: Blob, gen: number): Promise<void> => {
    // 終了済み（または次のターンに切り替わった後）なら、再生を始めない
    if (gen !== audioGenerationRef.current) return
    const url = URL.createObjectURL(blob)
    const el = new Audio()
    aiAudioRef.current = el
    el.src = url
    setAiSpeaking(true)

    let rafId: number | null = null

    const cleanup = () => {
      if (rafId !== null) {
        cancelAnimationFrame(rafId)
        rafId = null
      }
      // この要素が現在共有されている再生対象でない場合(=より新しいターンの
      // 音声に切り替わった後の古いクリーンアップ)は、新しい方の再生状態
      // (aiSpeaking/aiLevel/RAF)を巻き戻さないよう、自身のリソース解放のみ行う
      const isCurrent = aiAudioRef.current === el
      if (isCurrent) {
        if (aiLevelRafRef.current !== null) {
          cancelAnimationFrame(aiLevelRafRef.current)
          aiLevelRafRef.current = null
        }
        setAiLevel(0)
        setAiSpeaking(false)
        aiAudioRef.current = null
      }
      URL.revokeObjectURL(url)
      el.removeAttribute('src')
      el.load()
    }

    try {
      if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
        aiAudioCtxRef.current = new AudioContext()
      }
      const ctx = aiAudioCtxRef.current
      await ctx.resume()
      // await中に次のターンが始まっていれば、この古い音声は再生せず破棄する
      if (gen !== audioGenerationRef.current) { cleanup(); return }

      const source = ctx.createMediaElementSource(el)
      const analyser = ctx.createAnalyser()
      analyser.fftSize = 512
      analyser.smoothingTimeConstant = 0.6
      source.connect(analyser)
      analyser.connect(ctx.destination)

      const timeData = new Uint8Array(analyser.fftSize)
      const trackLevel = () => {
        analyser.getByteTimeDomainData(timeData)
        let sum = 0
        for (const v of timeData) {
          const n = (v - 128) / 128
          sum += n * n
        }
        const rms = Math.sqrt(sum / timeData.length)
        setAiLevel(Math.min(1, rms * 6))
        rafId = requestAnimationFrame(trackLevel)
      }
      rafId = requestAnimationFrame(trackLevel)
      aiLevelRafRef.current = rafId
    } catch (err) {
      // AudioContext 未対応時は Audio 要素のデフォルト出力にフォールバック（リップシンクなし）
      console.warn('[Interview] AudioContext routing unavailable; playing via element output', err)
    }

    // AudioContext設定中に次のターンが始まっていれば、ここでも再生せず破棄する
    if (gen !== audioGenerationRef.current) { cleanup(); return }

    return new Promise<void>((resolve) => {
      el.onended = () => {
        cleanup()
        resolve()
      }
      el.onerror = () => {
        console.error('[Interview] AI audio element error', el.error)
        cleanup()
        resolve()
      }
      el.play().catch((err) => {
        console.error('[Interview] AI audio play() failed (check CSP media-src / autoplay)', err)
        cleanup()
        resolve()
      })
    })
  }

  /**
   * 発話保存をチェーンに積む(#1476)。
   * 再試行しても保存できなければ utteranceSaveFailed を立て、UIに「記録できていない」ことを出す。
   * 面接自体は止めない（止めてもユーザーにできることが無い）。
   */
  const queueUtteranceSave = (sessionId: number, userId: number, role: 'user' | 'ai', text: string) => {
    // IDはチェーンに積む時点で1つだけ発行する。再試行の中で作り直すと、
    // 結果不明の失敗（応答ロスト・タイムアウト）の再送が別の発話として保存される(#1476)。
    const clientUtteranceId = newClientUtteranceId()
    utteranceSaveChainRef.current = utteranceSaveChainRef.current
      .then(async () => {
        const saved = await saveUtteranceWithRetry(
          () => interviewApi.saveUtterance(sessionId, userId, role, text, clientUtteranceId),
        )
        if (!saved) setUtteranceSaveFailed(true)
      })
      // チェーンが reject のまま残ると、面接終了時にこれを待つ処理ごと落ちて
      // レポート生成が始まらなくなる。必ず解決する鎖にしておく(#1476)。
      .catch(e => {
        console.error('[utterance save chain error]', e)
        setUtteranceSaveFailed(true)
      })
  }

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
          total_questions: Math.max(1, selectedPosition?.questions || 1),
          question_elapsed_seconds: 0,
          question_duration_seconds: Math.max(60, interviewLimits.questionDurationSeconds || 180),
        }),
      }, TURN_FETCH_TIMEOUT_MS, async res => {
        if (!res.ok) throw new Error(extractApiErrorMessage(await res.text()))
        return parseMultipartResponse(res)
      })
      if (typeof meta.company_reading === 'string' && meta.company_reading) {
        companyContextRef.current.reading = meta.company_reading
      }
      if (typeof meta.company_info === 'string' && meta.company_info) {
        companyContextRef.current.info = meta.company_info
      }
      const aiText: string = meta.ai_text || ''
      setIsDeepeningQuestion(Boolean(meta.is_deepening))
      setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
      if (aiText) {
        historyRef.current.push({ role: 'assistant', content: aiText })
        setUtterances(p => [...p, { role: 'ai', text: aiText }])
        queueUtteranceSave(sessionId, userId, 'ai', aiText)
      }
      return audio
    }
    await playTurnAudio(receive())
  }

  const handleJoinWithConsent = () => {
    if (!consentGiven) {
      setConsentDialogOpen(true)
      return
    }
    handleJoin()
  }

  const handleJoin = async () => {
    if (!user) return
    setErrorMessage(null)
    setUtterances([])
    setPartialUser(''); setPartialAi('')
    setReport(null); setReportStatus('idle'); setReportRetryError('')
    setRemainingSeconds(interviewLimits.maxMinutes * 60)
    setElapsedSeconds(0)
    setCurrentQuestionIndex(1)
    setQuestionElapsedSeconds(0)
    setSessionWarningShown(false)
    setUtteranceSaveFailed(false)
    utteranceSaveChainRef.current = Promise.resolve()
    // 前回の面接でハングしたターンを次の終了処理が待ち続けないよう、参加時に捨てる
    inFlightTurnRef.current = Promise.resolve()
    media.setMicEnabled(true); media.setCameraEnabled(true)
    historyRef.current = []

    try {
      setStatus('connecting')
      const nextGender = getNextAvatarGender()
      setAvatarGender(nextGender)

      // ユーザー操作中に AudioContext を事前作成・resume（autoplay policy 対策）
      try {
        if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
          aiAudioCtxRef.current = new AudioContext()
        }
        await aiAudioCtxRef.current.resume()
      } catch { /* 非対応環境は無視 */ }

      const stream = await media.ensureStream()

      const created = await interviewApi.createSession(user.user_id, 'ja', nextGender)
      sessionRef.current = created
      setSession(created)
      await interviewApi.startSession(created.id, user.user_id)
      setStatus('connected')

      if (stream) {
        media.startVideoRecording(stream)
      }

      startTimer(selectedPosition.questions)
      await doStartTurn(created.id, user.user_id)
    } catch (error: any) {
      setStatus('error')
      setErrorMessage(parseMediaError(error))
      cleanupConnection()
    }
  }

  const resolveScoreSessionId = (userId: number) => {
    if (typeof window !== 'undefined') {
      const chatSession = window.localStorage.getItem('chat_session_id')
      if (chatSession && chatSession.trim() !== '') {
        return chatSession
      }
    }
    return `interview-${userId}`
  }

  const handleStop = async (forced = false) => {
    const videoBlob = await media.stopAndCollectVideoBlob()

    cleanupConnection()
    // タイマーのsetIntervalから呼ばれた場合でも最新値を読むため、stateではなくrefを使う(#926)
    const currentSession = sessionRef.current
    const currentUser = userRef.current
    if (!currentUser || !currentSession) {
      setFinishFailed(false)
      setStatus('finished')
      setReportStatus('pending')
      setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: false, forced }))
      return
    }

    const scoreSessionId = resolveScoreSessionId(currentUser.user_id)
    try {
      const res = await fetch(`/api/user/weight-scores?user_id=${currentUser.user_id}&session_id=${encodeURIComponent(scoreSessionId)}`)
      const data = await res.json()
      setScoresBefore(data.weight_scores ?? null)
    } catch {
      // 取得失敗時は scoresBefore=null のまま。ScoreUpdateBanner側で「スコア比較なし」と表示する(#1015)
      setScoresBefore(null)
    }

    // 終了APIを待たずに画面を先へ進める。以降の待ちでユーザーを止めないための順序(#1476)。
    setFinishFailed(false)
    setStatus('finished')
    setReportStatus('pending')
    setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: false, forced }))

    // 動画アップロードは発話保存と独立なので先に始める
    if (videoBlob) {
      const MB = 1024 * 1024
      const sizeMB = (videoBlob.size / MB).toFixed(1)
      if (videoBlob.size > 200 * MB) {
        setVideoSizeWarning(`動画サイズが ${sizeMB} MB と非常に大きいです。アップロードに時間がかかる場合があります。`)
      }
      setVideoUploadStatus('uploading')
      setVideoUploadProgress(0)
      interviewApi.uploadVideo(
        currentSession.id,
        currentUser.user_id,
        videoBlob,
        (percent) => setVideoUploadProgress(percent),
      )
        .then(() => { setVideoUploadStatus('done'); setVideoSizeWarning(null) })
        .catch(() => setVideoUploadStatus('error'))
    }

    // 積み残した発話保存を「全部」片付けてから終了APIを呼ぶ(#1476)。
    // 画面は既に finished へ進めてあるので、ここで待ってもユーザーは止まらない。
    try {
      // 応答待ちのターンが発話を積み終えるのを先に待つ。ここを飛ばすと、
      // 「完了する」の後に届いた /turn の発話が finishSession の後に保存され、
      // レポートが最後のやり取り抜きで生成される(#1476)。
      await inFlightTurnRef.current
      await flushThenFinish(
        utteranceSaveChainRef.current,
        () => interviewApi.finishSession(currentSession.id, currentUser.user_id),
      )
    } catch {
      // 終了APIの失敗を握りつぶさず、レポート画面にエラーと再試行手段を出す(#1015)
      setFinishFailed(true)
      setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: true, forced }))
    }
    // ポーリングは終了API（=レポート生成のキュー投入）の後に始める。
    // 先に始めるとタイムアウトの3分が生成開始前から減り始める。
    startReportPolling(currentSession.id, currentUser.user_id)
  }

  /** finishSession失敗時にレポート画面から再試行するための関数(#1015) */
  const retryFinish = async () => {
    const currentSession = sessionRef.current
    const currentUser = userRef.current
    if (!currentSession || !currentUser) return
    try {
      await interviewApi.finishSession(currentSession.id, currentUser.user_id)
      setFinishFailed(false)
      setErrorMessage(null)
    } catch {
      setFinishFailed(true)
      setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: true, forced: false }))
    }
  }

  const stopReportPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const loadScoresAfter = async (userId: number) => {
    const scoreSessionId = resolveScoreSessionId(userId)
    try {
      const res = await fetch(`/api/user/weight-scores?user_id=${userId}&session_id=${encodeURIComponent(scoreSessionId)}`)
      const data = await res.json()
      setScoresAfter(data.weight_scores ?? null)
    } catch { /* ignore */ }
  }

  const startReportPolling = (sessionId: number, userId: number) => {
    stopReportPolling()
    const generation = ++pollGenerationRef.current
    pollSessionRef.current = { sessionId, userId }
    pollStartedAtRef.current = Date.now()
    pollFailureCountRef.current = 0
    setReportStatus('pending')

    const tick = async () => {
      if (generation !== pollGenerationRef.current) return

      const startedAt = pollStartedAtRef.current ?? Date.now()
      let hasReport = false
      let reportPayload: InterviewReport | null = null

      try {
        const detail = await interviewApi.getDetail(sessionId, userId)
        // 通信が復帰したら連続失敗数をリセットする（#1057）
        pollFailureCountRef.current = 0
        if (detail.report) {
          hasReport = true
          reportPayload = detail.report
        }
      } catch {
        pollFailureCountRef.current += 1
      }

      if (generation !== pollGenerationRef.current) return

      const outcome = evaluateReportPollTick({
        startedAtMs: startedAt,
        nowMs: Date.now(),
        timeoutMs: REPORT_POLL_TIMEOUT_MS,
        hasReport,
        consecutiveFailures: pollFailureCountRef.current,
      })

      if (outcome === 'ready' && reportPayload) {
        setReport(reportPayload)
        setReportStatus('ready')
        stopReportPolling()
        await loadScoresAfter(userId)
        return
      }
      if (outcome === 'error') {
        setReportStatus('error')
        stopReportPolling()
        return
      }
      if (outcome === 'timeout') {
        setReportStatus('timeout')
        stopReportPolling()
      }
    }

    // 初回は即時、以降は interval
    void tick()
    pollRef.current = setInterval(() => { void tick() }, REPORT_POLL_INTERVAL_MS)
  }

  /**
   * レポート取得の再試行(#1015, #1476)。
   *
   * ポーリングを再開するだけでは、生成ジョブ自体が失われている場合に永久に復旧しない
   * （asynqの再試行を使い切る／フォールバックworkerで1回失敗する／キューが溢れる、
   * かつ finishSession は終了済みセッションを再キューしない）。
   * そこで再開の前に生成ジョブの再投入を依頼する。既にレポートがあればサーバー側で何もしない。
   */
  const retryReportPolling = async () => {
    const target = pollSessionRef.current
      ?? (session && user ? { sessionId: session.id, userId: user.user_id } : null)
    if (!target) return
    setReportRetryError('')
    try {
      await interviewApi.regenerateReport(target.sessionId, target.userId)
    } catch (e) {
      // 再投入が失敗した＝ジョブは入り直っていない。ここでポーリングへ戻すと、
      // ユーザーは失敗を知らされないまま「生成中」をさらに3分見せられて再びタイムアウトする(#1476)。
      console.error('[report regenerate error]', e)
      setReportRetryError(REPORT_RETRY_FAILED_MESSAGE)
      return
    }
    startReportPolling(target.sessionId, target.userId)
  }

  const startRecording = () => {
    if (!media.streamRef.current || isRecordingRef.current || turnPendingRef.current) return
    const audioTracks = media.streamRef.current.getAudioTracks()
    if (audioTracks.length === 0) return
    const micStream = new MediaStream(audioTracks)
    audioChunksRef.current = []
    discardTurnRef.current = false
    // mimeType はブラウザが実際に出せる形式から選ぶ。'audio/webm' 固定だと
    // Safari では NotSupportedError で例外になり、録音が始まらなかった。
    const format = pickRecorderFormat()
    recorderFormatRef.current = format
    const mr = new MediaRecorder(
      micStream,
      format.mimeType
        ? { mimeType: format.mimeType, audioBitsPerSecond: 128000 }
        : { audioBitsPerSecond: 128000 },
    )
    mr.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
    mr.onstop = () => {
      if (discardTurnRef.current) {
        discardTurnRef.current = false
        audioChunksRef.current = []
        return
      }
      void sendTurn()
    }
    mediaRecorderRef.current = mr
    mr.start()
    setIsRecording(true)
  }
  startRecordingRef.current = startRecording

  const stopRecording = () => {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === 'inactive') return
    mediaRecorderRef.current.stop()
    setIsRecording(false)
    setTurnPending(true)
  }
  stopRecordingRef.current = stopRecording

  /**
   * 発話が確定しなかった録音を破棄する（送信しない）。
   *
   * VAD は頭切れを避けるため低いしきい値で録音を始める。
   * 物音で始まった録音まで送ると、無音や雑音だけの音声で
   * STT を呼ぶことになり、費用と誤認識が増える。
   */
  const discardRecording = () => {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === 'inactive') return
    discardTurnRef.current = true
    mediaRecorderRef.current.stop()
    setIsRecording(false)
  }
  discardRecordingRef.current = discardRecording

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
    formData.append('history', JSON.stringify(historyRef.current))
    formData.append('turn_count', String(Math.floor(historyRef.current.length / 2) + 1))
    formData.append('remaining_seconds', String(remainingSeconds))
    formData.append('question_index', String(currentQuestionIndex))
    formData.append('total_questions', String(Math.max(1, selectedPosition.questions)))
    formData.append('question_elapsed_seconds', String(questionElapsedSeconds))
    formData.append('question_duration_seconds', String(Math.max(60, interviewLimits.questionDurationSeconds || 180)))
    formData.append('company_name', interviewCompany?.name || '')
    formData.append('company_reading', companyContextRef.current.reading || interviewCompany?.name_reading || '')
    formData.append('position', selectedPosition?.title || '')
    formData.append('company_info', companyContextRef.current.info || buildCompanyInfo(interviewCompany))
    formData.append('company_type', selectedPosition?.category || 'general')
    formData.append('company_id', String(interviewCompany?.id || 0))
    const receive = async (): Promise<Blob> => {
      await authService.ensureFreshUserToken()
      const { meta, audio } = await fetchAndReadWithTimeout(`${BACKEND_URL}/api/interviews/${session.id}/turn`, {
        method: 'POST',
        headers: { ...authService.getUserFetchHeaders() },
        body: formData,
      }, TURN_FETCH_TIMEOUT_MS, async res => {
        if (!res.ok) throw new Error(extractApiErrorMessage(await res.text()))
        return parseMultipartResponse(res)
      })
      if (typeof meta.company_reading === 'string' && meta.company_reading) {
        companyContextRef.current.reading = meta.company_reading
      }
      if (typeof meta.company_info === 'string' && meta.company_info) {
        companyContextRef.current.info = meta.company_info
      }
      const userText: string = meta.user_text || ''
      const aiText: string = meta.ai_text || ''
      setIsDeepeningQuestion(Boolean(meta.is_deepening))
      setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
      if (userText) {
        historyRef.current.push({ role: 'user', content: userText })
        setUtterances(p => [...p, { role: 'user', text: userText }])
        queueUtteranceSave(session.id, user.user_id, 'user', userText)
      }
      if (aiText) {
        historyRef.current.push({ role: 'assistant', content: aiText })
        setUtterances(p => [...p, { role: 'ai', text: aiText }])
        queueUtteranceSave(session.id, user.user_id, 'ai', aiText)
      }
      return audio
    }
    try {
      await playTurnAudio(receive())
    } catch (e: unknown) {
      setErrorMessage(parseMediaError(e))
    } finally {
      setTurnPending(false)
    }
  }

  const sendReportEmail = async () => {
    if (!session || !user) return
    setEmailSending(true)
    setEmailError('')
    try {
      await interviewApi.sendReportEmail(session.id, user.user_id)
      setEmailSent(true)
    } catch {
      // 握り潰すとユーザーが送信成功と誤解するため、必ずUIへ伝える（#1056）
      setEmailError('メールの送信に失敗しました。時間をおいて再度お試しください。')
    } finally {
      setEmailSending(false)
    }
  }

  return {
    errorMessage,
    utterances,
    partialUser,
    partialAi,
    remainingSeconds,
    elapsedSeconds,
    currentQuestionIndex,
    questionElapsedSeconds,
    isDeepeningQuestion,
    questionCategory,
    sessionWarningShown,
    session,
    report,
    reportStatus,
    reportRetryError,
    retryReportPolling,
    emailSending,
    emailSent,
    emailError,
    aiLevel,
    aiSpeaking,
    avatarGender,
    captionsVisible,
    setCaptionsVisible,
    handsFreeMode,
    setHandsFreeMode,
    consentDialogOpen,
    setConsentDialogOpen,
    consentGiven,
    setConsentGiven,
    isRecording,
    turnPending,
    videoUploadStatus,
    videoUploadProgress,
    videoSizeWarning,
    scoresBefore,
    scoresAfter,
    finishFailed,
    utteranceSaveFailed,
    retryFinish,
    aiAudioRef,
    transcriptEndRef,
    handleJoin,
    handleJoinWithConsent,
    handleStop,
    startRecording,
    stopRecording,
    sendReportEmail,
  }
}
