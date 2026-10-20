'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, CircularProgress, Typography } from '@mui/material'
import { authService } from '@/lib/auth'

type PeriodStatus = {
  limit_usd: number
  used_usd: number
  remaining_usd: number
  exceeded: boolean
}

type ScopeStatus = {
  scope: string
  daily: PeriodStatus
  monthly: PeriodStatus
}

type BudgetStatus = {
  user: ScopeStatus
  school?: ScopeStatus
  allowed: boolean
}

type BudgetResponse = BudgetStatus & { error?: string }

function formatPeriod(label: string, status: PeriodStatus): string {
  const remaining = `$${status.remaining_usd.toFixed(4)}`
  const used = `$${status.used_usd.toFixed(4)}`
  return `${label} 残り ${remaining}（利用 ${used} / 上限 $${status.limit_usd.toFixed(4)}）`
}

export function AIBudgetNotice() {
  const [status, setStatus] = useState<BudgetStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const requestID = useRef(0)

  const fetchStatus = useCallback(async (showLoading = true) => {
    const currentRequestID = ++requestID.current
    if (showLoading) setLoading(true)
    setError('')
    try {
      const response = await fetch('/api/user/ai-budget', {
        headers: authService.getUserFetchHeaders(),
        cache: 'no-store',
      })
      const data = (await response.json()) as BudgetResponse
      if (!response.ok) {
        throw new Error(data.error || `AI利用予算を取得できませんでした (${response.status})`)
      }
      if (currentRequestID === requestID.current) setStatus(data)
    } catch (err: unknown) {
      if (currentRequestID === requestID.current) {
        setError(err instanceof Error ? err.message : 'AI利用予算を取得できませんでした')
      }
    } finally {
      if (showLoading && currentRequestID === requestID.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchStatus()
    const timer = window.setInterval(() => {
      void fetchStatus(false)
    }, 30_000)
    return () => window.clearInterval(timer)
  }, [fetchStatus])

  if (loading) {
    return (
      <Box role="status" sx={{ display: 'flex', justifyContent: 'center', py: 1 }}>
        <CircularProgress size={20} aria-label="AI利用予算を読み込んでいます" />
      </Box>
    )
  }
  if (error) {
    return (
      <Alert
        severity="error"
        sx={{ mb: 1, borderRadius: 0 }}
        action={<Button color="inherit" size="small" onClick={() => void fetchStatus()}>再読み込み</Button>}
      >
        AI利用予算を確認できませんでした。{error}
      </Alert>
    )
  }
  if (!status) return null

  const entries: string[] = []
  if (status.user.daily.limit_usd > 0) {
    entries.push(formatPeriod('今日（個人）', status.user.daily))
  }
  if (status.user.monthly.limit_usd > 0) {
    entries.push(formatPeriod('今月（個人）', status.user.monthly))
  }
  if (status.school?.daily.limit_usd) {
    entries.push(formatPeriod('今日（学校）', status.school.daily))
  }
  if (status.school?.monthly.limit_usd) {
    entries.push(formatPeriod('今月（学校）', status.school.monthly))
  }
  if (entries.length === 0) return null

  return (
    <Alert severity={status.allowed ? 'info' : 'warning'} sx={{ mb: 1, borderRadius: 0 }}>
      <Typography component="span" variant="body2" fontWeight={600}>
        AI利用予算
      </Typography>
      <Box component="ul" sx={{ m: 0, mt: 0.5, pl: 2 }}>
        {entries.map((entry) => <li key={entry}>{entry}</li>)}
      </Box>
      {!status.allowed && (
        <Typography component="span" variant="body2">
          上限に達しているため、新しいAIリクエストは利用できません。
        </Typography>
      )}
    </Alert>
  )
}
