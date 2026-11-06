'use client'

import { useCallback, useEffect, useState } from 'react'
import { Alert } from '@mui/material'
import { authService } from '@/lib/auth'

const POLL_INTERVAL_MS = 60_000

type PeriodStatus = {
  limit_usd: number
  remaining_usd: number
}

type BudgetStatus = {
  user: { daily: PeriodStatus; monthly: PeriodStatus }
  school_blocked: boolean
  limit_configured: boolean
  allowed: boolean
}

function hasUserLimit(status: BudgetStatus): boolean {
  return status.user.daily.limit_usd > 0 || status.user.monthly.limit_usd > 0
}

function formatRemaining(status: BudgetStatus): string {
  const parts: string[] = []
  if (status.user.daily.limit_usd > 0) {
    parts.push(`今日はあと $${status.user.daily.remaining_usd.toFixed(2)}`)
  }
  if (status.user.monthly.limit_usd > 0) {
    parts.push(`今月はあと $${status.user.monthly.remaining_usd.toFixed(2)}`)
  }
  return parts.join('、')
}

// AI の利用上限を、上限が設定されている利用者にだけ知らせる。
// 上限がない利用者や取得に失敗したときは何も表示しない（AI の利用は実行時に別途判定される）。
export function AIBudgetNotice() {
  const [status, setStatus] = useState<BudgetStatus | null>(null)

  const fetchStatus = useCallback(async (): Promise<BudgetStatus | null> => {
    try {
      const response = await fetch('/api/user/ai-budget', {
        headers: authService.getUserFetchHeaders(),
        cache: 'no-store',
      })
      if (!response.ok) return null
      const data = (await response.json()) as BudgetStatus
      setStatus(data)
      return data
    } catch {
      return null
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    let timer: number | undefined
    const stop = () => {
      if (timer !== undefined) window.clearInterval(timer)
      timer = undefined
    }
    // 上限がない利用者には表示するものがないので問い合わせを止める。
    // 取得に失敗したときは上限の有無が分からないので、回復するまで続ける。
    const refresh = async () => {
      const latest = await fetchStatus()
      if (latest && !latest.limit_configured) stop()
    }
    timer = window.setInterval(() => {
      if (!document.hidden) void refresh()
    }, POLL_INTERVAL_MS)
    void refresh().then(() => {
      if (cancelled) stop()
    })
    return () => {
      cancelled = true
      stop()
    }
  }, [fetchStatus])

  if (!status) return null
  if (!status.allowed) {
    return (
      <Alert severity="warning" sx={{ mb: 1, borderRadius: 0 }}>
        {status.school_blocked
          ? '学校のAI利用上限に達したため、今は新しいAI機能を使えません。上限は日本時間の0時（月の上限は毎月1日の0時）に戻ります。'
          : 'AIの利用上限に達したため、今は新しいAI機能を使えません。上限は日本時間の0時（月の上限は毎月1日の0時）に戻ります。'}
      </Alert>
    )
  }
  if (!hasUserLimit(status)) return null
  return (
    <Alert severity="info" sx={{ mb: 1, borderRadius: 0 }}>
      AIの利用上限まで、{formatRemaining(status)}です。
    </Alert>
  )
}
