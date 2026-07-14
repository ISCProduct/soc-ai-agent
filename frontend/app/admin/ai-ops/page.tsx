'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'

type L1Coverage = {
  published_total: number
  info_fresh: number
  has_profile: number
  needs_warm: number
  info_rate: number
  profile_rate: number
  below_target?: boolean
  alerts?: string[]
}

type CollectionStatus = {
  name: string
  exists: boolean
  count: number
  latest_fetched_at?: string | null
  error?: string
}

type VectorStatus = {
  backend?: string
  collections?: CollectionStatus[]
  total_documents?: number
  error?: string
  message?: string
}

type CostSummary = {
  current_month_cost_usd?: number
  company_search?: {
    month: string
    count: number
    limit: number
    remaining: number
    enforce: boolean
    exceeded: boolean
  }
  realtime?: {
    current_month_cost_usd: number
  }
}

const COST_ALERT_THRESHOLD_USD = 40

const pct = (rate: number) => `${Math.round((rate || 0) * 100)}%`

export default function AdminAIOpsPage() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [coverage, setCoverage] = useState<L1Coverage | null>(null)
  const [vector, setVector] = useState<VectorStatus | null>(null)
  const [costs, setCosts] = useState<CostSummary | null>(null)
  const [warming, setWarming] = useState(false)
  const [actionMessage, setActionMessage] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    const headers = authService.getAdminFetchHeaders()
    try {
      const [covRes, vecRes, costRes] = await Promise.all([
        fetch('/api/admin/companies/l1-coverage', { headers, cache: 'no-store' }),
        fetch('/api/admin/vector/status', { headers, cache: 'no-store' }),
        fetch('/api/admin/costs', { headers, cache: 'no-store' }),
      ])
      const [covData, vecData, costData] = await Promise.all([
        covRes.ok ? covRes.json() : null,
        vecRes.ok ? vecRes.json() : null,
        costRes.ok ? costRes.json() : null,
      ])
      if (covData) setCoverage(covData)
      if (vecData) setVector(vecData)
      if (costData) setCosts(costData)
      if (!covRes.ok && !vecRes.ok && !costRes.ok) {
        setError('要約データの取得に失敗しました')
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '読み込みに失敗しました')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleWarmL1 = async (force: boolean) => {
    setWarming(true)
    setActionMessage('')
    try {
      const res = await fetch('/api/admin/companies/warm-l1', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({
          limit: 50,
          dry_run: false,
          force,
          include_info: true,
          include_persona: true,
        }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setActionMessage(data?.error || 'ウォームに失敗しました')
        return
      }
      setActionMessage(
        force
          ? `強制ウォーム完了（再調査あり）: 処理 ${data.processed ?? 0} / info ${data.info_ok ?? 0} / persona ${data.persona_ok ?? 0}`
          : `キャッシュ優先ウォーム完了: 処理 ${data.processed ?? 0} / info ${data.info_ok ?? 0} / persona ${data.persona_ok ?? 0}`,
      )
      await load()
    } finally {
      setWarming(false)
    }
  }

  const handleSeedL1 = async () => {
    setWarming(true)
    setActionMessage('')
    try {
      const res = await fetch('/api/admin/companies/seed-l1', {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setActionMessage(data?.error || 'シードに失敗しました')
        return
      }
      setActionMessage(`L1 シード完了: ${JSON.stringify(data)}`)
      await load()
    } finally {
      setWarming(false)
    }
  }

  const monthCost = costs?.current_month_cost_usd ?? 0
  const overThreshold = monthCost >= COST_ALERT_THRESHOLD_USD
  const search = costs?.company_search
  const collections = vector?.collections ?? []

  return (
    <Box sx={{ p: 4, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin">
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h4" fontWeight="bold" flex={1}>
          AI / RAG 運用
        </Typography>
        {loading && <CircularProgress size={22} />}
        <Button size="small" onClick={load} disabled={loading}>
          再読み込み
        </Button>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        RAG・DB キャッシュを優先し、LLM / WebSearch を必要なときだけ使うための運用ハブです。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}
      {actionMessage && (
        <Alert severity="info" sx={{ mb: 2 }} onClose={() => setActionMessage('')}>
          {actionMessage}
        </Alert>
      )}

      <Alert severity="info" sx={{ mb: 3 }}>
        <Typography variant="body2" fontWeight={600} gutterBottom>
          コスト削減の基本方針
        </Typography>
        <Typography variant="body2" component="div">
          1. DB / RAG に新鮮な企業情報があれば再調査しない（安価）
          <br />
          2. 不足分・期限切れのみ L1 ウォームや再埋め込みで補う
          <br />
          3. 「強制再取得」は LLM・Search を伴うため高コスト — 明示的に実行
        </Typography>
      </Alert>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 3 }} flexWrap="wrap" useFlexGap>
        <Card sx={{ flex: 1, minWidth: 220 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              今月 API コスト
            </Typography>
            <Typography
              variant="h4"
              fontWeight={700}
              color={overThreshold ? 'error.main' : monthCost > 20 ? 'warning.main' : 'success.main'}
            >
              ${monthCost.toFixed(4)}
            </Typography>
            <Chip
              size="small"
              sx={{ mt: 1 }}
              color={overThreshold ? 'error' : 'default'}
              label={`閾値 $${COST_ALERT_THRESHOLD_USD}${overThreshold ? ' 超過' : ' 未満'}`}
            />
            <Typography variant="caption" display="block" color="text.secondary" sx={{ mt: 1 }}>
              Realtime: ${(costs?.realtime?.current_month_cost_usd ?? 0).toFixed(4)}
            </Typography>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 220 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              企業 Search（今月）
            </Typography>
            <Typography
              variant="h4"
              fontWeight={700}
              color={search?.exceeded ? 'error.main' : 'text.primary'}
            >
              {(search?.count ?? 0).toLocaleString()}
              <Typography component="span" variant="h6" color="text.secondary">
                {' '}/ {(search?.limit ?? 0).toLocaleString()}
              </Typography>
            </Typography>
            <Typography variant="caption" color="text.secondary">
              残 {search?.remaining ?? '—'}
              {search?.enforce === false ? ' (observe)' : ''}
            </Typography>
            {search && (
              <LinearProgress
                variant="determinate"
                value={Math.min(100, (search.count / Math.max(1, search.limit)) * 100)}
                sx={{ mt: 1.5, height: 8, borderRadius: 1 }}
                color={search.exceeded ? 'error' : 'primary'}
              />
            )}
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 220 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              L1 企業カバレッジ
            </Typography>
            {coverage ? (
              <>
                <Typography variant="h4" fontWeight={700}>
                  {pct(coverage.info_rate)}
                </Typography>
                <Typography variant="caption" color="text.secondary" display="block">
                  info 新鮮 {coverage.info_fresh}/{coverage.published_total} · profile {pct(coverage.profile_rate)}
                </Typography>
                <Chip
                  size="small"
                  sx={{ mt: 1 }}
                  color={coverage.needs_warm > 0 ? 'warning' : 'success'}
                  label={`要ウォーム ${coverage.needs_warm}`}
                />
              </>
            ) : (
              <Typography color="text.secondary">—</Typography>
            )}
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 220 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              VectorDB ドキュメント
            </Typography>
            <Typography variant="h4" fontWeight={700}>
              {(vector?.total_documents ?? 0).toLocaleString()}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              backend: {vector?.backend || '—'} · collections {collections.length}
            </Typography>
            {vector?.error && (
              <Alert severity="warning" sx={{ mt: 1, py: 0 }}>
                {vector.error}
              </Alert>
            )}
          </CardContent>
        </Card>
      </Stack>

      {coverage?.alerts && coverage.alerts.length > 0 && (
        <Alert severity="warning" sx={{ mb: 3 }}>
          {coverage.alerts.join(' / ')}
        </Alert>
      )}

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            コスト削減アクション
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            シード・ウォームで DB キャッシュを厚くし、面接・チャットが毎回 Search しない状態を作ります。
          </Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button variant="contained" disabled={warming} onClick={handleSeedL1}>
              L1 カタログシード
            </Button>
            <Button
              variant="outlined"
              color="success"
              disabled={warming}
              onClick={() => handleWarmL1(false)}
              startIcon={warming ? <CircularProgress size={14} color="inherit" /> : undefined}
            >
              ウォーム（キャッシュ優先・安価）
            </Button>
            <Button
              variant="outlined"
              color="warning"
              disabled={warming}
              onClick={() => handleWarmL1(true)}
            >
              強制ウォーム（再調査・高価）
            </Button>
          </Stack>
        </CardContent>
      </Card>

      <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 3 }}>
        <Card sx={{ flex: 1 }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              RAG コレクション
            </Typography>
            {collections.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                コレクション情報がありません
              </Typography>
            ) : (
              <Stack spacing={1}>
                {collections.map((c) => (
                  <Box key={c.name}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Typography variant="body2" fontWeight={600}>
                        {c.name}
                      </Typography>
                      <Chip size="small" label={`${c.count} docs`} />
                      {!c.exists && <Chip size="small" color="warning" label="missing" />}
                    </Stack>
                    {c.latest_fetched_at && (
                      <Typography variant="caption" color="text.secondary">
                        latest: {new Date(c.latest_fetched_at).toLocaleString('ja-JP')}
                      </Typography>
                    )}
                    {c.error && (
                      <Typography variant="caption" color="error">
                        {c.error}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Stack>
            )}
            <Divider sx={{ my: 2 }} />
            <Button variant="contained" component={Link} href="/admin/vector-db">
              ベクトルDB管理へ
            </Button>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1 }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              関連画面
            </Typography>
            <Stack spacing={1.5}>
              <Button variant="outlined" component={Link} href="/admin/costs" fullWidth sx={{ justifyContent: 'flex-start' }}>
                APIコストモニタリング
              </Button>
              <Button variant="outlined" component={Link} href="/admin/companies" fullWidth sx={{ justifyContent: 'flex-start' }}>
                企業管理（シード・ウォーム・個別編集）
              </Button>
              <Button variant="outlined" component={Link} href="/admin/vector-db" fullWidth sx={{ justifyContent: 'flex-start' }}>
                ベクトルDB（reembed）
              </Button>
            </Stack>
            <Alert severity="success" sx={{ mt: 2 }}>
              企業詳細の「キャッシュから読込」は TTL 内なら LLM/Search を呼びません。不足時だけ「強制再取得（高価）」を使ってください。
            </Alert>
          </CardContent>
        </Card>
      </Stack>
    </Box>
  )
}
