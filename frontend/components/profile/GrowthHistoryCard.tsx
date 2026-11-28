'use client'

import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  List,
  ListItem,
  ListItemText,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { Alert } from '@/components/common/Alert'
import { BACKEND_URL } from '@/lib/config'

const ENDPOINT = `${BACKEND_URL}/api/user/growth-history`

type CategoryChange = {
  category: string
  previous: number | null
  current: number | null
  delta: number | null
  change_label: string
}

type GrowthHistory = {
  points: { id: number }[]
  has_previous: boolean
  categories: CategoryChange[]
  phase: {
    change_label: string
    newly_completed: string[]
  }
}

function isHistory(value: unknown): value is GrowthHistory {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return false
  const row = value as Partial<GrowthHistory>
  return Array.isArray(row.points)
    && typeof row.has_previous === 'boolean'
    && Array.isArray(row.categories)
    && !!row.phase
    && typeof row.phase.change_label === 'string'
    && Array.isArray(row.phase.newly_completed)
}

function deltaText(delta: number | null): string {
  if (delta == null || delta === 0) return ''
  const sign = delta > 0 ? '+' : ''
  return `（${sign}${delta}）`
}

/**
 * 10カテゴリの適性と診断フェーズが、前回の記録から伸びたかを出す（SOCAIAGENT-601）。
 * 値はシステムの見立て。記録はこの機能を入れてから始まり、過去のスコアは復元しない。
 */
export function GrowthHistoryCard() {
  const [history, setHistory] = useState<GrowthHistory | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [retryToken, setRetryToken] = useState(0)

  useEffect(() => {
    let cancelled = false
    const generation = retryToken
    setLoading(true)
    setError(false)
    void (async () => {
      try {
        const response = await fetch(ENDPOINT, { headers: authService.getUserFetchHeaders() })
        if (!response.ok) throw new Error('failed')
        const data: unknown = await response.json()
        if (!isHistory(data)) throw new Error('invalid')
        if (cancelled || generation !== retryToken) return
        setHistory(data)
      } catch {
        if (cancelled || generation !== retryToken) return
        setHistory(null)
        setError(true)
      } finally {
        if (!cancelled && generation === retryToken) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [retryToken])

  return (
    <Card sx={{ mb: 3, borderRadius: 2 }}>
      <CardContent sx={{ p: 3 }}>
        <Typography variant="subtitle1" component="h2" fontWeight="bold">
          以前と比べた伸び
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
          システムの見立てです。確定した適性ではありません。記録はこの機能を入れてから貯まり、それより前のスコアは残りません。
        </Typography>

        {loading && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mt: 2 }} role="status">
            <CircularProgress size={18} />
            <Typography variant="body2">成長の記録を読み込んでいます</Typography>
          </Box>
        )}

        {!loading && error && (
          <Alert
            severity="error"
            sx={{ mt: 2 }}
            action={
              <Button color="inherit" size="small" onClick={() => setRetryToken((n) => n + 1)}>
                再読み込み
              </Button>
            }
          >
            成長の記録を読み込めませんでした
          </Alert>
        )}

        {!loading && !error && history && history.points.length === 0 && (
          <Typography variant="body2" sx={{ mt: 2 }}>
            まだ記録がありません。診断や検査が進むと、ここに貯まります。
          </Typography>
        )}

        {!loading && !error && history && history.points.length > 0 && !history.has_previous && (
          <Typography variant="body2" sx={{ mt: 2 }}>
            まだ比べられる記録がありません。次に診断や検査が進むと、以前との差がここに出ます。
          </Typography>
        )}

        {!loading && !error && history && history.has_previous && (
          <Box sx={{ mt: 2 }}>
            <Typography variant="body2">
              診断の進み：{history.phase.change_label}
              {history.phase.newly_completed.length > 0
                ? `（新しく終わった段階：${history.phase.newly_completed.join('、')}）`
                : ''}
            </Typography>
            <List dense disablePadding sx={{ mt: 1 }}>
              {history.categories.map((change) => (
                <ListItem key={change.category} disableGutters>
                  <ListItemText
                    primary={`${change.category}：${change.change_label}${deltaText(change.delta)}`}
                  />
                </ListItem>
              ))}
            </List>
          </Box>
        )}
      </CardContent>
    </Card>
  )
}
