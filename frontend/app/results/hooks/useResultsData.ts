'use client'

import { useState, useEffect, useCallback, useRef, type MouseEvent } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { sendAnalysisReport } from '@/lib/api'
import { fetchWithTimeout } from '@/lib/fetch-timeout'
import { authService } from '@/lib/auth'
import { buildResultsPath, getResultsSessionContext } from '@/lib/results-navigation'
import {
  GUEST_APPLICATIONS_DISABLED_REASON,
  GUEST_EMAIL_DISABLED_REASON,
} from '@/lib/guest-limits'
import {
  fetchCompanyRelations,
  fetchCompanyMarketInfo,
  type CapitalRelation,
  type CompanyMarketInfo,
} from '@/lib/company-data'
import type {
  AnalysisScores,
  Company,
  SnackbarState,
  SuggestedRole,
} from '../types'
import {
  buildEmptyRecommendationsMessage,
  mapAnalysisScores,
  mapRecommendationToCompany,
} from '../utils'

/**
 * 結果ページのデータ取得・セッション導線・応募/お気に入り/メール操作。
 */
export function useResultsData() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [companies, setCompanies] = useState<Company[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [mounted, setMounted] = useState(false)
  const [isProvisional, setIsProvisional] = useState(false)
  const [jobSuitabilityComment, setJobSuitabilityComment] = useState('')
  const [suggestedRoles, setSuggestedRoles] = useState<SuggestedRole[]>([])
  const [scoreComment, setScoreComment] = useState('')
  const [analysisScores, setAnalysisScores] = useState<AnalysisScores | null>(null)
  const [analysisError, setAnalysisError] = useState<string | null>(null)
  const [selectedCompany, setSelectedCompany] = useState<Company | null>(null)
  const [detailTab, setDetailTab] = useState(0)
  const [relations, setRelations] = useState<CapitalRelation[]>([])
  const [marketInfo, setMarketInfo] = useState<CompanyMarketInfo[]>([])
  const [diagramLoading, setDiagramLoading] = useState(false)
  const [diagramError, setDiagramError] = useState<string | null>(null)
  const [emailSending, setEmailSending] = useState(false)
  const [favoritingId, setFavoritingId] = useState<number | null>(null)
  const [applyingId, setApplyingId] = useState<number | null>(null)
  const [snackbar, setSnackbar] = useState<SnackbarState>({
    open: false,
    message: '',
    severity: 'success',
  })

  const userId = searchParams.get('user_id')
  const sessionId = searchParams.get('session_id')
  const isGuestUser = authService.getStoredUser()?.is_guest === true
  const analysisAbortRef = useRef<AbortController | null>(null)

  useEffect(() => {
    setMounted(true)
  }, [])

  useEffect(() => {
    if (!mounted) return

    if (userId && sessionId) return

    const context = getResultsSessionContext()
    if (context) {
      router.replace(buildResultsPath(context))
      return
    }

    router.replace('/')
  }, [mounted, userId, sessionId, router])

  const fetchAnalysis = useCallback(async () => {
    if (!sessionId) return
    analysisAbortRef.current?.abort()
    const abortController = new AbortController()
    analysisAbortRef.current = abortController
    setAnalysisError(null)
    // session_id変更時に前セッションの分析テキスト/スコアが新セッションの企業一覧と
    // 並んで残存しないよう、fetch実行前に初期値へリセットする（Issue #949）
    setJobSuitabilityComment('')
    setSuggestedRoles([])
    setScoreComment('')
    setAnalysisScores(null)
    try {
      const res = await fetch(`/api/chat/analysis?session_id=${sessionId}`, {
        headers: authService.getUserFetchHeaders(),
        signal: abortController.signal,
      })
      if (!res.ok) throw new Error('分析データの取得に失敗しました')
      const data = await res.json()
      if (abortController.signal.aborted) return
      if (data?.job_suitability_comment) {
        setJobSuitabilityComment(data.job_suitability_comment)
      }
      if (data?.suggested_roles) {
        setSuggestedRoles(data.suggested_roles)
      }
      if (data?.score_comment) {
        setScoreComment(data.score_comment)
      }
      if (data?.scores) {
        setAnalysisScores(mapAnalysisScores(data.scores))
      }
    } catch (err) {
      if (abortController.signal.aborted || (err instanceof DOMException && err.name === 'AbortError')) {
        return
      }
      setAnalysisError('分析データの取得に失敗しました')
    }
  }, [sessionId])

  useEffect(() => {
    if (!mounted || !userId || !sessionId) {
      return
    }

    // #988: session_id変更で古いリクエストがまだ飛んでいる間に新しいリクエストが
    // 始まると、後から解決した古いレスポンスが新しいセッションの表示を上書きして
    // しまうことがあった。ignoreフラグで古いレスポンスの反映を止める。
    let ignore = false

    const fetchCompanies = async () => {
      try {
        setLoading(true)
        // 再フェッチ時に前回の error が残ると一覧が描画されない（Bugbot）
        setError(null)
        void fetchAnalysis()

        const response = await fetchWithTimeout(`/api/chat/recommendations?session_id=${sessionId}&limit=10`, {
          headers: authService.getUserFetchHeaders(),
        })

        if (ignore) return

        if (!response.ok) {
          throw new Error('企業データの取得に失敗しました')
        }

        const data = await response.json()

        if (ignore) return

        setIsProvisional(Boolean(data?.is_provisional))

        if (!data || !data.recommendations || !Array.isArray(data.recommendations) || data.recommendations.length === 0) {
          const reason = data?.reason || 'matching_results_empty'
          const diagnostics = data?.diagnostics
          setCompanies([])
          setError(buildEmptyRecommendationsMessage(reason, diagnostics))
          setLoading(false)
          return
        }

        if (data && data.recommendations && Array.isArray(data.recommendations)) {
          const mappedCompanies = data.recommendations.map(
            // eslint-disable-next-line @typescript-eslint/no-explicit-any -- API レスポンス
            (rec: any, index: number) => mapRecommendationToCompany(rec, index),
          )
          setCompanies(mappedCompanies)
          setError(null)
        } else {
          setError('企業データの形式が正しくありません')
        }
      } catch (err) {
        if (ignore) return
        setError(err instanceof Error ? err.message : '不明なエラー')
      } finally {
        if (!ignore) setLoading(false)
      }
    }

    fetchCompanies()

    return () => {
      ignore = true
    }
  }, [mounted, userId, sessionId, fetchAnalysis])

  // 企業詳細を開いたときに関係図データを取得
  useEffect(() => {
    if (selectedCompany && (detailTab === 1 || detailTab === 2)) {
      const loadDiagramData = async () => {
        if (relations.length === 0 || marketInfo.length === 0) {
          setDiagramLoading(true)
          setDiagramError(null)
          try {
            const [relationsData, marketData] = await Promise.all([
              fetchCompanyRelations(),
              fetchCompanyMarketInfo(),
            ])
            setRelations(relationsData)
            setMarketInfo(marketData)
          } catch (err) {
            setDiagramError(err instanceof Error ? err.message : '関連図データの取得に失敗しました')
          } finally {
            setDiagramLoading(false)
          }
        }
      }
      loadDiagramData()
    }
  }, [selectedCompany, detailTab, relations.length, marketInfo.length])

  const handleSendEmail = async () => {
    const user = authService.getStoredUser()
    if (!user || user.is_guest) {
      setSnackbar({ open: true, message: GUEST_EMAIL_DISABLED_REASON, severity: 'error' })
      return
    }
    if (!userId || !sessionId) return
    setEmailSending(true)
    try {
      const result = await sendAnalysisReport(Number(userId), sessionId)
      setSnackbar({ open: true, message: result.message || '分析レポートを送信しました', severity: 'success' })
    } catch (err) {
      setSnackbar({ open: true, message: err instanceof Error ? err.message : 'メール送信に失敗しました', severity: 'error' })
    } finally {
      setEmailSending(false)
    }
  }

  const handleBack = () => {
    router.push('/')
  }

  const handleReset = () => {
    sessionStorage.removeItem('chatSessionId')
    localStorage.removeItem('currentSessionId')
    router.push('/')
  }

  const handleToggleFavorite = async (e: MouseEvent, company: Company) => {
    e.stopPropagation()
    if (!company.matchId || favoritingId !== null) return
    setFavoritingId(company.matchId)
    try {
      const res = await fetch('/api/chat/favorite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({ match_id: company.matchId }),
      })
      if (res.ok) {
        setCompanies((prev) => prev.map((c) =>
          c.matchId === company.matchId ? { ...c, isFavorited: !c.isFavorited } : c,
        ))
        setSnackbar({
          open: true,
          message: company.isFavorited ? 'お気に入りを解除しました' : 'お気に入りに追加しました',
          severity: 'success',
        })
      } else {
        setSnackbar({ open: true, message: 'お気に入りの更新に失敗しました', severity: 'error' })
      }
    } catch {
      setSnackbar({ open: true, message: 'お気に入りの更新に失敗しました', severity: 'error' })
    } finally {
      setFavoritingId(null)
    }
  }

  const handleApply = async (e: MouseEvent, company: Company) => {
    e.stopPropagation()
    const user = authService.getStoredUser()
    if (!user || user.is_guest) {
      setSnackbar({ open: true, message: GUEST_APPLICATIONS_DISABLED_REASON, severity: 'error' })
      return
    }
    if (!company.matchId || company.isApplied || applyingId !== null) return
    setApplyingId(company.matchId)
    try {
      const res = await fetch('/api/applications', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({
          user_id: Number(userId),
          company_id: Number(company.id),
          match_id: company.matchId,
        }),
      })
      if (res.ok) {
        const data = await res.json()
        setCompanies((prev) => prev.map((c) =>
          c.matchId === company.matchId
            ? { ...c, isApplied: true, applicationId: data.id }
            : c,
        ))
        setSnackbar({ open: true, message: `${company.name} に応募しました`, severity: 'success' })
      } else {
        let message = '応募に失敗しました'
        try {
          const err = await res.json()
          if (err?.error) message = err.error
        } catch { /* ignore */ }
        setSnackbar({ open: true, message, severity: 'error' })
      }
    } catch {
      setSnackbar({ open: true, message: '応募に失敗しました', severity: 'error' })
    } finally {
      setApplyingId(null)
    }
  }

  const handleCloseDetail = () => {
    setSelectedCompany(null)
    setDetailTab(0)
  }

  const handleCloseSnackbar = () => {
    setSnackbar((prev) => ({ ...prev, open: false }))
  }

  const navigate = (path: string) => {
    router.push(path)
  }

  return {
    mounted,
    userId,
    sessionId,
    companies,
    loading,
    error,
    isProvisional,
    jobSuitabilityComment,
    suggestedRoles,
    scoreComment,
    analysisScores,
    analysisError,
    handleRetryAnalysis: fetchAnalysis,
    selectedCompany,
    setSelectedCompany,
    detailTab,
    setDetailTab,
    relations,
    marketInfo,
    diagramLoading,
    diagramError,
    emailSending,
    favoritingId,
    applyingId,
    snackbar,
    isGuestUser,
    handleSendEmail,
    handleBack,
    handleReset,
    handleToggleFavorite,
    handleApply,
    handleCloseDetail,
    handleCloseSnackbar,
    navigate,
  }
}
