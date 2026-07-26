'use client'

import { useEffect, useState, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { interviewLimits } from '@/lib/interview'
import { authService, User } from '@/lib/auth'
import ConsentDialog from './components/ConsentDialog'
import SelectionScreen from './components/SelectionScreen'
import LobbyScreen from './components/LobbyScreen'
import ReportScreen from './components/ReportScreen'
import SessionScreen from './components/SessionScreen'
import { PageLoading } from '@/components/common/PageLoading'
import { POSITIONS } from './constants'
import type { InterviewCompany, Position, InterviewStatus } from './types'
import { resolveCompanyByName } from './utils'
import { useInterviewMedia } from './hooks/useInterviewMedia'
import { useInterviewSession } from './hooks/useInterviewSession'

function InterviewContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  // status は media（connected 時の video アタッチ）と session の両方から参照するため page で保持
  const [status, setStatus] = useState<InterviewStatus>('selection')
  const [interviewCompany, setInterviewCompany] = useState<InterviewCompany | null>(null)
  const [selectedPosition, setSelectedPosition] = useState<Position>(POSITIONS[0])

  // Selection screen state
  const [allCompanies, setAllCompanies] = useState<InterviewCompany[]>([])
  const [companiesLoading, setCompaniesLoading] = useState(false)
  const [companySearch, setCompanySearch] = useState('')
  const [companySourceTab, setCompanySourceTab] = useState<'db' | 'web'>('db')
  const [webSearchResults, setWebSearchResults] = useState<{ name: string; description: string }[]>([])
  const [webSearchLoading, setWebSearchLoading] = useState(false)
  const [positionCategory, setPositionCategory] = useState<'general' | 'sier'>('general')
  const [companyHints, setCompanyHints] = useState<{ style_tags: string[]; top_questions: string[]; company_brief?: string } | null>(null)
  const [hintsLoading, setHintsLoading] = useState(false)

  const media = useInterviewMedia({ loading, status })
  const session = useInterviewSession({
    user,
    interviewCompany,
    selectedPosition,
    media,
    status,
    setStatus,
  })

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

  if (loading || !user) {
    return <PageLoading message="面接画面を準備しています..." />
  }

  const isActive = status === 'connecting' || status === 'connected'
  const isConnected = status === 'connected'
  const questionDurationSeconds = Math.max(60, interviewLimits.questionDurationSeconds || 180)
  const totalQuestionCount = Math.max(1, selectedPosition.questions)
  const questionProgress = Math.min(100, Math.round((session.questionElapsedSeconds / questionDurationSeconds) * 100))
  const questionRemainingSeconds = Math.max(0, questionDurationSeconds - session.questionElapsedSeconds)
  const questionRemainingLabel = (() => {
    if (questionRemainingSeconds <= 0) return '次の質問へ移行中...'
    if (questionRemainingSeconds < 60) return `あと${questionRemainingSeconds}秒で次の質問へ`
    const m = Math.floor(questionRemainingSeconds / 60)
    const s = questionRemainingSeconds % 60
    return s > 0 ? `あと${m}分${s}秒で次の質問へ` : `あと${m}分で次の質問へ`
  })()
  const companyName = interviewCompany?.name || 'AI面接練習'
  const partialAi = session.partialAi

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
      open={session.consentDialogOpen}
      consentGiven={session.consentGiven}
      onConsentChange={session.setConsentGiven}
      onClose={() => session.setConsentDialogOpen(false)}
      onConfirm={() => { session.setConsentDialogOpen(false); session.handleJoin() }}
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
        lobbyPermissionError={media.lobbyPermissionError}
        onRetryPermissions={() => { media.setLobbyPermissionError(null); window.location.reload() }}
        micEnabled={media.micEnabled}
        cameraEnabled={media.cameraEnabled}
        onToggleMic={media.toggleMic}
        onToggleCamera={media.toggleCamera}
        lobbyVideoRef={media.lobbyVideoRef}
        onBack={() => router.push('/')}
        onJoinWithConsent={session.handleJoinWithConsent}
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
        errorMessage={session.errorMessage}
        reportStatus={session.reportStatus}
        report={session.report}
        scoresBefore={session.scoresBefore}
        scoresAfter={session.scoresAfter}
        session={session.session}
        userId={user?.user_id}
        emailSending={session.emailSending}
        emailSent={session.emailSent}
        isGuest={!user || user.is_guest}
        onSendEmail={session.sendReportEmail}
        videoUploadStatus={session.videoUploadStatus}
        videoUploadProgress={session.videoUploadProgress}
        videoSizeWarning={session.videoSizeWarning}
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
      remainingSeconds={session.remainingSeconds}
      sessionWarningShown={session.sessionWarningShown}
      currentQuestionIndex={session.currentQuestionIndex}
      totalQuestionCount={totalQuestionCount}
      questionProgress={questionProgress}
      questionRemainingSeconds={questionRemainingSeconds}
      questionRemainingLabel={questionRemainingLabel}
      isDeepeningQuestion={session.isDeepeningQuestion}
      questionCategory={session.questionCategory}
      avatarGender={session.avatarGender}
      aiLevel={session.aiLevel}
      aiSpeaking={session.aiSpeaking}
      cameraEnabled={media.cameraEnabled}
      captionsVisible={session.captionsVisible}
      handsFreeMode={session.handsFreeMode}
      utterances={session.utterances}
      partialAi={partialAi}
      partialUser={session.partialUser}
      isRecording={session.isRecording}
      turnPending={session.turnPending}
      errorMessage={session.errorMessage}
      sessionVideoCallbackRef={media.sessionVideoCallbackRef}
      transcriptEndRef={session.transcriptEndRef}
      aiAudioRef={session.aiAudioRef}
      consentDialog={consentDialog}
      onToggleCamera={media.toggleCamera}
      onToggleCaptions={() => session.setCaptionsVisible(p => !p)}
      onToggleHandsFree={() => session.setHandsFreeMode(p => !p)}
      onStartRecording={session.startRecording}
      onStopRecording={session.stopRecording}
      onJoin={session.handleJoin}
      onStop={session.handleStop}
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
