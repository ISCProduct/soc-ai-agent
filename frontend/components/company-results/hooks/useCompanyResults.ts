'use client'

import { useState, useEffect, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import { authService } from '@/lib/auth'
import { generateITCompanies } from '../data'
import type { Company, UserData } from '../types'

/**
 * CompanyResults の状態・副作用・ハンドラを集約するフック。
 */
export function useCompanyResults(userData: UserData) {
  const router = useRouter()
  const [companies, setCompanies] = useState<Company[]>([])
  const [loading, setLoading] = useState(true)
  const [isProvisional, setIsProvisional] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [applyingId, setApplyingId] = useState<number | null>(null)

  // バックエンドからAI分析済みのスコアを取得
  useEffect(() => {
    const fetchRecommendations = async () => {
      try {
        // セッションIDを取得
        const sessionId = typeof window !== 'undefined'
          ? localStorage.getItem('chat_session_id')
          : null

        if (!sessionId) {
          console.error('No session ID found')
          setError('診断結果が見つかりませんでした。チャットで診断を完了してから再度お試しください。')
          setLoading(false)
          return
        }

        // バックエンドから推奨企業を取得
        const response = await fetch(`/api/chat/recommendations?session_id=${sessionId}&limit=10`, {
          headers: authService.getUserFetchHeaders(),
        })

        if (response.ok) {
          const data = await response.json()
          const recommendations = Array.isArray(data) ? data : data?.recommendations
          setIsProvisional(Boolean(data?.is_provisional))

          // バックエンドのレスポンスをフロントエンドの形式に変換
          const formattedCompanies = (recommendations || []).map((rec: {
            id: number
            match_id?: number
            category_name?: string
            industry?: string
            location?: string
            employees?: string
            reason?: string
            score?: number
            tech_stack?: string[]
            is_applied?: boolean
          }) => ({
            id: rec.id,
            matchId: rec.match_id,
            isApplied: rec.is_applied || false,
            name: rec.category_name || `企業 ${rec.id}`,
            industry: rec.industry || "IT・ソフトウェア",
            location: rec.location || "東京都",
            employees: rec.employees || "100-500名",
            description: rec.reason || "最先端技術を用いた開発を行う企業です。",
            matchScore: Math.round(rec.score || 0),
            tags: ["技術力重視", "成長企業"],
            techStack: rec.tech_stack || ["React", "Go", "AWS"],
            projectTypes: ["Web開発", "API開発"],
            salary: "400万円〜800万円",
            benefits: ["リモートワーク可", "フレックスタイム制"],
            culture: ["フラットな組織", "技術重視"],
            founded: "2015年",
            website: "https://example.com",
            size: "中規模企業",
          }))

          setCompanies(formattedCompanies)
        } else {
          console.error('Failed to fetch recommendations:', response.statusText)
          // フォールバック: ローカル計算
          const generatedCompanies = generateITCompanies(userData)
          setCompanies(generatedCompanies)
        }
      } catch (error) {
        console.error('Failed to fetch recommendations:', error)
        // エラー時はローカル計算にフォールバック
        const generatedCompanies = generateITCompanies(userData)
        setCompanies(generatedCompanies)
      } finally {
        setLoading(false)
      }
    }

    fetchRecommendations()
  }, [userData])

  const handleShowDetail = useCallback((company: Company) => {
    router.push(`/company/${company.id}`)
  }, [router])

  const handleApply = useCallback(async (company: Company) => {
    if (!company.matchId || company.isApplied || applyingId !== null) return
    setApplyingId(company.matchId)
    try {
      const res = await fetch('/api/applications', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({ company_id: company.id, match_id: company.matchId }),
      })
      if (res.ok) {
        setCompanies((prev) => prev.map((c) =>
          c.matchId === company.matchId ? { ...c, isApplied: true } : c,
        ))
      } else {
        let message = '応募に失敗しました'
        try {
          const err = await res.json()
          if (err?.message) message = err.message
        } catch { /* ignore */ }
        setError(message)
      }
    } catch {
      setError('応募に失敗しました。通信環境をご確認のうえ再度お試しください。')
    } finally {
      setApplyingId(null)
    }
  }, [applyingId])

  return {
    companies,
    loading,
    isProvisional,
    error,
    applyingId,
    handleShowDetail,
    handleApply,
  }
}
