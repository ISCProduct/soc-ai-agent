'use client'

import { useCallback, useEffect, useState } from 'react'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService } from '@/lib/auth'
import { sortLanguageStats, upsertRepoSummary } from '../utils'
import type { GitHubProfile, GitHubRepo, LanguageStat, RepoSummary, SkillScore } from '../types'

/**
 * GitHubSkills の状態・副作用・ハンドラを集約するフック。
 */
export function useGitHubSkills(userId: number, targetRole: string) {
  const [scores, setScores] = useState<SkillScore[]>([])
  const [profile, setProfile] = useState<GitHubProfile | null>(null)
  const [langStats, setLangStats] = useState<LanguageStat[]>([])
  const [repos, setRepos] = useState<GitHubRepo[]>([])
  const [summaries, setSummaries] = useState<RepoSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [notLinked, setNotLinked] = useState(false)
  const [needsReauth, setNeedsReauth] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [skillsError, setSkillsError] = useState(false)
  const [profileError, setProfileError] = useState(false)
  const [summariesError, setSummariesError] = useState(false)
  const [connecting, setConnecting] = useState(false)
  const [summarizingRepo, setSummarizingRepo] = useState<string | null>(null)
  const [expandedRepo, setExpandedRepo] = useState<string | false>(false)

  const fetchAll = useCallback(async () => {
    setLoading(true)
    setError(null)
    setNotLinked(false)
    setSkillsError(false)
    setProfileError(false)
    setSummariesError(false)
    try {
      const headers = authService.getUserFetchHeaders()
      const [skillsRes, profileRes, summariesRes] = await Promise.all([
        fetch(`${BACKEND_URL}/api/github/skills?user_id=${userId}`, { headers }),
        fetch(`${BACKEND_URL}/api/github/profile?user_id=${userId}`, { headers }),
        fetch(`${BACKEND_URL}/api/github/repo/summaries?user_id=${userId}`, { headers }),
      ])

      if (skillsRes.ok) {
        const data = await skillsRes.json()
        setScores(Array.isArray(data) ? data : [])
      } else if (skillsRes.status !== 401) {
        // 401（未ログイン・期限切れ）以外は取得失敗としてセクション単位でエラー表示する
        setSkillsError(true)
      }

      if (profileRes.ok) {
        const data = await profileRes.json()
        setProfile(data.profile ?? null)
        setLangStats(
            Array.isArray(data.language_stats)
                ? sortLanguageStats(data.language_stats)
                : [],
        )
        setRepos(Array.isArray(data.repositories) ? data.repositories : [])
      } else if (profileRes.status === 404) {
        setNotLinked(true)
      } else if (profileRes.status !== 401) {
        setProfileError(true)
      }

      if (summariesRes.ok) {
        const data = await summariesRes.json()
        setSummaries(Array.isArray(data) ? data : [])
      } else if (summariesRes.status !== 401) {
        setSummariesError(true)
      }
    } catch {
      // 通信障害時は各セクションを「データなし」ではなくエラー扱いにする
      setError('データの取得に失敗しました')
      setSkillsError(true)
      setProfileError(true)
      setSummariesError(true)
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    fetchAll()
  }, [fetchAll])

  const handleSummarizeRepo = async (fullName: string) => {
    setSummarizingRepo(fullName)
    setError(null)
    try {
      const res = await fetch(`${BACKEND_URL}/api/github/repo/summarize?user_id=${userId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({ full_name: fullName, target_role: targetRole }),
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || `HTTP ${res.status}`)
      }
      const newSummary: RepoSummary = await res.json()
      setSummaries(prev => upsertRepoSummary(prev, newSummary, fullName))
    } catch (e) {
      setError(`要約生成に失敗しました: ${e instanceof Error ? e.message : '不明なエラー'}`)
    } finally {
      setSummarizingRepo(null)
    }
  }

  const handleConnect = () => {
    setConnecting(true)
    window.location.href = `${BACKEND_URL}/api/auth/github`
  }

  const handleSync = async () => {
    setSyncing(true)
    setError(null)
    setNeedsReauth(false)
    try {
      const res = await fetch(`${BACKEND_URL}/api/github/sync/wait?user_id=${userId}`, {
        method: 'POST',
        headers: authService.getUserFetchHeaders(),
      })
      if (res.status === 403) {
        const msg = await res.text()
        setNeedsReauth(true)
        setError(msg)
        return
      }
      if (!res.ok) {
        setError('同期に失敗しました')
        return
      }
      await fetchAll()
    } catch {
      setError('同期に失敗しました')
    } finally {
      setSyncing(false)
    }
  }

  const clearError = () => {
    setError(null)
    setNeedsReauth(false)
  }

  return {
    scores,
    profile,
    langStats,
    repos,
    summaries,
    loading,
    syncing,
    notLinked,
    needsReauth,
    error,
    skillsError,
    profileError,
    summariesError,
    connecting,
    summarizingRepo,
    expandedRepo,
    setExpandedRepo,
    handleSummarizeRepo,
    handleConnect,
    handleSync,
    clearError,
    refetch: fetchAll,
  }
}
