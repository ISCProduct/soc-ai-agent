'use client'

import { useEffect, useRef, useState } from 'react'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService, User } from '@/lib/auth'
import { interviewApi, interviewLimits, InterviewReport, InterviewSession } from '@/lib/interview'
import { parseMediaError, parseMultipartResponse } from '@/lib/interview-utils'
import { WeightScore } from '@/components/ScoreUpdateBanner'
import type { InterviewMedia } from './useInterviewMedia'
import { useHandsFreeVad } from './useHandsFreeVad'
import { buildCompanyInfo, getNextAvatarGender } from '../utils'
import {
  evaluateReportPollTick,
  REPORT_POLL_INTERVAL_MS,
  REPORT_POLL_TIMEOUT_MS,
} from '../reportPolling'
import type { Utterance, InterviewCompany, Position, InterviewStatus } from '../types'

export type ReportStatus = 'idle' | 'pending' | 'ready' | 'error' | 'timeout'

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
  const [emailSending, setEmailSending] = useState(false)
  const [emailSent, setEmailSent] = useState(false)
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

  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const historyRef = useRef<{ role: string; content: string }[]>([])
  const aiAudioRef = useRef<HTMLAudioElement | null>(null)
  const aiAudioCtxRef = useRef<AudioContext | null>(null)
  const aiLevelRafRef = useRef<number | null>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollStartedAtRef = useRef<number | null>(null)
  const pollSessionRef = useRef<{ sessionId: number; userId: number } | null>(null)
  /** 再試行時に古い tick の結果を破棄するための世代カウンタ */
  const pollGenerationRef = useRef(0)
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
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      mediaRecorderRef.current.stop(); mediaRecorderRef.current = null
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

  const playAudioBlob = async (blob: Blob): Promise<void> => {
    const gen = audioGenerationRef.current
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

    if (gen !== audioGenerationRef.current) { cleanup(); return }

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

  const doStartTurn = async (sessionId: number, userId: number) => {
    await authService.ensureFreshUserToken()
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/start-turn`, {
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
    })
    if (!res.ok) throw new Error(await res.text())
    const { meta, audio } = await parseMultipartResponse(res)
    const aiText: string = meta.ai_text || ''
    setIsDeepeningQuestion(Boolean(meta.is_deepening))
    setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
    if (aiText) {
      historyRef.current.push({ role: 'assistant', content: aiText })
      setUtterances(p => [...p, { role: 'ai', text: aiText }])
      try { await interviewApi.saveUtterance(sessionId, userId, 'ai', aiText) } catch (e) { console.error('[utterance save error]', e) }
    }
    await playAudioBlob(audio)
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
    setReport(null); setReportStatus('idle')
    setRemainingSeconds(interviewLimits.maxMinutes * 60)
    setElapsedSeconds(0)
    setCurrentQuestionIndex(1)
    setQuestionElapsedSeconds(0)
    setSessionWarningShown(false)
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

  const handleStop = async (forced = false) => {
    const videoBlob = await media.stopAndCollectVideoBlob()

    cleanupConnection()
    // タイマーのsetIntervalから呼ばれた場合でも最新値を読むため、stateではなくrefを使う(#926)
    const currentSession = sessionRef.current
    const currentUser = userRef.current
    if (currentUser && currentSession) {
      const scoreSessionId = `interview-${currentUser.user_id}`
      try {
        const res = await fetch(`/api/user/weight-scores?user_id=${currentUser.user_id}&session_id=${encodeURIComponent(scoreSessionId)}`)
        const data = await res.json()
        setScoresBefore(data.weight_scores ?? null)
      } catch { /* ignore */ }
      try { await interviewApi.finishSession(currentSession.id, currentUser.user_id) } catch { /* ignore */ }
    }
    setStatus('finished')
    setReportStatus('pending')
    if (forced) setErrorMessage('時間上限に達したため面接を終了しました。')
    if (currentSession && currentUser) {
      startReportPolling(currentSession.id, currentUser.user_id)

      // Upload video asynchronously
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
    }
  }

  const stopReportPolling = () => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }

  const loadScoresAfter = async (userId: number) => {
    const scoreSessionId = `interview-${userId}`
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
    setReportStatus('pending')

    const tick = async () => {
      if (generation !== pollGenerationRef.current) return

      const startedAt = pollStartedAtRef.current ?? Date.now()
      let hasReport = false
      let fetchFailed = false
      let reportPayload: InterviewReport | null = null

      try {
        const detail = await interviewApi.getDetail(sessionId, userId)
        if (detail.report) {
          hasReport = true
          reportPayload = detail.report
        }
      } catch {
        fetchFailed = true
      }

      if (generation !== pollGenerationRef.current) return

      const outcome = evaluateReportPollTick({
        startedAtMs: startedAt,
        nowMs: Date.now(),
        timeoutMs: REPORT_POLL_TIMEOUT_MS,
        hasReport,
        fetchFailed,
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

  const retryReportPolling = () => {
    const target = pollSessionRef.current
    if (!target) {
      if (session && user) {
        startReportPolling(session.id, user.user_id)
      }
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
    const mr = new MediaRecorder(micStream, { mimeType: 'audio/webm', audioBitsPerSecond: 128000 })
    mr.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
    mr.onstop = () => { void sendTurn() }
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

  const sendTurn = async () => {
    if (!user || !session) { setTurnPending(false); return }
    const chunks = audioChunksRef.current
    if (chunks.length === 0) { setTurnPending(false); return }
    const audioBlob = new Blob(chunks, { type: 'audio/webm' })
    const formData = new FormData()
    formData.append('audio', audioBlob, 'audio.webm')
    formData.append('user_id', String(user.user_id))
    formData.append('history', JSON.stringify(historyRef.current))
    formData.append('turn_count', String(Math.floor(historyRef.current.length / 2) + 1))
    formData.append('remaining_seconds', String(remainingSeconds))
    formData.append('question_index', String(currentQuestionIndex))
    formData.append('total_questions', String(Math.max(1, selectedPosition.questions)))
    formData.append('question_elapsed_seconds', String(questionElapsedSeconds))
    formData.append('question_duration_seconds', String(Math.max(60, interviewLimits.questionDurationSeconds || 180)))
    formData.append('company_name', interviewCompany?.name || '')
    formData.append('company_reading', interviewCompany?.name_reading || '')
    formData.append('position', selectedPosition?.title || '')
    formData.append('company_info', buildCompanyInfo(interviewCompany))
    formData.append('company_type', selectedPosition?.category || 'general')
    formData.append('company_id', String(interviewCompany?.id || 0))
    try {
      await authService.ensureFreshUserToken()
      const res = await fetch(`${BACKEND_URL}/api/interviews/${session.id}/turn`, {
        method: 'POST',
        headers: { ...authService.getUserFetchHeaders() },
        body: formData,
      })
      if (!res.ok) throw new Error(await res.text())
      const { meta, audio } = await parseMultipartResponse(res)
      const userText: string = meta.user_text || ''
      const aiText: string = meta.ai_text || ''
      setIsDeepeningQuestion(Boolean(meta.is_deepening))
      setQuestionCategory(typeof meta.question_category === 'string' ? meta.question_category : null)
      if (userText) {
        historyRef.current.push({ role: 'user', content: userText })
        setUtterances(p => [...p, { role: 'user', text: userText }])
        try { await interviewApi.saveUtterance(session.id, user.user_id, 'user', userText) } catch (e) { console.error('[utterance save error]', e) }
      }
      if (aiText) {
        historyRef.current.push({ role: 'assistant', content: aiText })
        setUtterances(p => [...p, { role: 'ai', text: aiText }])
        try { await interviewApi.saveUtterance(session.id, user.user_id, 'ai', aiText) } catch (e) { console.error('[utterance save error]', e) }
      }
      await playAudioBlob(audio)
    } catch (e: unknown) {
      setErrorMessage(parseMediaError(e))
    } finally {
      setTurnPending(false)
    }
  }

  const sendReportEmail = async () => {
    if (!session || !user) return
    setEmailSending(true)
    try {
      await interviewApi.sendReportEmail(session.id, user.user_id)
      setEmailSent(true)
    } catch {
      // ignore
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
    retryReportPolling,
    emailSending,
    emailSent,
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
