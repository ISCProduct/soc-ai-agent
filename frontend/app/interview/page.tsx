'use client'

import { useEffect, useRef, useState, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Drawer,
  FormControlLabel,
  Checkbox,
  IconButton,
  InputBase,
  LinearProgress,
  Paper,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import MicOffIcon from '@mui/icons-material/MicOff'
import VolumeUpIcon from '@mui/icons-material/VolumeUp'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import CallEndIcon from '@mui/icons-material/CallEnd'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import RefreshIcon from '@mui/icons-material/Refresh'
import ClosedCaptionIcon from '@mui/icons-material/ClosedCaption'
import PsychologyIcon from '@mui/icons-material/Psychology'
import LightbulbIcon from '@mui/icons-material/Lightbulb'
import SendIcon from '@mui/icons-material/Send'
import SearchIcon from '@mui/icons-material/Search'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ApartmentIcon from '@mui/icons-material/Apartment'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import { authService, User } from '@/lib/auth'
import { interviewApi, interviewLimits, InterviewReport, InterviewSession } from '@/lib/interview'
import { formatSeconds, parseJsonSafe, parseMediaError, parseMultipartResponse } from '@/lib/interview-utils'
import ThreeAvatar from './components/ThreeAvatar'
import InterviewSummary from './components/InterviewSummary'

const PRIMARY = '#ec5b13'
const BG_LIGHT = '#f8f6f6'
const BG_DARK = '#221610'

type Utterance = {
  role: 'user' | 'ai'
  text: string
}

type InterviewCompany = {
  id: number
  name: string
  name_reading?: string
  description?: string
  main_business?: string
  industry?: string
  location?: string
  employee_count?: number
  culture?: string
  work_style?: string
  welfare_details?: string
}

type Position = {
  id: string
  title: string
  department: string
  icon: string
  questions: number
  category: 'general' | 'sier'
}

const POSITIONS: Position[] = [
  { id: 'engineer', title: 'ソフトウェアエンジニア', department: 'Engineering', icon: '💻', questions: 8, category: 'general' },
  { id: 'designer', title: 'プロダクトデザイナー', department: 'Design', icon: '🎨', questions: 7, category: 'general' },
  { id: 'sales', title: '営業職', department: 'Sales', icon: '📈', questions: 7, category: 'general' },
  { id: 'marketing', title: 'マーケティング', department: 'Growth', icon: '📣', questions: 6, category: 'general' },
  { id: 'pm', title: 'プロダクトマネージャー', department: 'Product', icon: '🧭', questions: 9, category: 'general' },
  { id: 'data', title: 'データアナリスト', department: 'Data', icon: '📊', questions: 7, category: 'general' },
  { id: 'se', title: 'システムエンジニア（SE）', department: 'SIer / Development', icon: '🖥️', questions: 8, category: 'sier' },
  { id: 'infra', title: 'インフラエンジニア', department: 'SIer / Infrastructure', icon: '🔧', questions: 7, category: 'sier' },
  { id: 'it_consultant', title: 'ITコンサルタント', department: 'SIer / Consulting', icon: '📋', questions: 8, category: 'sier' },
  { id: 'pmo', title: 'PMO（プロジェクト管理）', department: 'SIer / PMO', icon: '📅', questions: 7, category: 'sier' },
  { id: 'network', title: 'ネットワークエンジニア', department: 'SIer / Network', icon: '🌐', questions: 7, category: 'sier' },
  { id: 'qa', title: 'テスト・品質保証（QA）', department: 'SIer / QA', icon: '✅', questions: 6, category: 'sier' },
]

function InterviewContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [status, setStatus] = useState<'selection' | 'lobby' | 'connecting' | 'connected' | 'error' | 'finished'>('selection')
  const [errorMessage, setErrorMessage] = useState<string | null>(null)
  const [utterances, setUtterances] = useState<Utterance[]>([])
  const [partialUser, setPartialUser] = useState('')
  const [partialAi, setPartialAi] = useState('')
  const [remainingSeconds, setRemainingSeconds] = useState(interviewLimits.maxMinutes * 60)
  const [elapsedSeconds, setElapsedSeconds] = useState(0)
  const [currentQuestionIndex, setCurrentQuestionIndex] = useState(1)
  const [questionElapsedSeconds, setQuestionElapsedSeconds] = useState(0)
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
  const [companyHints, setCompanyHints] = useState<{ style_tags: string[]; top_questions: string[] } | null>(null)
  const [hintsLoading, setHintsLoading] = useState(false)

  const [isRecording, _setIsRecording] = useState(false)
  const [turnPending, _setTurnPending] = useState(false)

  const [videoUploadStatus, setVideoUploadStatus] = useState<'idle' | 'uploading' | 'done' | 'error'>('idle')
  const [videoUploadProgress, setVideoUploadProgress] = useState(0)
  const [videoSizeWarning, setVideoSizeWarning] = useState<string | null>(null)

  const streamRef = useRef<MediaStream | null>(null)
  const lobbyVideoRef = useRef<HTMLVideoElement | null>(null)
  const sessionVideoRef = useRef<HTMLVideoElement | null>(null)
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
    const companyName = searchParams.get('company_name')
    const industry = searchParams.get('industry')
    const companyId = searchParams.get('company_id')
    if (!companyName || loading) return
    setInterviewCompany({
      id: companyId ? parseInt(companyId, 10) : 0,
      name: companyName,
      industry: industry || undefined,
    })
    setStatus('lobby')
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
        stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: true })
        streamRef.current = stream
        if (lobbyVideoRef.current) {
          lobbyVideoRef.current.srcObject = stream
          lobbyVideoRef.current.play().catch(() => undefined)
        }
      } catch (err: any) {
        if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
          setLobbyPermissionError('マイクとカメラへのアクセスが拒否されました。ブラウザの設定から許可してください。')
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
    return () => { cancelAnimationFrame(rafId); audioCtx.close() }
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

  const playAudioBlob = (blob: Blob): Promise<void> => {
    return new Promise((resolve) => {
      const url = URL.createObjectURL(blob)
      // Always create a fresh Audio element so createMediaElementSource can be called each time
      const el = new Audio()
      aiAudioRef.current = el
      el.src = url
      setAiSpeaking(true)

      // Set up AudioContext for real-time amplitude analysis (drives aiLevel / lipsync)
      let rafId: number | null = null
      try {
        if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
          aiAudioCtxRef.current = new AudioContext()
        }
        const ctx = aiAudioCtxRef.current
        if (ctx.state === 'suspended') ctx.resume().catch(() => {})

        const source   = ctx.createMediaElementSource(el)
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
      } catch { /* AudioContext not supported – lipsync disabled */ }

      const cleanup = () => {
        if (rafId !== null) cancelAnimationFrame(rafId)
        if (aiLevelRafRef.current !== null) cancelAnimationFrame(aiLevelRafRef.current)
        aiLevelRafRef.current = null
        setAiLevel(0)
      }

      el.onended = () => { cleanup(); setAiSpeaking(false); URL.revokeObjectURL(url); resolve() }
      el.onerror = () => { cleanup(); setAiSpeaking(false); URL.revokeObjectURL(url); resolve() }
      el.play().catch(() => { cleanup(); setAiSpeaking(false); resolve() })
    })
  }

  const doStartTurn = async (sessionId: number, userId: number) => {
    const BACKEND = process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:80'
    const res = await fetch(`${BACKEND}/api/interviews/${sessionId}/start-turn`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        user_id: userId,
        company_name: interviewCompany?.name || '',
        company_reading: interviewCompany?.name_reading || '',
        position: selectedPosition?.title || '',
        company_info: [interviewCompany?.description, interviewCompany?.main_business].filter(Boolean).join(' / '),
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

      // Acquire camera/mic stream
      let stream = streamRef.current
      if (!stream) {
        try {
          stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: true })
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
    formData.append('company_info', [interviewCompany?.description, interviewCompany?.main_business].filter(Boolean).join(' / '))
    formData.append('company_type', selectedPosition?.category || 'general')
    try {
      const BACKEND = process.env.NEXT_PUBLIC_BACKEND_URL || 'http://localhost:80'
      const res = await fetch(`${BACKEND}/api/interviews/${session.id}/turn`, {
        method: 'POST',
        body: formData,
      })
      if (!res.ok) throw new Error(await res.text())
      const { meta, audio } = await parseMultipartResponse(res)
      const userText: string = meta.user_text || ''
      const aiText: string = meta.ai_text || ''
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

  if (loading || !user) return null

  const isActive = status === 'connecting' || status === 'connected'
  const isConnected = status === 'connected'
  const questionDurationSeconds = Math.max(60, interviewLimits.questionDurationSeconds || 180)
  const totalQuestionCount = Math.max(1, selectedPosition.questions)
  const questionProgress = Math.min(100, Math.round((questionElapsedSeconds / questionDurationSeconds) * 100))
  const questionRemainingSeconds = Math.max(0, questionDurationSeconds - questionElapsedSeconds)
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

  // allCompanies is already filtered by server-side search, no client-side filter needed
  const filteredCompanies = allCompanies

  // ─────────────────────────────────────────────
  // SELECTION SCREEN  (Step 1 of 3)
  // ─────────────────────────────────────────────
  if (status === 'selection') {
    return (
      <Box sx={{ minHeight: '100vh', bgcolor: BG_LIGHT }}>
        {/* Header */}
        <Box component="header" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: { xs: 3, lg: 10 }, py: 2, bgcolor: '#fff', borderBottom: '1px solid #e2e8f0' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box sx={{ color: PRIMARY, display: 'flex', alignItems: 'center' }}>
              <PsychologyIcon sx={{ fontSize: 32 }} />
            </Box>
            <Typography sx={{ fontWeight: 700, fontSize: 20, color: '#0f172a' }}>InterviewAI</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <IconButton sx={{ bgcolor: '#f1f5f9', color: '#475569' }} size="small" onClick={() => router.push('/')}>
              <ArrowBackIcon fontSize="small" />
            </IconButton>
            <Box sx={{ width: 40, height: 40, borderRadius: '50%', bgcolor: `${PRIMARY}30`, border: `1px solid ${PRIMARY}50`, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <Typography sx={{ fontWeight: 700, color: PRIMARY, fontSize: 14 }}>
                {(user.name || 'U').charAt(0).toUpperCase()}
              </Typography>
            </Box>
          </Box>
        </Box>

        {/* Main */}
        <Box component="main" sx={{ display: 'flex', justifyContent: 'center', py: 5, px: { xs: 3, lg: 10 } }}>
          <Box sx={{ maxWidth: 896, width: '100%', display: 'flex', flexDirection: 'column', gap: 4 }}>

            {/* Step indicator + Title */}
            <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                <Typography sx={{ color: PRIMARY, fontWeight: 600, fontSize: 13, textTransform: 'uppercase', letterSpacing: 1 }}>
                  Step 1 / 3
                </Typography>
                <Box sx={{ height: 4, width: 96, bgcolor: '#e2e8f0', borderRadius: 9999, overflow: 'hidden' }}>
                  <Box sx={{ height: '100%', width: '33%', bgcolor: PRIMARY }} />
                </Box>
              </Box>
              <Typography variant="h4" sx={{ fontWeight: 700, color: '#0f172a' }}>練習する企業・職種を選ぶ</Typography>
              <Typography sx={{ color: '#64748b', fontSize: 15 }}>
                志望企業と職種を選択して、AIが面接内容をカスタマイズします。
              </Typography>
            </Box>

            {/* 3-col grid */}
            <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '2fr 1fr' }, gap: 4, alignItems: 'start' }}>

              {/* Left: Company + Position */}
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>

                {/* Company section */}
                <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0' }}>
                  <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2 }}>志望企業</Typography>

                  {/* Source tab toggle */}
                  <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
                    <Button
                      size="small"
                      onClick={() => { setCompanySourceTab('db'); setWebSearchResults([]) }}
                      sx={{
                        px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                        bgcolor: companySourceTab === 'db' ? PRIMARY : '#f1f5f9',
                        color: companySourceTab === 'db' ? '#fff' : '#475569',
                        '&:hover': { bgcolor: companySourceTab === 'db' ? `${PRIMARY}e0` : '#e2e8f0' },
                      }}
                    >
                      🏢 企業管理から選択
                    </Button>
                    <Button
                      size="small"
                      onClick={() => { setCompanySourceTab('web'); setAllCompanies([]) }}
                      sx={{
                        px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                        bgcolor: companySourceTab === 'web' ? PRIMARY : '#f1f5f9',
                        color: companySourceTab === 'web' ? '#fff' : '#475569',
                        '&:hover': { bgcolor: companySourceTab === 'web' ? `${PRIMARY}e0` : '#e2e8f0' },
                      }}
                    >
                      🔍 WEB検索
                    </Button>
                  </Box>

                  {/* Search / direct input */}
                  <Box sx={{ position: 'relative', mb: 2 }}>
                    <SearchIcon sx={{ position: 'absolute', left: 12, top: '50%', transform: 'translateY(-50%)', color: '#94a3b8', fontSize: 20 }} />
                    <Box
                      component="input"
                      value={companySearch}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
                        setCompanySearch(e.target.value)
                        if (companySourceTab === 'db' && e.target.value.trim()) {
                          setInterviewCompany({ id: 0, name: e.target.value.trim() })
                        }
                      }}
                      placeholder={companySourceTab === 'web' ? '企業名を入力してWEB検索' : '企業名を入力、または下のリストから選択'}
                      sx={{
                        width: '100%', pl: '40px', pr: 2, py: 1.5,
                        bgcolor: '#f8fafc', border: '1px solid #e2e8f0',
                        borderRadius: 2, fontSize: 14, color: '#0f172a',
                        outline: 'none', boxSizing: 'border-box',
                        '&:focus': { borderColor: PRIMARY, boxShadow: `0 0 0 2px ${PRIMARY}20` },
                        fontFamily: 'inherit',
                      }}
                    />
                  </Box>

                  {/* Manual entry confirmation chip (DB mode only) */}
                  {companySourceTab === 'db' && companySearch.trim() && interviewCompany?.id === 0 && (
                    <Box sx={{ mb: 2, p: 1.5, borderRadius: 2, bgcolor: `${PRIMARY}08`, border: `1px solid ${PRIMARY}30`, display: 'flex', alignItems: 'center', gap: 1 }}>
                      <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 18 }} />
                      <Typography sx={{ fontSize: 13, color: PRIMARY, fontWeight: 600 }}>
                        「{interviewCompany.name}」で面接練習します
                      </Typography>
                    </Box>
                  )}

                  {/* DB mode: Company chips from DB */}
                  {companySourceTab === 'db' && (
                    <>
                      <Typography sx={{ fontSize: 12, color: '#94a3b8', mb: 1 }}>登録企業から選択</Typography>
                      {companiesLoading ? (
                        <LinearProgress sx={{ borderRadius: 1 }} />
                      ) : (
                        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5 }}>
                          {filteredCompanies.map(c => {
                            const isSelected = interviewCompany?.id === c.id
                            return (
                              <Button
                                key={c.id}
                                size="small"
                                onClick={() => { setInterviewCompany(c); setCompanySearch(c.name) }}
                                startIcon={isSelected ? <CheckCircleIcon sx={{ fontSize: '16px !important' }} /> : undefined}
                                sx={{
                                  px: 2, py: 0.8, borderRadius: 2, fontWeight: 500, fontSize: 13,
                                  textTransform: 'none',
                                  bgcolor: isSelected ? PRIMARY : '#f1f5f9',
                                  color: isSelected ? '#fff' : '#475569',
                                  '&:hover': { bgcolor: isSelected ? `${PRIMARY}e0` : '#e2e8f0' },
                                }}
                              >
                                {c.name}
                              </Button>
                            )
                          })}
                          {filteredCompanies.length === 0 && !companiesLoading && !companySearch.trim() && (
                            <Typography sx={{ color: '#94a3b8', fontSize: 13 }}>登録企業が見つかりません</Typography>
                          )}
                        </Box>
                      )}
                    </>
                  )}

                  {/* WEB mode: web search results */}
                  {companySourceTab === 'web' && (
                    <>
                      <Typography sx={{ fontSize: 12, color: '#94a3b8', mb: 1 }}>
                        {companySearch.trim() ? 'WEB検索結果' : 'キーワードを入力してください'}
                      </Typography>
                      {webSearchLoading ? (
                        <LinearProgress sx={{ borderRadius: 1 }} />
                      ) : (
                        <Stack spacing={1}>
                          {webSearchResults.map((result, i) => {
                            const isSelected = interviewCompany?.name === result.name
                            return (
                              <Box
                                key={i}
                                onClick={() => setInterviewCompany({ id: 0, name: result.name, description: result.description })}
                                sx={{
                                  p: 1.5, borderRadius: 2, cursor: 'pointer',
                                  border: `2px solid ${isSelected ? PRIMARY : '#e2e8f0'}`,
                                  bgcolor: isSelected ? `${PRIMARY}08` : '#f8fafc',
                                  transition: 'all 0.15s',
                                  '&:hover': { borderColor: PRIMARY, bgcolor: `${PRIMARY}05` },
                                }}
                              >
                                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                                  {isSelected && <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 16, flexShrink: 0 }} />}
                                  <Typography sx={{ fontWeight: 600, fontSize: 14, color: '#0f172a' }}>{result.name}</Typography>
                                </Box>
                                {result.description && (
                                  <Typography sx={{ fontSize: 12, color: '#64748b', mt: 0.5, lineHeight: 1.5 }}>
                                    {result.description.slice(0, 100)}{result.description.length > 100 ? '...' : ''}
                                  </Typography>
                                )}
                              </Box>
                            )
                          })}
                          {webSearchResults.length === 0 && companySearch.trim() && !webSearchLoading && (
                            <Typography sx={{ color: '#94a3b8', fontSize: 13 }}>検索結果が見つかりません</Typography>
                          )}
                        </Stack>
                      )}
                    </>
                  )}
                </Paper>

                {/* Position section */}
                <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0' }}>
                  <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2 }}>応募職種</Typography>

                  {/* Category tab */}
                  <Box sx={{ display: 'flex', gap: 1, mb: 2 }}>
                    <Button
                      size="small"
                      onClick={() => {
                        setPositionCategory('general')
                        if (selectedPosition.category === 'sier') setSelectedPosition(POSITIONS[0])
                      }}
                      sx={{
                        px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                        bgcolor: positionCategory === 'general' ? PRIMARY : '#f1f5f9',
                        color: positionCategory === 'general' ? '#fff' : '#475569',
                        '&:hover': { bgcolor: positionCategory === 'general' ? `${PRIMARY}e0` : '#e2e8f0' },
                      }}
                    >
                      💼 一般・IT職種
                    </Button>
                    <Button
                      size="small"
                      onClick={() => {
                        setPositionCategory('sier')
                        if (selectedPosition.category === 'general') setSelectedPosition(POSITIONS.find(p => p.category === 'sier')!)
                      }}
                      sx={{
                        px: 2, py: 0.6, borderRadius: 2, textTransform: 'none', fontWeight: 600, fontSize: 13,
                        bgcolor: positionCategory === 'sier' ? PRIMARY : '#f1f5f9',
                        color: positionCategory === 'sier' ? '#fff' : '#475569',
                        '&:hover': { bgcolor: positionCategory === 'sier' ? `${PRIMARY}e0` : '#e2e8f0' },
                      }}
                    >
                      🏗️ SIer職種
                    </Button>
                  </Box>

                  <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' }, gap: 1.5 }}>
                    {POSITIONS.filter(p => p.category === positionCategory).map(pos => {
                      const isSelected = selectedPosition.id === pos.id
                      return (
                        <Box
                          key={pos.id}
                          onClick={() => setSelectedPosition(pos)}
                          sx={{
                            position: 'relative', display: 'flex', alignItems: 'center', gap: 1.5,
                            p: 2, borderRadius: 2, cursor: 'pointer',
                            border: `2px solid ${isSelected ? PRIMARY : 'transparent'}`,
                            bgcolor: isSelected ? `${PRIMARY}08` : '#f8fafc',
                            transition: 'all 0.15s',
                            '&:hover': { borderColor: isSelected ? PRIMARY : '#cbd5e1' },
                          }}
                        >
                          <Typography sx={{ fontSize: 22 }}>{pos.icon}</Typography>
                          <Box sx={{ flex: 1 }}>
                            <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#0f172a' }}>{pos.title}</Typography>
                            <Typography sx={{ fontSize: 12, color: '#94a3b8' }}>{pos.department}</Typography>
                          </Box>
                          <Box sx={{ position: 'absolute', right: 12, top: '50%', transform: 'translateY(-50%)' }}>
                            {isSelected
                              ? <CheckCircleIcon sx={{ color: PRIMARY, fontSize: 20 }} />
                              : <Box sx={{ width: 18, height: 18, borderRadius: '50%', border: '2px solid #cbd5e1' }} />
                            }
                          </Box>
                        </Box>
                      )
                    })}
                  </Box>
                </Paper>
              </Box>

              {/* Right: Summary + CTA */}
              <Box sx={{ position: { md: 'sticky' }, top: 32, display: 'flex', flexDirection: 'column', gap: 3 }}>
                <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${PRIMARY}30`, bgcolor: `${PRIMARY}05` }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 3 }}>
                    <Box sx={{ width: 48, height: 48, borderRadius: 2, bgcolor: '#fff', border: '1px solid #e2e8f0', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                      <ApartmentIcon sx={{ color: PRIMARY }} />
                    </Box>
                    <Box>
                      <Typography sx={{ fontWeight: 700, fontSize: 15 }}>{interviewCompany?.name || '企業未選択'}</Typography>
                      <Typography sx={{ fontSize: 13, color: '#64748b' }}>{interviewCompany?.industry || '業種未設定'}</Typography>
                    </Box>
                  </Box>

                  <Stack spacing={2.5}>
                    <Box>
                      <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: 1, mb: 0.5 }}>応募ポジション</Typography>
                      <Typography sx={{ fontWeight: 600, fontSize: 15 }}>{selectedPosition.title}</Typography>
                    </Box>
                    <Box>
                      <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#94a3b8', textTransform: 'uppercase', letterSpacing: 1, mb: 0.5 }}>企業概要</Typography>
                      <Typography sx={{ fontSize: 13, color: '#64748b', lineHeight: 1.7 }}>
                        {interviewCompany?.description
                          ? interviewCompany.description.slice(0, 120) + (interviewCompany.description.length > 120 ? '...' : '')
                          : '企業を選択すると詳細が表示されます。'}
                      </Typography>
                    </Box>
                    <Box sx={{ pt: 2, borderTop: `1px solid ${PRIMARY}15` }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                        <Typography sx={{ fontSize: 14 }}>⏱</Typography>
                        <Typography sx={{ fontSize: 13, color: '#475569' }}>所要時間: {interviewLimits.maxMinutes}分</Typography>
                      </Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Typography sx={{ fontSize: 14 }}>❓</Typography>
                        <Typography sx={{ fontSize: 13, color: '#475569' }}>{selectedPosition.questions}問 技術・行動面接</Typography>
                      </Box>
                    </Box>
                  </Stack>
                </Paper>

                {/* 企業別面接傾向ヒント */}
                {interviewCompany && (
                  <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: '1px solid #fcd34d', bgcolor: '#fffbeb' }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1.5 }}>
                      <LightbulbIcon sx={{ fontSize: 18, color: '#d97706' }} />
                      <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#92400e' }}>
                        {interviewCompany.name} の面接傾向
                      </Typography>
                    </Box>
                    {hintsLoading ? (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <CircularProgress size={14} sx={{ color: '#d97706' }} />
                        <Typography sx={{ fontSize: 12, color: '#92400e' }}>傾向を調査中...</Typography>
                      </Box>
                    ) : companyHints && (companyHints.style_tags.length > 0 || companyHints.top_questions.length > 0) ? (
                      <Stack spacing={1.5}>
                        {companyHints.style_tags.length > 0 && (
                          <Box>
                            <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#b45309', mb: 0.5 }}>面接スタイル</Typography>
                            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 0.5 }}>
                              {companyHints.style_tags.map((tag, i) => (
                                <Chip key={i} label={tag} size="small" sx={{ bgcolor: '#fef3c7', color: '#92400e', fontWeight: 600, fontSize: 11, border: '1px solid #fcd34d' }} />
                              ))}
                            </Box>
                          </Box>
                        )}
                        {companyHints.top_questions.length > 0 && (
                          <Box>
                            <Typography sx={{ fontSize: 11, fontWeight: 700, color: '#b45309', mb: 0.5 }}>よく聞かれる質問</Typography>
                            <Stack spacing={0.5}>
                              {companyHints.top_questions.map((q, i) => (
                                <Box key={i} sx={{ display: 'flex', gap: 1, alignItems: 'flex-start' }}>
                                  <Typography sx={{ fontSize: 11, fontWeight: 700, color: PRIMARY, minWidth: 16 }}>{i + 1}.</Typography>
                                  <Typography sx={{ fontSize: 12, color: '#78350f', lineHeight: 1.5 }}>{q}</Typography>
                                </Box>
                              ))}
                            </Stack>
                          </Box>
                        )}
                      </Stack>
                    ) : (
                      <Typography sx={{ fontSize: 12, color: '#92400e' }}>企業を選択すると面接傾向を表示します。</Typography>
                    )}
                  </Paper>
                )}

                <Button
                  variant="contained"
                  fullWidth
                  endIcon={<ArrowForwardIcon />}
                  disabled={!interviewCompany}
                  onClick={() => setStatus('lobby')}
                  sx={{
                    bgcolor: PRIMARY, '&:hover': { bgcolor: `${PRIMARY}e0` },
                    borderRadius: 2, py: 1.8, fontWeight: 700, fontSize: 16,
                    textTransform: 'none',
                    boxShadow: `0 8px 24px ${PRIMARY}30`,
                    '&:disabled': { bgcolor: '#e2e8f0', color: '#94a3b8', boxShadow: 'none' },
                  }}
                >
                  面接を開始する
                </Button>
                <Typography sx={{ fontSize: 12, textAlign: 'center', color: '#94a3b8' }}>
                  開始すると<Box component="a" href="#" sx={{ textDecoration: 'underline', color: 'inherit' }}>利用規約</Box>に同意したことになります
                </Typography>
              </Box>
            </Box>
          </Box>
        </Box>
      </Box>
    )
  }

  // ─────────────────────────────────────────────
  // 録画同意ダイアログ（LOBBY / SESSION 画面で共有）
  // ─────────────────────────────────────────────
  const consentDialog = (
    <Dialog open={consentDialogOpen} onClose={() => setConsentDialogOpen(false)} maxWidth="sm" fullWidth>
      <DialogTitle>面接練習を開始する前に</DialogTitle>
      <DialogContent>
        <Typography variant="body2" sx={{ mb: 2 }}>
          面接練習では、より良いフィードバックのために以下のデータを収集します。
        </Typography>
        <Typography variant="body2" component="ul" sx={{ pl: 2, mb: 2, color: 'text.secondary' }}>
          <li>音声・動画の録画（フィードバック生成のみに使用）</li>
          <li>会話のテキスト（評価スコア算出に使用）</li>
        </Typography>
        <Typography variant="body2" color="error.main" sx={{ mb: 2 }}>
          ※ これらのデータは企業には提供されません。サービス内でのみ使用します。
        </Typography>
        <Typography variant="body2" sx={{ mb: 2 }}>
          録画データは面接セッション終了後90日で自動削除されます。
          詳細は<Button size="small" sx={{ p: 0, minWidth: 0, textDecoration: 'underline', fontSize: 'inherit' }}
            onClick={() => window.open('/privacy', '_blank')}>プライバシーポリシー</Button>をご確認ください。
        </Typography>
        <FormControlLabel
          control={
            <Checkbox
              checked={consentGiven}
              onChange={(e) => setConsentGiven(e.target.checked)}
            />
          }
          label="上記の内容に同意して面接練習を始める"
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={() => setConsentDialogOpen(false)}>キャンセル</Button>
        <Button
          variant="contained"
          disabled={!consentGiven}
          onClick={() => { setConsentDialogOpen(false); handleJoin() }}
        >
          面接に参加
        </Button>
      </DialogActions>
    </Dialog>
  )

  // ─────────────────────────────────────────────
  // LOBBY SCREEN
  // ─────────────────────────────────────────────
  if (status === 'lobby') {
    return (
      <Box sx={{ minHeight: '100vh', bgcolor: BG_LIGHT, display: 'flex', flexDirection: 'column' }}>
        {/* Header */}
        <Box component="header" sx={{ px: 3, py: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box sx={{ width: 40, height: 40, borderRadius: 2, bgcolor: PRIMARY, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <PsychologyIcon sx={{ color: '#fff', fontSize: 24 }} />
            </Box>
            <Box>
              <Typography sx={{ fontWeight: 700, fontSize: 16, lineHeight: 1.2 }}>AI面接練習</Typography>
              <Typography sx={{ fontSize: 12, color: 'text.secondary' }}>セッション: {companyName}</Typography>
            </Box>
          </Box>
          <IconButton size="small" onClick={() => router.push('/')}>
            <ArrowBackIcon fontSize="small" />
          </IconButton>
        </Box>

        {/* Content */}
        <Box sx={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', p: { xs: 2, md: 6 } }}>
          <Box sx={{ maxWidth: 960, width: '100%', display: 'grid', gridTemplateColumns: { xs: '1fr', lg: '7fr 5fr' }, gap: { xs: 4, lg: 8 }, alignItems: 'center' }}>

            {/* Camera preview */}
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 3 }}>
              <Box sx={{ position: 'relative', width: '100%', aspectRatio: '16/9', bgcolor: '#202124', borderRadius: 2, overflow: 'hidden', boxShadow: '0 4px 20px rgba(0,0,0,0.25)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                {lobbyPermissionError ? (
                  <Box sx={{ textAlign: 'center', p: 3 }}>
                    <Typography sx={{ color: '#f28b82', mb: 2, fontSize: 14 }}>{lobbyPermissionError}</Typography>
                    <Button size="small" startIcon={<RefreshIcon />} onClick={() => { setLobbyPermissionError(null); window.location.reload() }} sx={{ color: '#8ab4f8' }}>
                      再試行
                    </Button>
                  </Box>
                ) : (
                  <video
                    ref={lobbyVideoRef}
                    muted
                    playsInline
                    style={{ width: '100%', height: '100%', objectFit: 'cover', transform: 'scaleX(-1)', display: cameraEnabled ? 'block' : 'none' }}
                  />
                )}
                {!lobbyPermissionError && !cameraEnabled && (
                  <Box sx={{ position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                    <VideocamOffIcon sx={{ color: '#9aa0a6', fontSize: 48 }} />
                  </Box>
                )}

                {/* Camera label */}
                <Box sx={{ position: 'absolute', top: 12, left: 12, bgcolor: 'rgba(0,0,0,0.5)', px: 1.5, py: 0.5, borderRadius: 1 }}>
                  <Typography sx={{ color: '#fff', fontSize: 13 }}>{user.name || 'あなた'}</Typography>
                </Box>

                {/* Controls overlay */}
                <Box sx={{ position: 'absolute', bottom: 16, left: 0, right: 0, display: 'flex', justifyContent: 'center', gap: 2 }}>
                  <Tooltip title={micEnabled ? 'マイクをオフ' : 'マイクをオン'}>
                    <IconButton
                      onClick={() => {
                        if (!streamRef.current) return
                        const next = !micEnabled
                        streamRef.current.getAudioTracks().forEach(t => { t.enabled = next })
                        setMicEnabled(next)
                      }}
                      sx={{ bgcolor: micEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', border: '1px solid rgba(255,255,255,0.3)', '&:hover': { bgcolor: micEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' } }}
                    >
                      {micEnabled ? <MicIcon sx={{ color: '#fff' }} /> : <MicOffIcon sx={{ color: '#fff' }} />}
                    </IconButton>
                  </Tooltip>
                  <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
                    <IconButton
                      onClick={() => {
                        if (!streamRef.current) return
                        const next = !cameraEnabled
                        streamRef.current.getVideoTracks().forEach(t => { t.enabled = next })
                        setCameraEnabled(next)
                      }}
                      sx={{ bgcolor: cameraEnabled ? 'rgba(255,255,255,0.15)' : '#ea4335', border: '1px solid rgba(255,255,255,0.3)', '&:hover': { bgcolor: cameraEnabled ? 'rgba(255,255,255,0.25)' : '#c5221f' } }}
                    >
                      {cameraEnabled ? <VideocamIcon sx={{ color: '#fff' }} /> : <VideocamOffIcon sx={{ color: '#fff' }} />}
                    </IconButton>
                  </Tooltip>
                </Box>
              </Box>

              <Typography variant="body2" sx={{ color: 'text.secondary', fontSize: 13 }}>
                カメラとマイクの確認が完了したら「面接に参加」を押してください
              </Typography>
            </Box>

            {/* Join panel */}
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: { xs: 'center', lg: 'flex-start' }, textAlign: { xs: 'center', lg: 'left' } }}>
              {searchParams.get('company_name') && (
                <Box sx={{ mb: 2, px: 2, py: 1, bgcolor: '#e8f5e9', borderRadius: 2, border: '1px solid #a5d6a7', width: '100%', maxWidth: 340 }}>
                  <Typography sx={{ fontSize: 13, color: '#2e7d32', fontWeight: 600 }}>
                    {interviewCompany?.name}（{interviewCompany?.industry || '業種未設定'}）向けの面接練習を始めます
                  </Typography>
                </Box>
              )}
              <Typography variant="h4" sx={{ fontWeight: 400, color: '#202124', mb: 1 }}>
                準備はできましたか？
              </Typography>
              <Typography sx={{ color: 'text.secondary', mb: 1, fontSize: 15 }}>
                {companyName}
              </Typography>
              <Box sx={{ mb: 3, px: 2, py: 1, bgcolor: '#e3f2fd', borderRadius: 1, border: '1px solid #90caf9' }}>
                <Typography sx={{ fontSize: 13, color: '#1565c0' }}>
                  ⏱ このセッションは最大{interviewLimits.maxMinutes}分です
                </Typography>
              </Box>

              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ width: '100%', maxWidth: 340 }}>
                <Button
                  variant="contained"
                  onClick={handleJoinWithConsent}
                  sx={{
                    flex: 1,
                    bgcolor: '#1a73e8',
                    '&:hover': { bgcolor: '#1557b0' },
                    borderRadius: 9999,
                    py: 1.2,
                    fontWeight: 500,
                    fontSize: 15,
                    textTransform: 'none',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.2)',
                  }}
                >
                  面接に参加
                </Button>
              </Stack>

              <Box sx={{ mt: 5 }}>
                <Typography variant="body2" sx={{ color: 'text.secondary', mb: 1.5 }}>募集要項を確認する</Typography>
                <Button
                  size="small"
                  onClick={() => {/* scroll to info */ }}
                  sx={{ color: '#1a73e8', textTransform: 'none', fontWeight: 500, p: 0, '&:hover': { textDecoration: 'underline', bgcolor: 'transparent' } }}
                >
                  求人詳細を見る →
                </Button>
              </Box>
            </Box>
          </Box>
        </Box>

        {/* Footer */}
        <Box component="footer" sx={{ py: 2.5, display: 'flex', justifyContent: 'center', gap: 4 }}>
          {['プライバシー', '利用規約', 'ヘルプ'].map(label => (
            <Typography key={label} variant="body2" sx={{ color: 'text.secondary', cursor: 'pointer', '&:hover': { textDecoration: 'underline' } }}>
              {label}
            </Typography>
          ))}
        </Box>

        {consentDialog}
      </Box>
    )
  }

  // ─────────────────────────────────────────────
  // REPORT SCREEN (finished)
  // ─────────────────────────────────────────────
  if (status === 'finished') {
    return (
      <Box sx={{ minHeight: '100vh', bgcolor: BG_DARK, color: '#e8eaed', overflowY: 'auto', p: { xs: 2, md: 4 } }}>
        <Box sx={{ maxWidth: 720, mx: 'auto' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 4 }}>
            <IconButton sx={{ color: '#bdc1c6' }} onClick={() => router.push('/')}>
              <ArrowBackIcon />
            </IconButton>
            <Typography variant="h5" sx={{ fontWeight: 700, color: '#e8eaed' }}>面接レポート</Typography>
          </Box>

          {errorMessage && (
            <Paper sx={{ bgcolor: 'rgba(234,67,53,0.15)', border: '1px solid rgba(234,67,53,0.4)', p: 2, mb: 2, borderRadius: 2 }}>
              <Typography variant="body2" sx={{ color: '#f28b82' }}>{errorMessage}</Typography>
            </Paper>
          )}

          {reportStatus === 'pending' && (
            <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(255,255,255,0.08)', p: 3, borderRadius: 2 }}>
              <Typography sx={{ color: '#e8eaed', mb: 1.5 }}>レポートを生成中です...</Typography>
              <LinearProgress sx={{ bgcolor: '#3c4043', '& .MuiLinearProgress-bar': { bgcolor: '#4285f4' } }} />
            </Paper>
          )}

          {reportStatus === 'ready' && report && (
            <Stack spacing={2}>
              <InterviewSummary report={report} userId={user?.user_id} theme="dark" />
              <Button
                variant="outlined"
                fullWidth
                disabled={emailSending || emailSent || !user || user.is_guest}
                onClick={async () => {
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
                sx={{ color: emailSent ? '#34a853' : PRIMARY, borderColor: emailSent ? '#34a853' : PRIMARY, '&:hover': { borderColor: PRIMARY, bgcolor: 'rgba(236,91,19,0.08)' } }}
              >
                {emailSent ? '✓ メールを送信しました' : emailSending ? '送信中...' : 'レポートをメールで受け取る'}
              </Button>
            </Stack>
          )}

          {reportStatus === 'error' && (
            <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(234,67,53,0.3)', p: 3, borderRadius: 2 }}>
              <Typography variant="body2" sx={{ color: '#f28b82' }}>レポート生成に失敗しました。</Typography>
            </Paper>
          )}

          {/* Video upload status */}
          {videoUploadStatus !== 'idle' && (
            <Paper sx={{ bgcolor: '#2d2e31', border: '1px solid rgba(255,255,255,0.1)', p: 2, borderRadius: 2 }}>
              {videoSizeWarning && (
                <Typography variant="caption" sx={{ color: '#fdd663', display: 'block', mb: 1 }}>
                  ⚠ {videoSizeWarning}
                </Typography>
              )}
              <Typography variant="body2" sx={{ color: videoUploadStatus === 'done' ? '#34a853' : videoUploadStatus === 'error' ? '#f28b82' : '#9aa0a6', mb: videoUploadStatus === 'uploading' ? 1 : 0 }}>
                {videoUploadStatus === 'uploading' && `動画をアップロード中... ${videoUploadProgress}%`}
                {videoUploadStatus === 'done' && '✓ 動画のアップロードが完了しました'}
                {videoUploadStatus === 'error' && '動画のアップロードに失敗しました。ネットワーク接続を確認してください'}
              </Typography>
              {videoUploadStatus === 'uploading' && (
                <LinearProgress
                  variant="determinate"
                  value={videoUploadProgress}
                  sx={{ height: 6, borderRadius: 3, bgcolor: 'rgba(255,255,255,0.1)', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }}
                />
              )}
            </Paper>
          )}
        </Box>
      </Box>
    )
  }

  // ─────────────────────────────────────────────
  // SESSION SCREEN (connecting / connected / error)
  // ─────────────────────────────────────────────
  return (
    <Box sx={{ height: '100vh', width: '100vw', bgcolor: BG_DARK, display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>

      {/* ── Header ── */}
      <Box component="header" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 3, py: 1.5, flexShrink: 0, borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ width: 36, height: 36, borderRadius: 2, bgcolor: PRIMARY, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <PsychologyIcon sx={{ color: '#fff', fontSize: 20 }} />
          </Box>
          <Box>
            <Typography sx={{ fontWeight: 700, fontSize: 15, color: '#e8eaed', lineHeight: 1.2 }}>AI面接練習</Typography>
            <Typography sx={{ fontSize: 12, color: '#9aa0a6' }}>セッション: {companyName}</Typography>
          </Box>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
          {isActive && (
            <Box sx={{ display: 'flex', alignItems: 'center', px: 1.5, py: 0.6, borderRadius: 9999, bgcolor: 'rgba(255,255,255,0.08)', gap: 1 }}>
              <Box component="span" sx={{ fontSize: 16 }}>⏱</Box>
              <Typography sx={{ fontSize: 13, color: '#e8eaed', fontWeight: 500 }}>{formatSeconds(remainingSeconds)}</Typography>
            </Box>
          )}
          {isActive && (
            <Box sx={{ display: 'flex', alignItems: 'center', px: 1.5, py: 0.6, borderRadius: 9999, bgcolor: 'rgba(236,91,19,0.2)', border: `1px solid ${PRIMARY}66` }}>
              <Typography sx={{ fontSize: 12, color: '#ffd8c2', fontWeight: 700 }}>
                質問 {currentQuestionIndex}/{totalQuestionCount}
              </Typography>
            </Box>
          )}
          {isConnected && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: '#34a853', animation: 'pulse 2s infinite', '@keyframes pulse': { '0%,100%': { opacity: 1 }, '50%': { opacity: 0.4 } } }} />
              <Typography sx={{ fontSize: 12, color: '#34a853', fontWeight: 600 }}>接続中</Typography>
            </Box>
          )}
        </Box>
      </Box>

      {/* ── Time warning ── */}
      {isConnected && sessionWarningShown && (
        <Box sx={{ px: 2, pt: 1, flexShrink: 0 }}>
          <Box sx={{ bgcolor: remainingSeconds <= 0 ? 'rgba(234,67,53,0.2)' : 'rgba(251,188,4,0.15)', border: `1px solid ${remainingSeconds <= 0 ? '#ea4335' : '#fbbc04'}`, borderRadius: 1, px: 2, py: 0.8, display: 'flex', alignItems: 'center', gap: 1 }}>
            <Typography sx={{ fontSize: 13, color: remainingSeconds <= 0 ? '#f28b82' : '#fdd663', fontWeight: 600 }}>
              {remainingSeconds <= 0
                ? 'セッションが終了しました。'
                : `⚠️ 残り約${Math.ceil(remainingSeconds / 60)}分です。セッションがまもなく終了します。`}
            </Typography>
          </Box>
        </Box>
      )}

      {/* ── Question progress ── */}
      {isConnected && (
        <Box sx={{ px: 2, pt: 1, flexShrink: 0 }}>
          <Paper sx={{ bgcolor: 'rgba(255,255,255,0.06)', border: '1px solid rgba(255,255,255,0.12)', borderRadius: 2, p: 1.5 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
              <Typography sx={{ fontSize: 13, color: '#e8eaed', fontWeight: 700 }}>
                質問 {currentQuestionIndex}/{totalQuestionCount}
              </Typography>
              <Typography sx={{ fontSize: 12, color: '#cdd1d5' }}>
                全体経過 {formatSeconds(elapsedSeconds)} • 質問内 {formatSeconds(questionElapsedSeconds)} / 残り目安 {formatSeconds(questionRemainingSeconds)}
              </Typography>
            </Box>
            <LinearProgress
              variant="determinate"
              value={questionProgress}
              sx={{ height: 7, borderRadius: 9999, bgcolor: 'rgba(255,255,255,0.12)', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }}
            />
          </Paper>
        </Box>
      )}

      {/* ── Main ── */}
      <Box sx={{ flex: 1, display: 'flex', flexDirection: { xs: 'column', lg: 'row' }, gap: 2, px: 2, pb: '88px', pt: 2, overflow: 'hidden', minHeight: 0 }}>

        {/* Video grid */}
        <Box sx={{ flex: 1, display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, gap: 2, alignContent: 'center', minHeight: 0 }}>

          {/* AI interviewer tile */}
          <Box sx={{ position: 'relative', aspectRatio: '16/9', borderRadius: 2, overflow: 'hidden', bgcolor: '#303134', boxShadow: '0 8px 32px rgba(0,0,0,0.4)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <Box sx={{
              width: '100%',
              height: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'radial-gradient(circle at 50% 40%, #3c4043 0%, #202124 100%)',
            }}>
              <Box sx={{
                width: { xs: 120, md: 180 },
                height: { xs: 120, md: 180 },
                borderRadius: '50%',
                boxShadow: aiSpeaking ? `0 0 0 16px rgba(236,91,19,0.15), 0 0 40px rgba(236,91,19,0.1)` : 'none',
                transition: 'box-shadow 0.3s',
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
                <ThreeAvatar gender={avatarGender} audioStream={null} level={aiLevel} speaking={aiSpeaking} />
              </Box>
            </Box>

            {/* Speaking indicator */}
            {aiSpeaking && (
              <Box sx={{ position: 'absolute', top: 12, right: 12, width: 10, height: 10, borderRadius: '50%', bgcolor: PRIMARY, animation: 'pulse 1s infinite' }} />
            )}

            {/* Label */}
            <Box sx={{ position: 'absolute', bottom: 12, left: 12, display: 'flex', alignItems: 'center', gap: 1, bgcolor: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(8px)', px: 1.5, py: 0.6, borderRadius: 1.5 }}>
              <Box sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: PRIMARY }} />
              <Typography sx={{ color: '#fff', fontSize: 13, fontWeight: 500 }}>
                面接官AI（{isFemale ? '女性' : '男性'}）
              </Typography>
            </Box>


            {/* Error overlay */}
            {status === 'error' && (
              <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.8)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', p: 3, gap: 2 }}>
                <Typography sx={{ color: '#f28b82', textAlign: 'center', fontSize: 14, lineHeight: 1.6 }}>{errorMessage}</Typography>
                <Button variant="contained" startIcon={<RefreshIcon />} onClick={handleJoin}
                  sx={{ bgcolor: '#4285f4', '&:hover': { bgcolor: '#3367d6' }, textTransform: 'none' }}>
                  再接続する
                </Button>
              </Box>
            )}

            {/* Connecting overlay */}
            {status === 'connecting' && (
              <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.6)', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2 }}>
                <Typography sx={{ color: '#e8eaed' }}>接続中...</Typography>
                <LinearProgress sx={{ width: 160, bgcolor: '#3c4043', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }} />
              </Box>
            )}
          </Box>

          {/* Subtitle below avatar frame */}
          {captionsVisible && latestAiText && (
            <Box sx={{ mx: 1, mt: -0.5, mb: 0.5, bgcolor: 'rgba(0,0,0,0.55)', backdropFilter: 'blur(8px)', borderRadius: 1.5, px: 2, py: 0.8, textAlign: 'center' }}>
              <Typography sx={{ color: '#fff', fontSize: 13, lineHeight: 1.5 }}>{latestAiText}</Typography>
            </Box>
          )}

          {/* User camera tile */}
          <Box sx={{ position: 'relative', aspectRatio: '16/9', borderRadius: 2, overflow: 'hidden', bgcolor: '#3c4043', boxShadow: `0 0 0 2px rgba(236,91,19,0.3), 0 8px 32px rgba(0,0,0,0.4)`, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
            <video
              ref={sessionVideoRef}
              muted
              playsInline
              style={{ width: '100%', height: '100%', objectFit: 'cover', transform: 'scaleX(-1)', display: cameraEnabled ? 'block' : 'none' }}
            />
            {!cameraEnabled && (
              <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
                <VideocamOffIcon sx={{ color: '#9aa0a6', fontSize: 40 }} />
                <Typography sx={{ color: '#9aa0a6', fontSize: 13 }}>カメラオフ</Typography>
              </Box>
            )}
            <Box sx={{ position: 'absolute', bottom: 12, left: 12, display: 'flex', alignItems: 'center', gap: 1, bgcolor: 'rgba(0,0,0,0.5)', backdropFilter: 'blur(8px)', px: 1.5, py: 0.6, borderRadius: 1.5 }}>
              {micEnabled ? <MicIcon sx={{ color: '#fff', fontSize: 16 }} /> : <MicOffIcon sx={{ color: '#ea4335', fontSize: 16 }} />}
              <Typography sx={{ color: '#fff', fontSize: 13, fontWeight: 500 }}>あなた（候補者）</Typography>
            </Box>
            {/* Highlight border overlay */}
            <Box sx={{ position: 'absolute', inset: 0, border: `2px solid rgba(236,91,19,0.25)`, borderRadius: 2, pointerEvents: 'none' }} />
          </Box>
        </Box>

        {/* ── Right sidebar: Transcript ── */}
        <Box sx={{ width: { xs: '100%', lg: 360 }, display: 'flex', flexDirection: 'column', bgcolor: 'rgba(255,255,255,0.04)', border: '1px solid rgba(255,255,255,0.08)', borderRadius: 2, overflow: 'hidden', flexShrink: 0 }}>
          {/* Sidebar header */}
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: 2, py: 1.5, borderBottom: '1px solid rgba(255,255,255,0.08)' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
              <Box component="span" sx={{ fontSize: 20 }}>💬</Box>
              <Typography sx={{ fontWeight: 700, color: '#e8eaed', fontSize: 14 }}>リアルタイム字幕</Typography>
            </Box>
            {isConnected && (
              <Box sx={{ bgcolor: `${PRIMARY}20`, border: `1px solid ${PRIMARY}40`, px: 1, py: 0.3, borderRadius: 1 }}>
                <Typography sx={{ color: PRIMARY, fontSize: 10, fontWeight: 700, letterSpacing: 1 }}>LIVE</Typography>
              </Box>
            )}
          </Box>

          {/* Transcript list */}
          <Box sx={{ flex: 1, overflowY: 'auto', p: 2, display: 'flex', flexDirection: 'column', gap: 2.5 }}>
            {utterances.length === 0 && !partialAi && (
              <Typography sx={{ color: '#5f6368', fontSize: 13, textAlign: 'center', mt: 4 }}>
                面接が始まると字幕がここに表示されます
              </Typography>
            )}
            {utterances.map((u, i) => (
              <Box key={i} sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, alignItems: u.role === 'ai' ? 'flex-start' : 'flex-end' }}>
                <Typography sx={{ fontSize: 10, fontWeight: 700, color: u.role === 'ai' ? '#9aa0a6' : PRIMARY, letterSpacing: 1, textTransform: 'uppercase' }}>
                  {u.role === 'ai' ? `面接官AI（${isFemale ? '女性' : '男性'}）` : 'あなた'}
                </Typography>
                <Box sx={{
                  bgcolor: u.role === 'ai' ? 'rgba(255,255,255,0.06)' : PRIMARY,
                  px: 1.5, py: 1, borderRadius: u.role === 'ai' ? '0 12px 12px 12px' : '12px 0 12px 12px',
                  maxWidth: '90%',
                }}>
                  <Typography sx={{ fontSize: 13, color: '#e8eaed', lineHeight: 1.6 }}>{u.text}</Typography>
                </Box>
              </Box>
            ))}

            {/* Partial AI (typing) */}
            {partialAi && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5 }}>
                <Typography sx={{ fontSize: 10, fontWeight: 700, color: '#9aa0a6', letterSpacing: 1, textTransform: 'uppercase' }}>
                  面接官AI
                </Typography>
                <Box sx={{ bgcolor: 'rgba(255,255,255,0.06)', px: 1.5, py: 1, borderRadius: '0 12px 12px 12px', maxWidth: '90%', opacity: 0.7 }}>
                  <Typography sx={{ fontSize: 13, color: '#e8eaed', lineHeight: 1.6 }}>{partialAi}</Typography>
                </Box>
              </Box>
            )}

            {/* Partial user (typing) */}
            {partialUser && (
              <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.5, alignItems: 'flex-end' }}>
                <Typography sx={{ fontSize: 10, fontWeight: 700, color: PRIMARY, letterSpacing: 1, textTransform: 'uppercase' }}>あなた</Typography>
                <Box sx={{ bgcolor: `${PRIMARY}80`, px: 1.5, py: 1, borderRadius: '12px 0 12px 12px', maxWidth: '90%', opacity: 0.7 }}>
                  <Typography sx={{ fontSize: 13, color: '#fff', lineHeight: 1.6 }}>{partialUser}</Typography>
                </Box>
              </Box>
            )}

            {/* AI tip */}
            {utterances.length >= 2 && (
              <Box sx={{ p: 1.5, bgcolor: `${PRIMARY}10`, border: `1px solid ${PRIMARY}30`, borderRadius: 2, display: 'flex', gap: 1.5 }}>
                <LightbulbIcon sx={{ color: PRIMARY, fontSize: 20, flexShrink: 0 }} />
                <Box>
                  <Typography sx={{ fontSize: 11, fontWeight: 700, color: PRIMARY, mb: 0.3 }}>AIヒント</Typography>
                  <Typography sx={{ fontSize: 12, color: '#9aa0a6', lineHeight: 1.5 }}>
                    具体的なエピソードを交えて回答すると、より説得力が増します。
                  </Typography>
                </Box>
              </Box>
            )}

            <div ref={transcriptEndRef} />
          </Box>

          {/* Note input */}
          <Box sx={{ p: 1.5, borderTop: '1px solid rgba(255,255,255,0.08)' }}>
            <Box sx={{ display: 'flex', alignItems: 'center', bgcolor: 'rgba(255,255,255,0.06)', borderRadius: 2, px: 1.5, py: 0.5 }}>
              <InputBase
                value={noteInput}
                onChange={e => setNoteInput(e.target.value)}
                placeholder="メモやヒントのリクエストを入力..."
                sx={{ flex: 1, color: '#e8eaed', fontSize: 13, '& ::placeholder': { color: '#5f6368' } }}
              />
              <IconButton size="small" sx={{ color: noteInput ? PRIMARY : '#5f6368' }}>
                <SendIcon fontSize="small" />
              </IconButton>
            </Box>
          </Box>
        </Box>
      </Box>

      {/* ── Bottom control bar ── */}
      <Box sx={{
        position: 'fixed', bottom: 0, left: 0, right: 0,
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        px: { xs: 2, md: 4 }, py: 1.5,
        bgcolor: 'rgba(32,33,36,0.97)',
        borderTop: '1px solid rgba(255,255,255,0.06)',
        zIndex: 100,
      }}>
        {/* Left: session info */}
        <Box sx={{ display: { xs: 'none', md: 'flex' }, alignItems: 'center', gap: 1 }}>
          <Typography sx={{ fontSize: 13, color: '#9aa0a6', fontWeight: 500 }}>
            {companyName} 面接
          </Typography>
          {isActive && (
            <LinearProgress
              variant="determinate"
              value={progress}
              sx={{ width: 80, height: 4, borderRadius: 2, bgcolor: '#3c4043', '& .MuiLinearProgress-bar': { bgcolor: PRIMARY } }}
            />
          )}
        </Box>

        {/* Center: controls */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: { xs: 1, md: 1.5 }, mx: 'auto' }}>
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.5 }}>
            <Tooltip title={
              aiSpeaking ? 'AI発話中...' :
              turnPending ? 'AIが考えています...' :
              isRecording ? 'クリックして送信' :
              'クリックして話す'
            }>
              <span>
                <IconButton
                  onClick={isRecording ? stopRecording : startRecording}
                  disabled={!isConnected || turnPending || aiSpeaking}
                  sx={{
                    bgcolor: isRecording ? '#ea4335' : aiSpeaking ? `${PRIMARY}40` : 'rgba(255,255,255,0.08)',
                    width: 52, height: 52,
                    animation: isRecording ? 'micPulse 1s infinite' : 'none',
                    '@keyframes micPulse': { '0%,100%': { boxShadow: `0 0 0 0 rgba(234,67,53,0.4)` }, '50%': { boxShadow: `0 0 0 8px rgba(234,67,53,0)` } },
                    '&:hover': { bgcolor: isRecording ? '#c5221f' : 'rgba(255,255,255,0.15)' },
                    '&:disabled': { bgcolor: 'rgba(255,255,255,0.04)' },
                  }}
                >
                  {turnPending
                    ? <CircularProgress size={22} sx={{ color: '#9aa0a6' }} />
                    : aiSpeaking
                      ? <VolumeUpIcon sx={{ color: PRIMARY }} />
                      : isRecording
                        ? <MicIcon sx={{ color: '#fff' }} />
                        : <MicIcon sx={{ color: '#e8eaed' }} />
                  }
                </IconButton>
              </span>
            </Tooltip>
            <Typography sx={{ fontSize: 10, color: isRecording ? '#ea4335' : aiSpeaking ? PRIMARY : '#9aa0a6', fontWeight: 600, letterSpacing: 0.5, lineHeight: 1 }}>
              {isRecording ? '録音中' : turnPending ? '処理中' : aiSpeaking ? 'AI発話中' : '話す'}
            </Typography>
          </Box>

          <Tooltip title={cameraEnabled ? 'カメラをオフ' : 'カメラをオン'}>
            <span>
              <IconButton onClick={toggleCamera} disabled={!isConnected} sx={{ bgcolor: cameraEnabled ? 'rgba(255,255,255,0.08)' : '#ea4335', width: 48, height: 48, '&:hover': { bgcolor: cameraEnabled ? 'rgba(255,255,255,0.15)' : '#c5221f' }, '&:disabled': { bgcolor: 'rgba(255,255,255,0.04)' } }}>
                {cameraEnabled ? <VideocamIcon sx={{ color: '#e8eaed' }} /> : <VideocamOffIcon sx={{ color: '#fff' }} />}
              </IconButton>
            </span>
          </Tooltip>

          <Tooltip title={captionsVisible ? '字幕をオフ' : '字幕をオン'}>
            <IconButton onClick={() => setCaptionsVisible(p => !p)} sx={{ bgcolor: captionsVisible ? `${PRIMARY}30` : 'rgba(255,255,255,0.08)', width: 48, height: 48, '&:hover': { bgcolor: captionsVisible ? `${PRIMARY}40` : 'rgba(255,255,255,0.15)' } }}>
              <ClosedCaptionIcon sx={{ color: captionsVisible ? PRIMARY : '#9aa0a6' }} />
            </IconButton>
          </Tooltip>

          <Tooltip title={handsFreeMode ? 'ハンズフリーをオフ（ボタン操作に戻す）' : 'ハンズフリーをオン（声を検知して自動送信）'}>
            <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.5 }}>
              <IconButton
                onClick={() => setHandsFreeMode(p => !p)}
                disabled={!isConnected}
                sx={{
                  bgcolor: handsFreeMode ? `${PRIMARY}30` : 'rgba(255,255,255,0.08)',
                  width: 48, height: 48,
                  '&:hover': { bgcolor: handsFreeMode ? `${PRIMARY}40` : 'rgba(255,255,255,0.15)' },
                  '&:disabled': { bgcolor: 'rgba(255,255,255,0.04)' },
                }}
              >
                <MicIcon sx={{ color: handsFreeMode ? PRIMARY : '#9aa0a6', fontSize: 20 }} />
              </IconButton>
              <Typography sx={{ fontSize: 9, color: handsFreeMode ? PRIMARY : '#9aa0a6', fontWeight: 600, letterSpacing: 0.3 }}>
                {handsFreeMode ? 'AUTO' : 'ハンズフリー'}
              </Typography>
            </Box>
          </Tooltip>

          {/* End call / Join button */}
          {!isActive ? (
            <Button
              variant="contained"
              onClick={handleJoin}
              sx={{ bgcolor: '#34a853', '&:hover': { bgcolor: '#2d8f47' }, borderRadius: 9999, px: 3, py: 1, fontWeight: 600, textTransform: 'none', fontSize: 14 }}
            >
              面接を開始
            </Button>
          ) : (
            <Tooltip title="面接を終了">
              <Button
                variant="contained"
                startIcon={<CallEndIcon />}
                onClick={() => handleStop(false)}
                sx={{ bgcolor: '#ea4335', '&:hover': { bgcolor: '#c5221f' }, borderRadius: 9999, px: 3, py: 1, fontWeight: 600, textTransform: 'none', fontSize: 14 }}
              >
                終了
              </Button>
            </Tooltip>
          )}
        </Box>

        {/* Right: secondary actions */}
        <Box sx={{ display: { xs: 'none', md: 'flex' }, alignItems: 'center', gap: 1 }}>
          <Tooltip title="募集要項">
            <IconButton sx={{ color: '#9aa0a6', '&:hover': { bgcolor: 'rgba(255,255,255,0.08)' } }} onClick={() => {}}>
              <InfoOutlinedIcon />
            </IconButton>
          </Tooltip>
          <Typography variant="caption" sx={{ color: '#5f6368', ml: 1 }}>
            質問 {currentQuestionIndex}/{totalQuestionCount} • 残り {Math.floor(remainingSeconds / 60)}:{String(remainingSeconds % 60).padStart(2, '0')}
          </Typography>
        </Box>
      </Box>

      <audio ref={aiAudioRef} />

      {consentDialog}
    </Box>
  )
}

function getNextAvatarGender(): 'male' | 'female' {
  try {
    const key = 'interview_avatar_index'
    const current = Number(localStorage.getItem(key) || '0')
    const next = current + 1
    localStorage.setItem(key, String(next))
    return next % 2 === 0 ? 'female' : 'male'
  } catch {
    return 'male'
  }
}

export default function InterviewPage() {
  return (
    <Suspense fallback={null}>
      <InterviewContent />
    </Suspense>
  )
}
