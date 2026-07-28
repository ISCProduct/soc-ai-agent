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
            company_id: number
            company_name?: string
            industry?: string
            location?: string
            employees?: string
            description?: string
            reason?: string
            match_score?: number
            tags?: string[]
            tech_stack?: string[]
            project_types?: string[]
            salary?: string
            benefits?: string[]
            culture?: string[]
            founded?: string
            website?: string
            size?: string
          }) => ({
            id: rec.company_id,
            name: rec.company_name || `企業 ${rec.company_id}`,
            industry: rec.industry || "IT・ソフトウェア",
            location: rec.location || "東京都",
            employees: rec.employees || "100-500名",
            description: rec.description || rec.reason || "最先端技術を用いた開発を行う企業です。",
            matchScore: Math.round(rec.match_score || 0),
            tags: rec.tags || ["技術力重視", "成長企業"],
            techStack: rec.tech_stack || ["React", "Go", "AWS"],
            projectTypes: rec.project_types || ["Web開発", "API開発"],
            salary: rec.salary || "400万円〜800万円",
            benefits: rec.benefits || ["リモートワーク可", "フレックスタイム制"],
            culture: rec.culture || ["フラットな組織", "技術重視"],
            founded: rec.founded || "2015年",
            website: rec.website || "https://example.com",
            size: rec.size || "中規模企業",
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

  return {
    companies,
    loading,
    isProvisional,
    handleShowDetail,
  }
}
