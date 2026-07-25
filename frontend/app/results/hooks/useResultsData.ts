'use client'

import { useState, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { sendAnalysisReport } from '@/lib/api'
import { authService } from '@/lib/auth'
import { buildResultsPath, getResultsSessionContext } from '@/lib/results-navigation'
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
  const [empty, setEmpty] = useState(false)
  const [mounted, setMounted] = useState(false)
  const [isProvisional, setIsProvisional] = useState(false)
  const [jobSuitabilityComment, setJobSuitabilityComment] = useState('')
  const [suggestedRoles, setSuggestedRoles] = useState<SuggestedRole[]>([])
  const [scoreComment, setScoreComment] = useState('')
  const [analysisScores, setAnalysisScores] = useState<AnalysisScores | null>(null)
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

  useEffect(() => {
    if (!mounted || !userId || !sessionId) {
      return
    }

    const fetchCompanies = async () => {
      try {
        setLoading(true)
        console.log('[Results] Fetching recommendations for user:', userId, 'session:', sessionId)

        // 職種適性コメントを取得
        fetch(`/api/chat/analysis?session_id=${sessionId}`, { headers: authService.getUserFetchHeaders() })
          .then((r) => (r.ok ? r.json() : null))
          .then((data) => {
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
          })
          .catch(() => { /* サイレント失敗 */ })

        const response = await fetch(`/api/chat/recommendations?session_id=${sessionId}&limit=10`, {
          headers: authService.getUserFetchHeaders(),
        })

        if (!response.ok) {
          throw new Error('企業データの取得に失敗しました')
        }

        const data = await response.json()
        console.log('[Results] API Response:', data)

        setIsProvisional(Boolean(data?.is_provisional))

        if (!data || !data.recommendations || !Array.isArray(data.recommendations) || data.recommendations.length === 0) {
          console.error('[Results] No recommendations available')
          const reason = data?.reason || 'matching_results_empty'
          const diagnostics = data?.diagnostics
          setError(buildEmptyRecommendationsMessage(reason, diagnostics))
          setLoading(false)
          return
        }

        if (data && data.recommendations && Array.isArray(data.recommendations)) {
          const mappedCompanies = data.recommendations.map(
            // eslint-disable-next-line @typescript-eslint/no-explicit-any -- API レスポンス
            (rec: any, index: number) => {
              console.log('[Results] Mapping company data:', rec)
              return mapRecommendationToCompany(rec, index)
            },
          )
          console.log('[Results] Mapped companies:', mappedCompanies)
          setCompanies(mappedCompanies)
        } else {
          console.error('[Results] Invalid data format:', data)
          setError('企業データの形式が正しくありません')
        }
      } catch (err) {
        console.error('[Results] 企業データ取得エラー:', err)
        setError(err instanceof Error ? err.message : '不明なエラー')
      } finally {
        setLoading(false)
      }
    }

    fetchCompanies()
  }, [mounted, userId, sessionId])

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
      setSnackbar({ open: true, message: 'ゲストユーザーはメール送信できません', severity: 'error' })
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

  const handleToggleFavorite = async (e: React.MouseEvent, company: Company) => {
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
      }
    } finally {
      setFavoritingId(null)
    }
  }

  const handleApply = async (e: React.MouseEvent, company: Company) => {
    e.stopPropagation()
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
        const err = await res.json()
        setSnackbar({ open: true, message: err.error || '応募に失敗しました', severity: 'error' })
      }
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
    empty,
    isProvisional,
    jobSuitabilityComment,
    suggestedRoles,
    scoreComment,
    analysisScores,
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
