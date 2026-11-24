'use client'

import { useEffect, useRef, useState } from 'react'
import { User } from '@/lib/auth'
import { interviewApi, interviewLimits, InterviewSession } from '@/lib/interview'
import { parseMediaError } from '@/lib/interview/utils'
import { WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import type { InterviewMedia } from './useInterviewMedia'
import { useHandsFreeVad } from './useHandsFreeVad'
import { useInterviewAudioPlayback } from './useInterviewAudioPlayback'
import { useInterviewRecorder } from './useInterviewRecorder'
import { useInterviewClock } from './useInterviewClock'
import { useInterviewReport, resolveScoreSessionId } from './useInterviewReport'
import { useInterviewTurns } from './useInterviewTurns'
import { resolveFinishOutcomeMessage } from '../finishOutcome'
import type { Utterance, InterviewCompany, Position, InterviewStatus } from '../types'

export { REPORT_RETRY_FAILED_MESSAGE, type ReportStatus } from './useInterviewReport'

type UseInterviewSessionArgs = {
  user: User | null
  interviewCompany: InterviewCompany | null
  companyQuestionHints: string[]
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
  companyQuestionHints,
  selectedPosition,
  media,
  status,
  setStatus,
}: UseInterviewSessionArgs) {
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [utterances, setUtterances] = useState<Utterance[]>([])
  const [partialUser, setPartialUser] = useState('')
  const [partialAi, setPartialAi] = useState('')
  const [session, setSession] = useState<InterviewSession | null>(null)
  /** 学生の発話に区切りが来るたびに増える。面接官アバターの相槌の合図（#1646）。 */
  const [studentPauseCount, setStudentPauseCount] = useState(0)
  // 面接官は男性のみ。声(バックエンドの interviewer_gender)と見た目(Live2D)を揃える。
  const avatarGender = 'male' as const
  const [captionsVisible, setCaptionsVisible] = useState(true)
  const [handsFreeMode, setHandsFreeMode] = useState(false)
  const [consentDialogOpen, setConsentDialogOpen] = useState(false)
  const [consentGiven, setConsentGiven] = useState(false)
  const [videoUploadStatus, setVideoUploadStatus] = useState<'idle' | 'uploading' | 'done' | 'error'>('idle')
  const [videoUploadProgress, setVideoUploadProgress] = useState(0)
  const [videoSizeWarning, setVideoSizeWarning] = useState<string | null>(null)
  const [scoresBefore, setScoresBefore] = useState<WeightScore[] | null>(null)
  /** 面接終了API(finishSession)が失敗したかどうか。true の間はレポート画面に再試行UIを出す(#1015) */
  const [finishFailed, setFinishFailed] = useState(false)
  /**
   * 発話を書き起こしへ保存できなかったか。true の間は面接画面・レポート画面に警告を出す(#1476)。
   * 発話はサーバーが /start-turn・/turn の中で保存し、失敗を応答の utterance_save_failed で返す(#1969)。
   */
  const [utteranceSaveFailed, setUtteranceSaveFailed] = useState(false)
  const transcriptEndRef = useRef<HTMLDivElement | null>(null)
  // タイマーのsetIntervalコールバックがstale closureでhandleStop実行時点の
  // session/userを読んでしまう(#926)のを防ぐため、常に最新値を反映するrefを使う
  const sessionRef = useRef<InterviewSession | null>(null)
  const userRef = useRef<User | null>(null)

  // 小さなフックへ渡す後段の関数（sendTurn / handleStop）は、このレンダーで後から定義するものを
  // 呼び出し時に参照する。分割前と同じく「録音・タイマーを始めたレンダー」の関数が呼ばれる。
  const {
    aiLevel, setAiLevel, aiSpeaking, setAiSpeaking, aiSpeakingRef,
    aiAudioRef, aiAudioCtxRef, aiLevelRafRef, audioGenerationRef, playAudioBlob,
  } = useInterviewAudioPlayback()
  const {
    isRecording, setIsRecording, isRecordingRef, turnPending, setTurnPending, turnPendingRef,
    mediaRecorderRef, recorderFormatRef, discardTurnRef, audioChunksRef,
    startRecordingRef, stopRecordingRef, discardRecordingRef, startRecording, stopRecording,
  } = useInterviewRecorder({ media, sendTurn: () => sendTurn() })
  const {
    remainingSeconds, setRemainingSeconds, elapsedSeconds, setElapsedSeconds,
    currentQuestionIndex, setCurrentQuestionIndex, totalQuestionCount, setTotalQuestionCount,
    questionElapsedSeconds, setQuestionElapsedSeconds, isDeepeningQuestion, setIsDeepeningQuestion,
    questionCategory, setQuestionCategory, sessionWarningShown, setSessionWarningShown,
    timerRef, questionStartedAtRef, startTimer,
  } = useInterviewClock({ selectedPosition, handleStop: (forced?: boolean) => handleStop(forced) })
  const {
    report, setReport, reportStatus, setReportStatus, reportProgress, setReportProgress,
    reportFailedByServer, setReportFailedByServer, reportStartedAt, reportRetryError, setReportRetryError,
    emailSending, emailSent, emailError, scoresAfter, pollRef,
    startReportPolling, retryReportPolling, sendReportEmail,
  } = useInterviewReport({ session, user })
  const {
    inFlightTurnRef, startTurnSettledRef, historyRef, companyContextRef, companyContextGenerationRef,
    doStartTurn, sendTurn,
  } = useInterviewTurns({
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
    onLimitReached: () => { void handleStop(false, true) },
  })

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
    onPause: () => setStudentPauseCount((n) => n + 1),
  })

  useEffect(() => { userRef.current = user }, [user])

  // Cleanup on unmount
  useEffect(() => () => cleanupConnection(true), [])

  // Auto-scroll transcript
  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [utterances, partialAi])

  const cleanupConnection = (invalidateCompanyContext = false) => {
    audioGenerationRef.current++
    if (invalidateCompanyContext) companyContextGenerationRef.current++
    ;[timerRef, pollRef].forEach(r => { if (r.current) { clearInterval(r.current); r.current = null } })
    questionStartedAtRef.current = null
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

  const handleJoinWithConsent = () => {
    if (!consentGiven) {
      setConsentDialogOpen(true)
      return
    }
    handleJoin()
  }

  const handleJoin = async () => {
    if (!user) return
    companyContextGenerationRef.current++
    setErrorMessage(null)
    setUtterances([])
    setPartialUser(''); setPartialAi('')
    setReport(null); setReportStatus('idle'); setReportRetryError(''); setReportProgress(null); setReportFailedByServer(false)
    setRemainingSeconds(interviewLimits.maxMinutes * 60)
    setElapsedSeconds(0)
    setCurrentQuestionIndex(1)
    setTotalQuestionCount(Math.max(1, selectedPosition.questions))
    setQuestionElapsedSeconds(0)
    setSessionWarningShown(false)
    setUtteranceSaveFailed(false)
    // 前回の面接でハングしたターンを次の終了処理が待ち続けないよう、参加時に捨てる
    inFlightTurnRef.current = Promise.resolve()
    startTurnSettledRef.current = Promise.resolve()
    media.setMicEnabled(true); media.setCameraEnabled(true)
    historyRef.current = []
    companyContextRef.current = { reading: '', readingResolved: false }

    try {
      setStatus('connecting')
      // ユーザー操作中に AudioContext を事前作成・resume（autoplay policy 対策）
      try {
        if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
          aiAudioCtxRef.current = new AudioContext()
        }
        await aiAudioCtxRef.current.resume()
      } catch { /* 非対応環境は無視 */ }

      const stream = await media.ensureStream()

      const created = await interviewApi.createSession(user.user_id, 'ja', avatarGender)
      sessionRef.current = created
      setSession(created)
      await interviewApi.startSession(created.id, user.user_id)
      setStatus('connected')

      if (stream) {
        media.startVideoRecording(stream)
      }

      startTimer()
      await doStartTurn(created.id, user.user_id)
    } catch (error: unknown) {
      setStatus('error')
      setErrorMessage(parseMediaError(error))
      cleanupConnection()
    }
  }

  const handleStop = async (forced = false, limitReached = false) => {
    const videoBlob = await media.stopAndCollectVideoBlob()

    cleanupConnection()
    // タイマーのsetIntervalから呼ばれた場合でも最新値を読むため、stateではなくrefを使う(#926)
    const currentSession = sessionRef.current
    const currentUser = userRef.current
    if (!currentUser || !currentSession) {
      setFinishFailed(false)
      setStatus('finished')
      setReportStatus('pending')
      setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: false, forced, limitReached }))
      return
    }

    const scoreSessionId = resolveScoreSessionId(currentUser.user_id)
    try {
      const res = await fetch(`/api/user/weight-scores?session_id=${encodeURIComponent(scoreSessionId)}`)
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
    setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: false, forced, limitReached }))

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

    // 応答待ちのターンが終わってから終了APIを呼ぶ(#1476)。
    // 画面は既に finished へ進めてあるので、ここで待ってもユーザーは止まらない。
    try {
      // 発話はサーバーがターンの中で保存する(#1969)。応答待ちのターンを飛ばして終了すると、
      // 「完了する」の後に返った /turn の発話が finishSession の後に保存され、
      // レポートが最後のやり取り抜きで生成される(#1476)。
      await inFlightTurnRef.current
      await interviewApi.finishSession(currentSession.id, currentUser.user_id)
    } catch {
      // 終了APIの失敗を握りつぶさず、レポート画面にエラーと再試行手段を出す(#1015)
      setFinishFailed(true)
      setErrorMessage(resolveFinishOutcomeMessage({ finishFailed: true, forced, limitReached }))
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

  return {
    errorMessage,
    utterances,
    partialUser,
    partialAi,
    remainingSeconds,
    elapsedSeconds,
    currentQuestionIndex,
    totalQuestionCount,
    questionElapsedSeconds,
    isDeepeningQuestion,
    questionCategory,
    sessionWarningShown,
    session,
    report,
    reportStatus,
    reportProgress,
    reportFailedByServer,
    reportStartedAt,
    reportRetryError,
    retryReportPolling,
    emailSending,
    emailSent,
    emailError,
    aiLevel,
    aiSpeaking,
    studentPauseCount,
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
