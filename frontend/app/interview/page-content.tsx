'use client'

import { useCallback, useEffect, useState, Suspense } from 'react'
import dynamic from 'next/dynamic'
import { useRouter, useSearchParams } from 'next/navigation'
import { interviewLimits } from '@/lib/interview'
import { authService, User } from '@/lib/auth'
import ConsentDialog from './components/ConsentDialog'
import SelectionScreen, { type CompanyHints } from './components/SelectionScreen'
import LobbyScreen from './components/LobbyScreen'
import ReportScreen from './components/ReportScreen'
import { PageLoading } from '@/components/common/PageLoading'
import { GUEST_REGISTER_PATH } from '@/lib/guest-limits'
import { POSITIONS } from './constants'
import type { InterviewCompany, Position, InterviewStatus } from './types'
import { resolveCompanyByName } from './utils'
import { FetchTimeoutError, fetchWithTimeout } from '@/lib/fetch-timeout'
import { fetchWebSearchCompanies, WebSearchError } from '@/lib/interview/web-search'
import {
  clearInterviewLobbyDraft,
  loadInterviewLobbyDraft,
  resolvePositionFromDraft,
  saveInterviewLobbyDraft,
} from './lobbyDraft'
import { useInterviewMedia } from './hooks/useInterviewMedia'
import { useInterviewSession } from './hooks/useInterviewSession'

/** three.js 依存の SessionScreen は選択/ロビーでは不要のため遅延ロード */
const SessionScreen = dynamic(() => import('./components/SessionScreen'), {
  ssr: false,
  loading: () => <PageLoading message="面接セッションを準備しています..." />,
})

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
  const [companiesLoadError, setCompaniesLoadError] = useState<string | null>(null)
  const [companiesRefreshKey, setCompaniesRefreshKey] = useState(0)
  const [companySearch, setCompanySearch] = useState('')
  const [companySourceTab, setCompanySourceTab] = useState<'db' | 'web'>('db')
  const [webSearchResults, setWebSearchResults] = useState<{ name: string; description: string }[]>([])
  const [webSearchLoading, setWebSearchLoading] = useState(false)
  const [webSearchError, setWebSearchError] = useState<string | null>(null)
  const [webSearchRefreshKey, setWebSearchRefreshKey] = useState(0)
  const [positionCategory, setPositionCategory] = useState<'general' | 'sier'>('general')
  const [companyHints, setCompanyHints] = useState<CompanyHints | null>(null)
  const [hintsLoading, setHintsLoading] = useState(false)
  const [hintsError, setHintsError] = useState<string | null>(null)
  const [hintsRefreshKey, setHintsRefreshKey] = useState(0)

  const media = useInterviewMedia({ loading, status })
  const session = useInterviewSession({
    user,
    interviewCompany,
    companyQuestionHints: companyHints?.top_questions ?? [],
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

  // リロード時: sessionStorage の下書きからロビー（企業・職種）を復元する
  useEffect(() => {
    if (loading) return
    if (searchParams.get('company_name')) return
    const draft = loadInterviewLobbyDraft()
    if (!draft) return
    setInterviewCompany(draft.company)
    setSelectedPosition(resolvePositionFromDraft(draft.positionId, draft.positionCategory))
    setPositionCategory(draft.positionCategory)
    setStatus('lobby')
  }, [loading, searchParams])

  // ロビー以降は下書きを保持（リロードで会社選択へ戻らないようにする）
  useEffect(() => {
    if (status !== 'lobby' && status !== 'connecting' && status !== 'connected') return
    if (!interviewCompany?.name) return
    saveInterviewLobbyDraft({
      company: interviewCompany,
      positionId: selectedPosition.id,
      positionCategory,
    })
  }, [status, interviewCompany, selectedPosition.id, positionCategory])

  // 面接完了後は下書きを破棄（次回は選択画面から）
  useEffect(() => {
    if (status === 'finished') clearInterviewLobbyDraft()
  }, [status])

  const retryCompaniesLoad = useCallback(() => {
    setCompaniesRefreshKey(k => k + 1)
  }, [])

  const retryWebSearch = useCallback(() => {
    setWebSearchRefreshKey(k => k + 1)
  }, [])

  // Load company list for selection screen (initial fetch + debounced search)
  useEffect(() => {
    if (loading || companySourceTab !== 'db') return
    let cancelled = false
    const timer = setTimeout(async () => {
      setCompaniesLoading(true)
      setCompaniesLoadError(null)
      try {
        const params = new URLSearchParams({ limit: '50', offset: '0' })
        if (companySearch.trim()) params.set('name', companySearch.trim())
        const r = await fetchWithTimeout(`/api/companies?${params}`, { cache: 'no-store' })
        if (cancelled) return
        if (!r.ok) throw new Error('企業一覧の取得に失敗しました')
        const data = await r.json()
        // r.json() の待ち中に検索語が変わることがある。setState 直前で
        // 再確認して、古い一覧が最新を上書きするのを防ぐ(#1464)。
        if (cancelled) return
        const list: InterviewCompany[] = Array.isArray(data?.companies) ? data.companies : []
        setAllCompanies(list)
      } catch (e) {
        if (cancelled) return
        setCompaniesLoadError(
          e instanceof Error ? e.message : '企業一覧の取得に失敗しました',
        )
        setAllCompanies([])
      } finally {
        if (!cancelled) setCompaniesLoading(false)
      }
    }, companySearch ? 400 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [loading, companySearch, companySourceTab, companiesRefreshKey])

  // 企業別面接傾向ヒント取得
  useEffect(() => {
    setCompanyHints(null)
    setHintsError(null)
    if (!interviewCompany?.name) {
      setHintsLoading(false)
      return
    }
    setHintsLoading(true)
    let cancelled = false
    const timer = setTimeout(() => {
      setHintsLoading(true)
      void fetch('/api/companies/interview-hints', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ company_name: interviewCompany.name, position: selectedPosition.title }),
      })
        .then(async (response) => {
          if (!response.ok) throw new Error('企業の面接傾向を取得できませんでした')
          const data: unknown = await response.json()
          if (
            typeof data !== 'object'
            || data === null
            || !('style_tags' in data)
            || !Array.isArray(data.style_tags)
            || !data.style_tags.every((tag) => typeof tag === 'string')
            || !('top_questions' in data)
            || !Array.isArray(data.top_questions)
            || !data.top_questions.every((question) => typeof question === 'string')
            || ('company_brief' in data && typeof data.company_brief !== 'string')
          ) {
            throw new Error('企業の面接傾向を正しく読み取れませんでした')
          }
          if (!cancelled) setCompanyHints(data as CompanyHints)
        })
        .catch((error: unknown) => {
          if (!cancelled) {
            setCompanyHints(null)
            setHintsError(error instanceof Error ? error.message : '企業の面接傾向を取得できませんでした')
          }
        })
        .finally(() => { if (!cancelled) setHintsLoading(false) })
    }, 600)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [interviewCompany?.name, selectedPosition.title, hintsRefreshKey])

  const retryHints = useCallback(() => {
    setHintsRefreshKey((key) => key + 1)
  }, [])

  // WEB検索
  // 失敗とヒット0件を必ず区別する。以前はどちらも空配列にしていたため、
  // APIが落ちていても「検索結果が見つかりません」と出て、学生には
  // その企業が存在しないように見えていた(#1447)。
  useEffect(() => {
    if (loading || companySourceTab !== 'web') return
    if (!companySearch.trim()) { setWebSearchResults([]); setWebSearchError(null); return }
    let cancelled = false
    const ac = new AbortController()
    const timer = setTimeout(async () => {
      setWebSearchLoading(true)
      setWebSearchError(null)
      try {
        const results = await fetchWebSearchCompanies(companySearch.trim(), ac.signal)
        // JSON パース中に検索語が変わってクリーンアップが走ったら、
        // この結果は古い。setState 直前で再確認して上書きを防ぐ(#1464)。
        if (cancelled) return
        setWebSearchResults(results)
      } catch (e) {
        if (cancelled || ac.signal.aborted) return
        setWebSearchError(
          e instanceof WebSearchError || e instanceof FetchTimeoutError
            ? e.message
            : '検索できませんでした。しばらくしてからもう一度お試しください。',
        )
        setWebSearchResults([])
      } finally {
        if (!cancelled) setWebSearchLoading(false)
      }
    }, 500)
    return () => { cancelled = true; ac.abort(); clearTimeout(timer) }
  }, [loading, companySearch, companySourceTab, webSearchRefreshKey])

  const handleBackToSelection = useCallback(() => {
    clearInterviewLobbyDraft()
    media.stopStream()
    setStatus('selection')
    router.replace('/interview')
  }, [router, setStatus, media])

  if (loading || !user) {
    return <PageLoading message="面接画面を準備しています..." />
  }

  const isActive = status === 'connecting' || status === 'connected'
  const isConnected = status === 'connected'
  const questionDurationSeconds = Math.max(60, interviewLimits.questionDurationSeconds || 180)
  const totalQuestionCount = session.totalQuestionCount
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
        companiesLoadError={companiesLoadError}
        onRetryCompaniesLoad={retryCompaniesLoad}
        webSearchResults={webSearchResults}
        setWebSearchResults={setWebSearchResults}
        webSearchLoading={webSearchLoading}
        webSearchError={webSearchError}
        onRetryWebSearch={retryWebSearch}
        positionCategory={positionCategory}
        setPositionCategory={setPositionCategory}
        selectedPosition={selectedPosition}
        setSelectedPosition={setSelectedPosition}
        companyHints={companyHints}
        hintsLoading={hintsLoading}
        hintsError={hintsError}
        onRetryHints={retryHints}
        questionDurationSeconds={questionDurationSeconds}
        onStartInterview={() => {
          if (interviewCompany?.name) {
            saveInterviewLobbyDraft({
              company: interviewCompany,
              positionId: selectedPosition.id,
              positionCategory,
            })
          }
          setStatus('lobby')
        }}
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
        onRetryPermissions={() => { void media.retryLobbyPreview() }}
        micEnabled={media.micEnabled}
        cameraEnabled={media.cameraEnabled}
        onToggleMic={media.toggleMic}
        onToggleCamera={media.toggleCamera}
        lobbyVideoRef={media.lobbyVideoRef}
        onBack={handleBackToSelection}
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
        emailError={session.emailError}
        isGuest={!user || user.is_guest}
        onRegisterClick={() => router.push(GUEST_REGISTER_PATH)}
        onSendEmail={session.sendReportEmail}
        onRetryReport={() => { void session.retryReportPolling() }}
        reportRetryError={session.reportRetryError}
        finishFailed={session.finishFailed}
        onRetryFinish={session.retryFinish}
        utteranceSaveFailed={session.utteranceSaveFailed}
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
      studentPauseCount={session.studentPauseCount}
      cameraEnabled={media.cameraEnabled}
      captionsVisible={session.captionsVisible}
      handsFreeMode={session.handsFreeMode}
      utterances={session.utterances}
      partialAi={partialAi}
      partialUser={session.partialUser}
      isRecording={session.isRecording}
      turnPending={session.turnPending}
      errorMessage={session.errorMessage}
      utteranceSaveFailed={session.utteranceSaveFailed}
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

export default function PageContent() {
  return (
    <Suspense fallback={<PageLoading message="面接画面を準備しています..." />}>
      <InterviewContent />
    </Suspense>
  )
}
