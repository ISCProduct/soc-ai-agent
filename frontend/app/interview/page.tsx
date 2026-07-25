'use client'

import { useEffect, useRef, useState, Suspense, useCallback } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService, User } from '@/lib/auth'
import { interviewApi, interviewLimits, InterviewReport, InterviewSession } from '@/lib/interview'
import { parseJsonSafe, parseMediaError, parseMultipartResponse } from '@/lib/interview-utils'
import ConsentDialog from './components/ConsentDialog'
import SelectionScreen from './components/SelectionScreen'
import LobbyScreen from './components/LobbyScreen'
import ReportScreen from './components/ReportScreen'
import SessionScreen from './components/SessionScreen'
import { WeightScore } from '@/components/ScoreUpdateBanner'
import { PageLoading } from '@/components/common/PageLoading'
import { POSITIONS } from './constants'
import type { Utterance, InterviewCompany, Position, InterviewStatus } from './types'
import { resolveCompanyByName, getNextAvatarGender } from './utils'

function InterviewContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [status, setStatus] = useState<InterviewStatus>('selection')
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
  const [reportStatus, setReportStatus] = useState<'idle' | 'pending' | 'ready' | 'error'>('idle')
  const [emailSending, setEmailSending] = useState(false)
  const [emailSent, setEmailSent] = useState(false)
  const [aiLevel, setAiLevel] = useState(0)
  const [aiSpeaking, _setAiSpeaking] = useState(false)
  const [avatarGender, setAvatarGender] = useState<'male' | 'female'>('male')
  const [interviewCompany, setInterviewCompany] = useState<InterviewCompany | null>(null)
  const [micEnabled, setMicEnabled] = useState(true)
  const [cameraEnabled, setCameraEnabled] = useState(true)
  const [noteInput, setNoteInput] = useState('')
  const [lobbyPermissionError, setLobbyPermissionError] = useState<string | null>(null)
  const [captionsVisible, setCaptionsVisible] = useState(true)
  // Selection screen state
  const [allCompanies, setAllCompanies] = useState<InterviewCompany[]>([])
  const [companiesLoading, setCompaniesLoading] = useState(false)
  const [companySearch, setCompanySearch] = useState('')
  const [selectedPosition, setSelectedPosition] = useState<Position>(POSITIONS[0])
  const [companySourceTab, setCompanySourceTab] = useState<'db' | 'web'>('db')
  const [webSearchResults, setWebSearchResults] = useState<{ name: string; description: string }[]>([])
  const [webSearchLoading, setWebSearchLoading] = useState(false)
  const [positionCategory, setPositionCategory] = useState<'general' | 'sier'>('general')
  const [companyHints, setCompanyHints] = useState<{ style_tags: string[]; top_questions: string[]; company_brief?: string } | null>(null)
  const [hintsLoading, setHintsLoading] = useState(false)

  const [isRecording, _setIsRecording] = useState(false)
  const [turnPending, _setTurnPending] = useState(false)

  const [videoUploadStatus, setVideoUploadStatus] = useState<'idle' | 'uploading' | 'done' | 'error'>('idle')
  const [videoUploadProgress, setVideoUploadProgress] = useState(0)
  const [videoSizeWarning, setVideoSizeWarning] = useState<string | null>(null)

  const [scoresBefore, setScoresBefore] = useState<WeightScore[] | null>(null)
  const [scoresAfter, setScoresAfter] = useState<WeightScore[] | null>(null)

  const streamRef = useRef<MediaStream | null>(null)
  const lobbyVideoRef = useRef<HTMLVideoElement | null>(null)
  const sessionVideoRef = useRef<HTMLVideoElement | null>(null)
  // video 要素がマウントした瞬間にストリームをアタッチするための callback ref。
  // useEffect([status]) では DOM コミット前に status が更新されるため srcObject が設定されないことがある。
  const sessionVideoCallbackRef = useCallback((node: HTMLVideoElement | null) => {
    sessionVideoRef.current = node
    if (node && streamRef.current) {
      node.srcObject = streamRef.current
      node.play().catch(() => undefined)
    }
  }, [])
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const videoRecorderRef = useRef<MediaRecorder | null>(null)
  const videoChunksRef = useRef<Blob[]>([])
  const historyRef = useRef<{ role: string; content: string }[]>([])
  const aiAudioRef = useRef<HTMLAudioElement | null>(null)
  const aiAudioCtxRef = useRef<AudioContext | null>(null)
  const aiLevelRafRef = useRef<number | null>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const sessionStartRef = useRef<number | null>(null)
  const transcriptEndRef = useRef<HTMLDivElement | null>(null)
  // ハンズフリーVAD用
  const [handsFreeMode, setHandsFreeMode] = useState(false)
  const [consentDialogOpen, setConsentDialogOpen] = useState(false)
  const [consentGiven, setConsentGiven] = useState(false)
  const isRecordingRef = useRef(false)
  const turnPendingRef = useRef(false)
  const aiSpeakingRef = useRef(false)

  // Auth check
  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) { router.replace('/login'); return }
    setUser(storedUser)
    setLoading(false)
  }, [router])

  // マッチング結果から遷移した場合: URL パラメータで企業を事前選択してロビーへ
  useEffect(() => {
    const companyNameParam = searchParams.get('company_name')
    const industry = searchParams.get('industry')
    const companyId = searchParams.get('company_id')
    if (!companyNameParam || loading) return
    const parsedId = companyId ? parseInt(companyId, 10) : 0
    setInterviewCompany({
      id: Number.isFinite(parsedId) ? parsedId : 0,
      name: companyNameParam,
      industry: industry || undefined,
    })
    setStatus('lobby')
    if (!companyId || parsedId === 0) {
      void resolveCompanyByName(companyNameParam, { industry: industry || undefined }).then((resolved) => {
        setInterviewCompany((prev) =>
          prev?.name === companyNameParam ? resolved : prev,
        )
      })
    }
  }, [loading, searchParams])

  // Load company list for selection screen (initial fetch + debounced search)
  useEffect(() => {
    if (loading || companySourceTab !== 'db') return
    let cancelled = false
    const timer = setTimeout(() => {
      setCompaniesLoading(true)
      const params = new URLSearchParams({ limit: '50', offset: '0' })
      if (companySearch.trim()) params.set('name', companySearch.trim())
      fetch(`/api/companies?${params}`, { cache: 'no-store' })
        .then(r => r.ok ? r.json() : null)
        .then(data => {
          if (cancelled) return
          const list: InterviewCompany[] = Array.isArray(data?.companies) ? data.companies : []
          setAllCompanies(list)
          if (list.length > 0 && !interviewCompany) setInterviewCompany(list[0])
        })
        .catch(() => { /* ignore */ })
        .finally(() => { if (!cancelled) setCompaniesLoading(false) })
    }, companySearch ? 400 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [loading, companySearch, companySourceTab]) // eslint-disable-line react-hooks/exhaustive-deps

  // 企業別面接傾向ヒント取得
  useEffect(() => {
    if (!interviewCompany?.name) { setCompanyHints(null); return }
    let cancelled = false
    const timer = setTimeout(() => {
      setHintsLoading(true)
      fetch('/api/companies/interview-hints', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ company_name: interviewCompany.name, position: selectedPosition.title }),
      })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (!cancelled && data) setCompanyHints(data) })
        .catch(() => {})
        .finally(() => { if (!cancelled) setHintsLoading(false) })
    }, 600)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [interviewCompany?.name, selectedPosition.title]) // eslint-disable-line react-hooks/exhaustive-deps

  // WEB検索
  useEffect(() => {
    if (loading || companySourceTab !== 'web') return
    if (!companySearch.trim()) { setWebSearchResults([]); return }
    let cancelled = false
    const timer = setTimeout(() => {
      setWebSearchLoading(true)
      fetch(`/api/companies/web-search?q=${encodeURIComponent(companySearch.trim())}`, { cache: 'no-store' })
        .then(r => r.ok ? r.json() : null)
        .then(data => {
          if (cancelled) return
          setWebSearchResults(Array.isArray(data?.results) ? data.results : [])
        })
        .catch(() => { if (!cancelled) setWebSearchResults([]) })
        .finally(() => { if (!cancelled) setWebSearchLoading(false) })
    }, 500)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [loading, companySearch, companySourceTab]) // eslint-disable-line react-hooks/exhaustive-deps

  // Lobby camera preview
  useEffect(() => {
    if (loading) return
    let stream: MediaStream | null = null
    const startPreview = async () => {
      try {
        stream = await navigator.mediaDevices.getUserMedia({ audio: { sampleRate: 48000, channelCount: 1, echoCancellation: true, noiseSuppression: true }, video: true })
        streamRef.current = stream
        if (lobbyVideoRef.current) {
          lobbyVideoRef.current.srcObject = stream
          lobbyVideoRef.current.play().catch(() => undefined)
        }
      } catch (err: any) {
        if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
          // カメラとマイクを個別に試してどちらがブロックされているか特定する
          const blocked: string[] = []
          await navigator.mediaDevices.getUserMedia({ video: true }).then(s => s.getTracks().forEach(t => t.stop())).catch(() => { blocked.push('カメラ') })
          await navigator.mediaDevices.getUserMedia({ audio: true }).then(s => s.getTracks().forEach(t => t.stop())).catch(() => { blocked.push('マイク') })
          const target = blocked.length > 0 ? blocked.join('と') : 'マイクとカメラ'
          setLobbyPermissionError(`${target}へのアクセスが拒否されました。`)
        } else if (err.name === 'NotFoundError') {
          setLobbyPermissionError('マイクまたはカメラが見つかりません。デバイスを確認してください。')
        } else {
          setLobbyPermissionError('カメラの起動に失敗しました。')
        }
      }
    }
    startPreview()
    return () => {
      // stream is kept in streamRef for reuse during interview
    }
  }, [loading])

  // Cleanup on unmount
  useEffect(() => () => cleanupConnection(), [])

  // Auto-scroll transcript
  useEffect(() => {
    transcriptEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [utterances, partialAi])

  // Attach stream to session video when connected
  useEffect(() => {
    if (status === 'connected' && sessionVideoRef.current && streamRef.current) {
      sessionVideoRef.current.srcObject = streamRef.current
      sessionVideoRef.current.play().catch(() => undefined)
    }
  }, [status])

  // ハンズフリーVAD: 音声検知で自動録音開始・停止
  useEffect(() => {
    if (!handsFreeMode || status !== 'connected' || !streamRef.current) return
    const VAD_THRESHOLD = 0.015   // 発話検知の音量閾値 (RMS)
    const SILENCE_MS = 2500       // この無音時間が続いたら自動送信（長めに設定して途切れ防止）
    const MIN_RECORDING_MS = 1000 // 録音開始後この時間は自動停止しない（息継ぎなどで誤停止しない）
    const audioCtx = new AudioContext()
    const source = audioCtx.createMediaStreamSource(streamRef.current)
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 512
    source.connect(analyser)
    const buf = new Float32Array(analyser.fftSize)
    let silenceStart: number | null = null
    let recordingStartTime: number | null = null
    let rafId: number
    const tick = () => {
      rafId = requestAnimationFrame(tick)
      analyser.getFloatTimeDomainData(buf)
      const rms = Math.sqrt(buf.reduce((s, v) => s + v * v, 0) / buf.length)
      const speaking = rms > VAD_THRESHOLD
      if (speaking) {
        silenceStart = null
        if (!isRecordingRef.current && !turnPendingRef.current && !aiSpeakingRef.current) {
          recordingStartTime = Date.now()
          startRecording()
        }
      } else if (isRecordingRef.current) {
        // 録音開始直後の短い無音（息継ぎ等）では止めない
        const elapsed = recordingStartTime ? Date.now() - recordingStartTime : Infinity
        if (elapsed < MIN_RECORDING_MS) return
        if (silenceStart === null) {
          silenceStart = Date.now()
        } else if (Date.now() - silenceStart > SILENCE_MS) {
          silenceStart = null
          recordingStartTime = null
          stopRecording()
        }
      } else {
        silenceStart = null
      }
    }
    rafId = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(rafId)
      source.disconnect()
      audioCtx.close().catch(() => {})
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handsFreeMode, status])

  const cleanupConnection = () => {
    ;[timerRef, pollRef].forEach(r => { if (r.current) { clearInterval(r.current); r.current = null } })
    if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
      mediaRecorderRef.current.stop(); mediaRecorderRef.current = null
    }
    if (aiAudioRef.current) { aiAudioRef.current.pause(); aiAudioRef.current.src = '' }
    if (aiLevelRafRef.current !== null) { cancelAnimationFrame(aiLevelRafRef.current); aiLevelRafRef.current = null }
    if (aiAudioCtxRef.current) { aiAudioCtxRef.current.close().catch(() => {}); aiAudioCtxRef.current = null }
    setAiLevel(0)
    if (streamRef.current) { streamRef.current.getTracks().forEach(t => t.stop()); streamRef.current = null }
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
    const url = URL.createObjectURL(blob)
    const el = new Audio()
    aiAudioRef.current = el
    el.src = url
    setAiSpeaking(true)

    let rafId: number | null = null
    let routedThroughCtx = false

    try {
      if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
        aiAudioCtxRef.current = new AudioContext()
      }
      const ctx = aiAudioCtxRef.current
      // resume を await して running 状態を確実に待つ（suspended のまま再生すると無音になる）
      await ctx.resume()

      const source   = ctx.createMediaElementSource(el)
      routedThroughCtx = true
      const analyser = ctx.createAnalyser()
      analyser.fftSize = 512
      analyser.smoothingTimeConstant = 0.6
      source.connect(analyser)
      analyser.connect(ctx.destination)

      const timeData = new Uint8Array(analyser.fftSize)
      const trackLevel = () => {
        analyser.getByteTimeDomainData(timeData)
        let sum = 0
        for (const v of timeData) { const n = (v - 128) / 128; sum += n * n }
        const rms = Math.sqrt(sum / timeData.length)
        setAiLevel(Math.min(1, rms * 6))
        rafId = requestAnimationFrame(trackLevel)
      }
      rafId = requestAnimationFrame(trackLevel)
      aiLevelRafRef.current = rafId
    } catch {
      // AudioContext 未対応またはセキュリティポリシーで拒否された場合はリップシンク無効で続行
      if (!routedThroughCtx) {
        // AudioContext を経由していないので Audio 要素がデフォルト出力に直接流れる
      }
    }

    const cleanup = () => {
      if (rafId !== null) cancelAnimationFrame(rafId)
      if (aiLevelRafRef.current !== null) cancelAnimationFrame(aiLevelRafRef.current)
      aiLevelRafRef.current = null
      setAiLevel(0)
    }

    return new Promise<void>((resolve) => {
      el.onended = () => { cleanup(); setAiSpeaking(false); URL.revokeObjectURL(url); resolve() }
      el.onerror = () => { cleanup(); setAiSpeaking(false); URL.revokeObjectURL(url); resolve() }
      el.play().catch(() => { cleanup(); setAiSpeaking(false); resolve() })
    })
  }

  const doStartTurn = async (sessionId: number, userId: number) => {
    const res = await fetch(`${BACKEND_URL}/api/interviews/${sessionId}/start-turn`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
      body: JSON.stringify({
        user_id: userId,
        company_name: interviewCompany?.name || '',
        company_reading: interviewCompany?.name_reading || '',
        position: selectedPosition?.title || '',
        company_info: [
          interviewCompany?.description,
          interviewCompany?.main_business,
          interviewCompany?.culture && `企業文化: ${interviewCompany.culture}`,
          interviewCompany?.work_style && `働き方: ${interviewCompany.work_style}`,
          interviewCompany?.welfare_details && `福利厚生: ${interviewCompany.welfare_details}`,
        ].filter(Boolean).join(' / '),
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
    setMicEnabled(true); setCameraEnabled(true)
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

      // Acquire camera/mic stream
      let stream = streamRef.current
      if (!stream) {
        try {
          stream = await navigator.mediaDevices.getUserMedia({ audio: { sampleRate: 48000, channelCount: 1, echoCancellation: true, noiseSuppression: true }, video: true })
        } catch (err: any) {
          if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') throw new Error('NotAllowedError')
          if (err.name === 'NotFoundError') throw new Error('NotFoundError')
          throw err
        }
        streamRef.current = stream
      }

      const created = await interviewApi.createSession(user.user_id, 'ja', nextGender)
      setSession(created)
      await interviewApi.startSession(created.id, user.user_id)
      setStatus('connected')

      // Start video recording
      if (stream) {
        try {
          const mimeType = MediaRecorder.isTypeSupported('video/webm;codecs=vp9') ? 'video/webm;codecs=vp9' : 'video/webm'
          videoChunksRef.current = []
          // ビットレートを制限して 10 分録画でも約 25 MB 以内に収める
          // video: 300 kbps + audio: 32 kbps ≈ 332 kbps → 10 分 ≈ 24.9 MB
          const vr = new MediaRecorder(stream, {
            mimeType,
            videoBitsPerSecond: 300_000,
            audioBitsPerSecond: 32_000,
          })
          vr.ondataavailable = (e) => { if (e.data.size > 0) videoChunksRef.current.push(e.data) }
          vr.start(1000)
          videoRecorderRef.current = vr
        } catch { /* camera unavailable — skip recording */ }
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
    // Stop video recorder and collect blob before cleanup
    const vr = videoRecorderRef.current
    let videoBlob: Blob | null = null
    if (vr && vr.state !== 'inactive') {
      await new Promise<void>((resolve) => {
        vr.onstop = () => resolve()
        vr.stop()
      })
      if (videoChunksRef.current.length > 0) {
        videoBlob = new Blob(videoChunksRef.current, { type: 'video/webm' })
      }
      videoRecorderRef.current = null
      videoChunksRef.current = []
    }

    cleanupConnection()
    const currentSession = session
    const currentUser = user
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

  const startReportPolling = (sessionId: number, userId: number) => {
    if (pollRef.current) clearInterval(pollRef.current)
    pollRef.current = setInterval(async () => {
      try {
        const detail = await interviewApi.getDetail(sessionId, userId)
        if (detail.report) {
          setReport(detail.report); setReportStatus('ready')
          clearInterval(pollRef.current!); pollRef.current = null
          const scoreSessionId = `interview-${userId}`
          try {
            const res = await fetch(`/api/user/weight-scores?user_id=${userId}&session_id=${encodeURIComponent(scoreSessionId)}`)
            const data = await res.json()
            setScoresAfter(data.weight_scores ?? null)
          } catch { /* ignore */ }
        }
      } catch { setReportStatus('error') }
    }, 3000)
  }

  // ref と state を常に同期（VAD の stale closure 対策）
  const setIsRecording = (v: boolean) => { isRecordingRef.current = v; _setIsRecording(v) }
  const setTurnPending = (v: boolean) => { turnPendingRef.current = v; _setTurnPending(v) }
  const setAiSpeaking = (v: boolean) => { aiSpeakingRef.current = v; _setAiSpeaking(v) }

  const startRecording = () => {
    if (!streamRef.current || isRecordingRef.current || turnPendingRef.current) return
    const audioTracks = streamRef.current.getAudioTracks()
    if (audioTracks.length === 0) return
    const micStream = new MediaStream(audioTracks)
    audioChunksRef.current = []
    const mr = new MediaRecorder(micStream, { mimeType: 'audio/webm' })
    mr.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
    mr.onstop = () => { void sendTurn() }
    mediaRecorderRef.current = mr
    mr.start()
    setIsRecording(true)
  }

  const stopRecording = () => {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === 'inactive') return
    mediaRecorderRef.current.stop()
    setIsRecording(false)
    setTurnPending(true)
  }

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
    formData.append('company_info', [
      interviewCompany?.description,
      interviewCompany?.main_business,
      interviewCompany?.culture && `企業文化: ${interviewCompany.culture}`,
      interviewCompany?.work_style && `働き方: ${interviewCompany.work_style}`,
      interviewCompany?.welfare_details && `福利厚生: ${interviewCompany.welfare_details}`,
    ].filter(Boolean).join(' / '))
    formData.append('company_type', selectedPosition?.category || 'general')
    formData.append('company_id', String(interviewCompany?.id || 0))
    try {
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
    } catch (e: any) {
      setErrorMessage(e.message || '通信エラーが発生しました')
    } finally {
      setTurnPending(false)
    }
  }

  const toggleMic = () => {
    if (!streamRef.current) return
    const next = !micEnabled
    streamRef.current.getAudioTracks().forEach(t => { t.enabled = next })
    setMicEnabled(next)
  }

  const toggleCamera = () => {
    if (!streamRef.current) return
    const next = !cameraEnabled
    streamRef.current.getVideoTracks().forEach(t => { t.enabled = next })
    setCameraEnabled(next)
  }

  if (loading || !user) {
    return <PageLoading message="面接画面を準備しています..." />
  }

  const isActive = status === 'connecting' || status === 'connected'
  const isConnected = status === 'connected'
  const questionDurationSeconds = Math.max(60, interviewLimits.questionDurationSeconds || 180)
  const totalQuestionCount = Math.max(1, selectedPosition.questions)
  const questionProgress = Math.min(100, Math.round((questionElapsedSeconds / questionDurationSeconds) * 100))
  const questionRemainingSeconds = Math.max(0, questionDurationSeconds - questionElapsedSeconds)
  const questionRemainingLabel = (() => {
    if (questionRemainingSeconds <= 0) return '次の質問へ移行中...'
    if (questionRemainingSeconds < 60) return `あと${questionRemainingSeconds}秒で次の質問へ`
    const m = Math.floor(questionRemainingSeconds / 60)
    const s = questionRemainingSeconds % 60
    return s > 0 ? `あと${m}分${s}秒で次の質問へ` : `あと${m}分で次の質問へ`
  })()
  const progress = Math.min(100, Math.round(((interviewLimits.maxMinutes * 60 - remainingSeconds) / (interviewLimits.maxMinutes * 60)) * 100))
  const isFemale = avatarGender === 'female'
  const companyName = interviewCompany?.name || 'AI面接練習'
  const latestAiText = partialAi || (utterances.filter(u => u.role === 'ai').slice(-1)[0]?.text ?? '')
  const recruitingText = [
    '【募集背景】', interviewCompany?.description || '企業情報の取得後に表示されます。', '',
    '【仕事内容】', interviewCompany?.main_business || interviewCompany?.industry || '詳細は面接内でご案内します。', '',
    '【職場環境】', interviewCompany?.work_style || '勤務形態は選考でご説明します。', '',
    '【企業文化・福利厚生】',
    `${interviewCompany?.culture || 'チームで成果を重視する文化'} / ${interviewCompany?.welfare_details || '情報準備中'}`,
    '', `【勤務地・人数】 ${interviewCompany?.location || '未設定'} / ${interviewCompany?.employee_count ? interviewCompany.employee_count + '名' : '非公開'}`,
  ].join('\n')

  // ─────────────────────────────────────────────
  // SELECTION SCREEN  (Step 1 of 3)
  // ─────────────────────────────────────────────
  if (status === 'selection') {
    return (
      <SelectionScreen
        user={user}
        onBack={() => router.push('/')}
        interviewCompany={interviewCompany}
        setInterviewCompany={setInterviewCompany}
        companySourceTab={companySourceTab}
        setCompanySourceTab={setCompanySourceTab}
        companySearch={companySearch}
        setCompanySearch={setCompanySearch}
        allCompanies={allCompanies}
        setAllCompanies={setAllCompanies}
        companiesLoading={companiesLoading}
        webSearchResults={webSearchResults}
        setWebSearchResults={setWebSearchResults}
        webSearchLoading={webSearchLoading}
        positionCategory={positionCategory}
        setPositionCategory={setPositionCategory}
        selectedPosition={selectedPosition}
        setSelectedPosition={setSelectedPosition}
        companyHints={companyHints}
        hintsLoading={hintsLoading}
        questionDurationSeconds={questionDurationSeconds}
        onStartInterview={() => setStatus('lobby')}
      />
    )
  }


  // ─────────────────────────────────────────────
  // 録画同意ダイアログ（LOBBY / SESSION 画面で共有）
  // ─────────────────────────────────────────────
  const consentDialog = (
    <ConsentDialog
      open={consentDialogOpen}
      consentGiven={consentGiven}
      onConsentChange={setConsentGiven}
      onClose={() => setConsentDialogOpen(false)}
      onConfirm={() => { setConsentDialogOpen(false); handleJoin() }}
    />
  )

  // ─────────────────────────────────────────────
  // LOBBY SCREEN
  // ─────────────────────────────────────────────
  if (status === 'lobby') {
    return (
      <LobbyScreen
        userName={user.name}
        companyName={companyName}
        interviewCompany={interviewCompany}
        fromMatchingResults={Boolean(searchParams.get('company_name'))}
        lobbyPermissionError={lobbyPermissionError}
        onRetryPermissions={() => { setLobbyPermissionError(null); window.location.reload() }}
        micEnabled={micEnabled}
        cameraEnabled={cameraEnabled}
        onToggleMic={toggleMic}
        onToggleCamera={toggleCamera}
        lobbyVideoRef={lobbyVideoRef}
        onBack={() => router.push('/')}
        onJoinWithConsent={handleJoinWithConsent}
        consentDialog={consentDialog}
      />
    )
  }

  // ─────────────────────────────────────────────
  // REPORT SCREEN (finished)
  // ─────────────────────────────────────────────
  if (status === 'finished') {
    return (
      <ReportScreen
        onBack={() => router.push('/')}
        errorMessage={errorMessage}
        reportStatus={reportStatus}
        report={report}
        scoresBefore={scoresBefore}
        scoresAfter={scoresAfter}
        session={session}
        userId={user?.user_id}
        emailSending={emailSending}
        emailSent={emailSent}
        isGuest={!user || user.is_guest}
        onSendEmail={async () => {
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
        }}
        videoUploadStatus={videoUploadStatus}
        videoUploadProgress={videoUploadProgress}
        videoSizeWarning={videoSizeWarning}
      />
    )
  }

  // ─────────────────────────────────────────────
  // SESSION SCREEN (connecting / connected / error)
  // ─────────────────────────────────────────────
  return (
    <SessionScreen
      status={status}
      companyName={companyName}
      userName={user.name}
      isActive={isActive}
      isConnected={isConnected}
      remainingSeconds={remainingSeconds}
      sessionWarningShown={sessionWarningShown}
      currentQuestionIndex={currentQuestionIndex}
      totalQuestionCount={totalQuestionCount}
      questionProgress={questionProgress}
      questionRemainingSeconds={questionRemainingSeconds}
      questionRemainingLabel={questionRemainingLabel}
      isDeepeningQuestion={isDeepeningQuestion}
      questionCategory={questionCategory}
      avatarGender={avatarGender}
      aiLevel={aiLevel}
      aiSpeaking={aiSpeaking}
      cameraEnabled={cameraEnabled}
      captionsVisible={captionsVisible}
      handsFreeMode={handsFreeMode}
      utterances={utterances}
      partialAi={partialAi}
      partialUser={partialUser}
      isRecording={isRecording}
      turnPending={turnPending}
      errorMessage={errorMessage}
      sessionVideoCallbackRef={sessionVideoCallbackRef}
      transcriptEndRef={transcriptEndRef}
      aiAudioRef={aiAudioRef}
      consentDialog={consentDialog}
      onToggleCamera={toggleCamera}
      onToggleCaptions={() => setCaptionsVisible(p => !p)}
      onToggleHandsFree={() => setHandsFreeMode(p => !p)}
      onStartRecording={startRecording}
      onStopRecording={stopRecording}
      onJoin={handleJoin}
      onStop={handleStop}
    />
  )
}

export default function InterviewPage() {
  return (
    <Suspense fallback={<PageLoading message="面接画面を準備しています..." />}>
      <InterviewContent />
    </Suspense>
  )
}
