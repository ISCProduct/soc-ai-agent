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
  Stack,
  Typography,
  Pagination,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'
import { PageContainer } from '@/components/admin/PageContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { AdminListCard } from '@/components/admin/AdminListCard'
import { StatusBadge } from '@/components/admin/StatusBadge'

const PAGE_SIZE = 50

type Company = {
  id: number
  name: string
  industry?: string
  location?: string
  source_type?: string
  is_provisional?: boolean
  data_status?: string
  info_fetched_at?: string | null
  jobs_fetched_at?: string | null
  tech_fetched_at?: string | null
  relations_fetched_at?: string | null
  website_url?: string
  description?: string
}

type L1Coverage = {
  published_total: number
  info_fresh: number
  has_profile: number
  needs_warm: number
  info_rate: number
  profile_rate: number
  info_target?: number
  profile_target?: number
  below_target?: boolean
  alerts?: string[]
}

const sourceLabel = (sourceType?: string) => {
  if (sourceType === 'official') return '公式'
  if (sourceType === 'job_site') return 'クローリング'
  return '手動'
}

const pct = (rate: number) => `${Math.round((rate || 0) * 100)}%`

export default function AdminCompaniesPage() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [companies, setCompanies] = useState<Company[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [filterStatus, setFilterStatus] = useState<'all' | 'draft' | 'published'>('all')
  const [coverage, setCoverage] = useState<L1Coverage | null>(null)
  const [warming, setWarming] = useState(false)
  const [warmMessage, setWarmMessage] = useState('')
  const [fetchAllId, setFetchAllId] = useState<number | null>(null)
  const [fetchAllMessage, setFetchAllMessage] = useState('')
  const [missingBatchLoading, setMissingBatchLoading] = useState(false)

  const fetchCoverage = useCallback(async () => {
    const res = await fetch('/api/admin/companies/l1-coverage', {
      headers: authService.getAdminFetchHeaders(),
      cache: 'no-store',
    })
    if (!res.ok) return
    const data = await res.json()
    setCoverage(data)
  }, [])

  const fetchCompanies = async (p: number = page) => {
    setError('')
    const offset = (p - 1) * PAGE_SIZE
    const res = await fetch(`/api/admin/companies?limit=${PAGE_SIZE}&offset=${offset}`, {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '企業一覧の取得に失敗しました')
      return
    }
    setCompanies(data?.companies || [])
    setTotal(data?.total ?? 0)
  }

  useEffect(() => {
    fetchCompanies(1)
    fetchCoverage()
  }, [fetchCoverage])

  const handlePageChange = (_: React.ChangeEvent<unknown>, value: number) => {
    setPage(value)
    fetchCompanies(value)
  }

  const handleSeedL1 = async () => {
    setWarming(true)
    setWarmMessage('')
    setError('')
    try {
      const res = await fetch('/api/admin/companies/seed-l1', {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || data?.message || 'L1シード投入に失敗しました')
        return
      }
      setWarmMessage(
        `シード投入: 作成 ${data.created ?? 0} / 更新 ${data.updated ?? 0} / スキップ ${data.skipped ?? 0}`,
      )
      await fetchCoverage()
      await fetchCompanies(page)
    } finally {
      setWarming(false)
    }
  }

  const handleWarmL1 = async (dryRun: boolean) => {
    setWarming(true)
    setWarmMessage('')
    setError('')
    try {
      const res = await fetch('/api/admin/companies/warm-l1', {
        method: 'POST',
        headers: {
          ...authService.getAdminFetchHeaders(),
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ limit: 100, dry_run: dryRun, include_info: true, include_persona: true }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || data?.message || 'L1温存に失敗しました')
        return
      }
      if (dryRun) {
        setWarmMessage(`ドライラン: 候補 ${data.candidate_count ?? 0} 社（上限 ${data.limit}）`)
      } else {
        setWarmMessage(
          `温存完了: 処理 ${data.processed ?? 0} / info ${data.info_ok ?? 0} / persona ${data.persona_ok ?? 0} / エラー ${data.errors ?? 0}`,
        )
      }
      if (data.coverage) setCoverage(data.coverage)
      else await fetchCoverage()
    } finally {
      setWarming(false)
    }
  }

  const handleFetchMissingBatch = async (dryRun: boolean) => {
    setMissingBatchLoading(true)
    setFetchAllMessage('')
    setError('')
    try {
      const res = await fetch('/api/admin/companies/fetch-missing-batch', {
        method: 'POST',
        headers: {
          ...authService.getAdminFetchHeaders(),
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ limit: 20, dry_run: dryRun }),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError(data?.error || data?.message || `不足データ一括取得に失敗しました (${res.status})`)
        return
      }
      if (dryRun) {
        const names = Array.isArray(data.items)
          ? (data.items as { name?: string }[]).slice(0, 5).map((i) => i.name).filter(Boolean).join(', ')
          : ''
        setFetchAllMessage(
          `不足データ対象: ${data.candidate_n ?? 0} 社（上限 ${data.limit}）` +
            (names ? ` — 例: ${names}${(data.candidate_n ?? 0) > 5 ? ' …' : ''}` : ''),
        )
      } else {
        setFetchAllMessage(
          `不足データ一括取得完了: 処理 ${data.processed ?? 0} 社` +
            ` / 基本情報 ${data.info_ok ?? 0}` +
            ` / 求人 ${data.jobs_ok ?? 0}` +
            ` / Tech ${data.tech_ok ?? 0}` +
            ` / 関係 ${data.relations_ok ?? 0}` +
            ` / エラー ${data.errors ?? 0}`,
        )
        if ((data.errors ?? 0) > 0) {
          setError(`一部の企業で取得に失敗しました（エラー ${data.errors} 件）。詳細はサーバーログを確認してください。`)
        }
        await fetchCompanies(page)
        await fetchCoverage()
      }
    } finally {
      setMissingBatchLoading(false)
    }
  }

  const handlePublish = async (companyId: number) => {
    setError('')
    const res = await fetch(`/api/admin/companies/${companyId}/publish`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(data?.error || `承認に失敗しました (${res.status})`)
      return
    }
    fetchCompanies(page)
  }

  const handleReject = async (companyId: number) => {
    setError('')
    const res = await fetch(`/api/admin/companies/${companyId}/reject`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(data?.error || `却下に失敗しました (${res.status})`)
      return
    }
    fetchCompanies(page)
  }

  const summarizeFetchAll = (data: Record<string, unknown>) => {
    const stepLabel = (key: string, label: string) => {
      const step = data[key] as { status?: string; detail?: string; count?: number } | undefined
      if (!step?.status) return null
      if (step.status === 'fetched') {
        return step.count != null ? `${label}取得(${step.count})` : `${label}取得`
      }
      if (step.status === 'skipped') return `${label}スキップ`
      if (step.status === 'error') return `${label}失敗`
      return null
    }
    return [
      stepLabel('info_step', '基本情報'),
      stepLabel('jobs_step', '求人'),
      stepLabel('tech_step', 'Tech'),
      stepLabel('relations_step', '関係'),
    ]
      .filter(Boolean)
      .join(' / ')
  }

  const handleFetchAll = async (companyId: number) => {
    setError('')
    setFetchAllMessage('')
    setFetchAllId(companyId)
    try {
      const res = await fetch(`/api/admin/companies/${companyId}/fetch-all`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError(data?.error || `一括取得に失敗しました (${res.status})`)
        return
      }
      const summary = summarizeFetchAll(data)
      if (data.ok === false && Array.isArray(data.errors) && data.errors.length > 0) {
        setError(`一部失敗: ${(data.errors as string[]).join('; ')}`)
      }
      setFetchAllMessage(summary ? `一括取得完了: ${summary}` : '一括取得完了')
      await fetchCompanies(page)
      await fetchCoverage()
    } finally {
      setFetchAllId(null)
    }
  }

  const missingLabels = (c: Company) => {
    const labels: string[] = []
    if (!c.info_fetched_at || !c.description) labels.push('基本情報')
    if (!c.jobs_fetched_at) labels.push('求人')
    if (!c.tech_fetched_at) labels.push('Tech')
    if (!c.relations_fetched_at) labels.push('関係')
    return labels
  }

  const filteredCompanies = companies.filter((c) => {
    if (filterStatus === 'all') return true
    return c.data_status === filterStatus
  })

  const pageCount = Math.ceil(total / PAGE_SIZE)

  return (
    <PageContainer maxWidth={1000}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
          <Typography variant="h4" fontWeight="bold">
            企業管理
          </Typography>
        </Stack>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" size="small" component={Link} href="/admin/job-positions">
            求人管理
          </Button>
          <Button variant="outlined" size="small" component={Link} href="/admin/graduate-employments">
            就職情報管理
          </Button>
          <Button variant="contained" size="small" component={Link} href="/admin/companies/new">
            + 企業を追加
          </Button>
        </Stack>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        企業情報の公開ステータスを管理します。公開済みでも「基本情報・URL取得」から AI 取得・更新できます。（全 {total.toLocaleString()} 件）
      </Typography>

      <ErrorAlert error={error} />
      {fetchAllMessage && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setFetchAllMessage('')}>
          {fetchAllMessage}
        </Alert>
      )}

      <Card sx={{ mb: 2 }}>
        <CardContent>
          <Stack spacing={2}>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }} justifyContent="space-between">
              <Box>
                <Typography variant="h6" sx={{ mb: 0.5 }}>不足データの一括取得</Typography>
                <Typography variant="body2" color="text.secondary">
                  未取得／TTL切れの基本情報・求人・Tech・関係だけを埋めます（1回あたり最大20社・公開優先）。
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Button
                  variant="outlined"
                  size="small"
                  disabled={missingBatchLoading || warming}
                  onClick={() => handleFetchMissingBatch(true)}
                >
                  対象確認
                </Button>
                <Button
                  variant="contained"
                  color="secondary"
                  size="small"
                  disabled={missingBatchLoading || warming}
                  onClick={() => handleFetchMissingBatch(false)}
                  startIcon={missingBatchLoading ? <CircularProgress size={14} color="inherit" /> : null}
                >
                  {missingBatchLoading ? '一括取得中...' : '不足データを一括取得'}
                </Button>
              </Stack>
            </Stack>
            <Divider />
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }} justifyContent="space-between">
            <Box>
              <Typography variant="h6" sx={{ mb: 0.5 }}>L1 カタログ充足（マッチング用）</Typography>
              <Typography variant="body2" color="text.secondary">
                公開企業の基本情報 TTL 内 + WeightProfile。Core/中小SI を優先して日次温存します。
              </Typography>
              {coverage && (
                <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mt: 1.5 }}>
                  <Chip size="small" label={`公開 ${coverage.published_total}`} />
                  <Chip
                    size="small"
                    color={coverage.info_rate < (coverage.info_target ?? 0.8) ? 'error' : 'success'}
                    label={`Info ${pct(coverage.info_rate)} / 目標 ${pct(coverage.info_target ?? 0.8)}`}
                  />
                  <Chip
                    size="small"
                    color={coverage.profile_rate < (coverage.profile_target ?? 0.95) ? 'error' : 'info'}
                    label={`Profile ${pct(coverage.profile_rate)} / 目標 ${pct(coverage.profile_target ?? 0.95)}`}
                  />
                  <Chip size="small" color="warning" label={`要温存 ${coverage.needs_warm}`} />
                  {coverage.below_target && (
                    <Chip size="small" color="error" label="閾値割れ" />
                  )}
                </Stack>
              )}
              {coverage?.alerts?.length ? (
                <Typography variant="body2" color="error" sx={{ mt: 1 }}>
                  {coverage.alerts.join(' / ')}
                </Typography>
              ) : null}
              {warmMessage && (
                <Typography variant="body2" sx={{ mt: 1 }}>{warmMessage}</Typography>
              )}
            </Box>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <Button variant="outlined" size="small" disabled={warming} onClick={() => handleSeedL1()}>
                サンプルシード投入
              </Button>
              <Button variant="outlined" size="small" disabled={warming} onClick={() => handleWarmL1(true)}>
                ドライラン
              </Button>
              <Button
                variant="contained"
                size="small"
                disabled={warming}
                onClick={() => handleWarmL1(false)}
                startIcon={warming ? <CircularProgress size={14} color="inherit" /> : undefined}
              >
                L1温存（最大100社）
              </Button>
            </Stack>
          </Stack>
          </Stack>
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Typography variant="h6">企業一覧</Typography>
            <Stack direction="row" spacing={1}>
              {(['all', 'draft', 'published'] as const).map((s) => (
                <Chip
                  key={s}
                  label={s === 'all' ? 'すべて' : s === 'draft' ? '下書き' : '公開'}
                  variant={filterStatus === s ? 'filled' : 'outlined'}
                  color={s === 'published' ? 'success' : s === 'draft' ? 'warning' : 'default'}
                  onClick={() => setFilterStatus(s)}
                  clickable
                />
              ))}
            </Stack>
          </Stack>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={1}>
            {filteredCompanies.map((company) => (
              <AdminListCard key={company.id}>
                <Stack direction="row" alignItems="center" justifyContent="space-between">
                  <Box>
                    <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                      <Typography variant="subtitle1" fontWeight="bold">
                        {company.name}
                      </Typography>
                      <StatusBadge status={company.data_status} />
                      <Chip label={sourceLabel(company.source_type)} size="small" variant="outlined" />
                      {company.is_provisional && (
                        <Chip label="暫定" size="small" color="default" variant="outlined" />
                      )}
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {company.industry || '業種未設定'} / {company.location || '所在地未設定'}
                    </Typography>
                    {missingLabels(company).length > 0 && (
                      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap sx={{ mt: 0.5 }}>
                        {missingLabels(company).map((label) => (
                          <Chip key={label} size="small" color="warning" variant="outlined" label={`未取得: ${label}`} />
                        ))}
                      </Stack>
                    )}
                  </Box>
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    <Button
                      variant="contained"
                      color="secondary"
                      size="small"
                      onClick={() => handleFetchAll(company.id)}
                      disabled={fetchAllId === company.id || warming}
                      startIcon={fetchAllId === company.id ? <CircularProgress size={14} color="inherit" /> : null}
                    >
                      {fetchAllId === company.id ? '一括取得中...' : '不足情報を一括取得'}
                    </Button>
                    <Button
                      variant="contained"
                      size="small"
                      component={Link}
                      href={`/admin/companies/${company.id}/info`}
                    >
                      基本情報・URL取得
                    </Button>
                    <Button
                      variant="outlined"
                      size="small"
                      component={Link}
                      href={`/admin/companies/${company.id}/relations`}
                    >
                      関係・市場
                    </Button>
                    <Button
                      variant="outlined"
                      size="small"
                      component={Link}
                      href={`/admin/companies/${company.id}/edit`}
                    >
                      技術スタック編集
                    </Button>
                    {company.data_status !== 'published' ? (
                      <>
                        <Button
                          variant="contained"
                          color="success"
                          size="small"
                          onClick={() => handlePublish(company.id)}
                        >
                          承認
                        </Button>
                        <Button
                          variant="outlined"
                          color="error"
                          size="small"
                          onClick={() => handleReject(company.id)}
                        >
                          却下
                        </Button>
                      </>
                    ) : (
                      <Chip size="small" color="success" label="公開中（情報取得可）" />
                    )}
                  </Stack>
                </Stack>
              </AdminListCard>
            ))}
          </Stack>

          {pageCount > 1 && (
            <Box sx={{ display: 'flex', justifyContent: 'center', mt: 3 }}>
              <Pagination
                count={pageCount}
                page={page}
                onChange={handlePageChange}
                color="primary"
              />
            </Box>
          )}
        </CardContent>
      </Card>
    </PageContainer>
  )
}
