'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  InputAdornment,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
  Pagination,
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import SearchIcon from '@mui/icons-material/Search'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { StatusBadge } from '@/components/admin/StatusBadge'
import { fetchCompanyPrimary, formatFetchPrimarySummary } from '@/lib/admin-company-fetch'

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
  tech_stack?: string
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
  const [searchInput, setSearchInput] = useState('')
  const [searchName, setSearchName] = useState('')
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

  const fetchCompanies = useCallback(async (p: number, name: string, status: 'all' | 'draft' | 'published') => {
    setError('')
    const offset = (p - 1) * PAGE_SIZE
    const params = new URLSearchParams({
      limit: String(PAGE_SIZE),
      offset: String(offset),
    })
    const trimmed = name.trim()
    if (trimmed) params.set('name', trimmed)
    if (status !== 'all') params.set('status', status)
    const res = await fetch(`/api/admin/companies?${params}`, {
      headers: authService.getAdminFetchHeaders(),
      cache: 'no-store',
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '企業一覧の取得に失敗しました')
      return
    }
    setCompanies(data?.companies || [])
    setTotal(data?.total ?? 0)
  }, [])

  // 検索入力をデバウンスして反映
  useEffect(() => {
    const timer = setTimeout(() => {
      const next = searchInput.trim()
      setSearchName((prev) => {
        if (prev !== next) {
          setPage(1)
        }
        return next
      })
    }, 350)
    return () => clearTimeout(timer)
  }, [searchInput])

  useEffect(() => {
    void fetchCompanies(page, searchName, filterStatus)
  }, [fetchCompanies, page, searchName, filterStatus])

  useEffect(() => {
    void fetchCoverage()
  }, [fetchCoverage])

  const handlePageChange = (_: React.ChangeEvent<unknown>, value: number) => {
    setPage(value)
  }

  const reloadCurrentList = async () => {
    await fetchCompanies(page, searchName, filterStatus)
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
      await reloadCurrentList()
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
          `一括取得完了: ${data.processed ?? 0} 社` +
            `（基本 ${data.info_ok ?? 0} / 技術 ${data.tech_ok ?? 0} / 関係 ${data.relations_ok ?? 0} / 求人 ${data.jobs_ok ?? 0}）` +
            ` / エラー ${data.errors ?? 0}`,
        )
        if ((data.errors ?? 0) > 0) {
          setError(`一部の企業で取得に失敗しました（エラー ${data.errors} 件）。`)
        }
        await reloadCurrentList()
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
    await reloadCurrentList()
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
    await reloadCurrentList()
  }

  const summarizeFetchAll = (data: Record<string, unknown>) => {
    return formatFetchPrimarySummary(data as Parameters<typeof formatFetchPrimarySummary>[0])
  }

  const handleFetchAll = async (companyId: number, force = false) => {
    setError('')
    setFetchAllMessage('')
    setFetchAllId(companyId)
    try {
      const { ok, status, data } = await fetchCompanyPrimary(
        companyId,
        authService.getAdminFetchHeaders(),
        force,
      )
      if (!ok) {
        setError(data?.error || `企業情報の取得に失敗しました (${status})`)
        return
      }
      const summary = summarizeFetchAll(data as Record<string, unknown>)
      if (data.ok === false && Array.isArray(data.errors) && data.errors.length > 0) {
        setError(`一部失敗: ${data.errors.join('; ')}`)
      }
      setFetchAllMessage(summary ? `主3種取得完了: ${summary}` : '主3種取得完了')
      await reloadCurrentList()
      await fetchCoverage()
    } finally {
      setFetchAllId(null)
    }
  }

  const missingLabels = (c: Company) => {
    const labels: string[] = []
    if (!c.info_fetched_at || !c.description || !c.website_url) labels.push('基本')
    if (!c.tech_fetched_at || !c.tech_stack || c.tech_stack === '[]') labels.push('技術')
    if (!c.relations_fetched_at) labels.push('関係')
    if (!c.jobs_fetched_at) labels.push('求人')
    return labels
  }

  const pageCount = Math.ceil(total / PAGE_SIZE)
  const busy = warming || missingBatchLoading || fetchAllId !== null

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="企業管理"
        description={
          searchName
            ? `「${searchName}」の検索結果 ${total.toLocaleString()} 件`
            : `公開ステータスと企業情報（基本・技術・ビジネス関係）を管理します。全 ${total.toLocaleString()} 件`
        }
        backHref="/admin"
        actions={
          <>
            <Button component={Link} href="/admin/job-positions" size="small" color="inherit">
              求人
            </Button>
            <Button component={Link} href="/admin/graduate-employments" size="small" color="inherit">
              就職情報
            </Button>
            <Button variant="contained" component={Link} href="/admin/companies/new" disableElevation>
              企業を追加
            </Button>
          </>
        }
      />

      <ErrorAlert error={error} />
      {fetchAllMessage && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setFetchAllMessage('')}>
          {fetchAllMessage}
        </Alert>
      )}

      <Accordion
        disableGutters
        elevation={0}
        sx={{
          mb: 2,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px !important',
          '&:before': { display: 'none' },
          overflow: 'hidden',
        }}
      >
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Stack spacing={0.25}>
            <Typography fontWeight={600}>運用ツール</Typography>
            <Typography variant="caption" color="text.secondary">
              不足データの一括取得・L1カタログ温存
              {coverage
                ? `（公開 ${coverage.published_total} / Info ${pct(coverage.info_rate)} / Profile ${pct(coverage.profile_rate)}）`
                : ''}
            </Typography>
          </Stack>
        </AccordionSummary>
        <AccordionDetails sx={{ pt: 0, borderTop: '1px solid', borderColor: 'divider' }}>
          <Stack spacing={2.5} sx={{ py: 1.5 }}>
            <Stack
              direction={{ xs: 'column', md: 'row' }}
              spacing={2}
              alignItems={{ md: 'center' }}
              justifyContent="space-between"
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography variant="subtitle2" fontWeight={600}>
                  不足データを一括取得
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  未取得／TTL切れの基本・技術・ビジネス関係（＋求人）を最大20社まで埋めます。
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} flexShrink={0}>
                <Button variant="outlined" size="small" disabled={busy} onClick={() => handleFetchMissingBatch(true)}>
                  対象確認
                </Button>
                <Button
                  variant="contained"
                  size="small"
                  color="secondary"
                  disabled={busy}
                  onClick={() => handleFetchMissingBatch(false)}
                  startIcon={missingBatchLoading ? <CircularProgress size={14} color="inherit" /> : null}
                  disableElevation
                >
                  {missingBatchLoading ? '取得中…' : '一括取得'}
                </Button>
              </Stack>
            </Stack>

            <Box sx={{ borderTop: '1px solid', borderColor: 'divider', pt: 2 }}>
              <Stack
                direction={{ xs: 'column', md: 'row' }}
                spacing={2}
                alignItems={{ md: 'flex-start' }}
                justifyContent="space-between"
              >
                <Box sx={{ minWidth: 0 }}>
                  <Typography variant="subtitle2" fontWeight={600}>
                    L1カタログ充足
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                    マッチング用。公開企業の基本情報と WeightProfile を温存します。
                  </Typography>
                  {coverage && (
                    <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
                      <Chip size="small" variant="outlined" label={`公開 ${coverage.published_total}`} />
                      <Chip
                        size="small"
                        variant="outlined"
                        color={coverage.info_rate < (coverage.info_target ?? 0.8) ? 'error' : 'default'}
                        label={`Info ${pct(coverage.info_rate)}`}
                      />
                      <Chip
                        size="small"
                        variant="outlined"
                        color={coverage.profile_rate < (coverage.profile_target ?? 0.95) ? 'error' : 'default'}
                        label={`Profile ${pct(coverage.profile_rate)}`}
                      />
                      <Chip size="small" variant="outlined" label={`要温存 ${coverage.needs_warm}`} />
                    </Stack>
                  )}
                  {coverage?.alerts?.length ? (
                    <Typography variant="caption" color="error" sx={{ display: 'block', mt: 1 }}>
                      {coverage.alerts.join(' / ')}
                    </Typography>
                  ) : null}
                  {warmMessage && (
                    <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
                      {warmMessage}
                    </Typography>
                  )}
                </Box>
                <Stack direction="row" spacing={1} flexShrink={0} flexWrap="wrap" useFlexGap>
                  <Button variant="text" size="small" disabled={busy} onClick={() => handleSeedL1()}>
                    シード投入
                  </Button>
                  <Button variant="outlined" size="small" disabled={busy} onClick={() => handleWarmL1(true)}>
                    ドライラン
                  </Button>
                  <Button
                    variant="contained"
                    size="small"
                    disabled={busy}
                    onClick={() => handleWarmL1(false)}
                    startIcon={warming ? <CircularProgress size={14} color="inherit" /> : undefined}
                    disableElevation
                  >
                    L1温存
                  </Button>
                </Stack>
              </Stack>
            </Box>
          </Stack>
        </AccordionDetails>
      </Accordion>

      <AdminPanel
        title="企業一覧"
        headerRight={
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
            <TextField
              size="small"
              placeholder="企業名で検索"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              sx={{ minWidth: { sm: 240 } }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" color="action" />
                  </InputAdornment>
                ),
              }}
            />
            <ToggleButtonGroup
              exclusive
              size="small"
              value={filterStatus}
              onChange={(_, v) => {
                if (!v) return
                setFilterStatus(v)
                setPage(1)
              }}
              sx={{
                '& .MuiToggleButton-root': {
                  px: 1.5,
                  textTransform: 'none',
                  borderColor: 'divider',
                },
              }}
            >
              <ToggleButton value="all">すべて</ToggleButton>
              <ToggleButton value="draft">下書き</ToggleButton>
              <ToggleButton value="published">公開</ToggleButton>
            </ToggleButtonGroup>
          </Stack>
        }
      >
        <Stack divider={<Box sx={{ borderBottom: '1px solid', borderColor: 'divider' }} />}>
          {companies.length === 0 && (
            <Box sx={{ px: 2.5, py: 6, textAlign: 'center' }}>
              <Typography color="text.secondary">
                {searchName ? `「${searchName}」に一致する企業がありません` : '該当する企業がありません'}
              </Typography>
            </Box>
          )}

          {companies.map((company) => {
            const missing = missingLabels(company)
            const fetching = fetchAllId === company.id
            return (
              <Box
                key={company.id}
                sx={{
                  px: 2.5,
                  py: 2,
                  '&:hover': { bgcolor: 'action.hover' },
                }}
              >
                <Stack
                  direction={{ xs: 'column', md: 'row' }}
                  spacing={2}
                  alignItems={{ md: 'center' }}
                  justifyContent="space-between"
                >
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap sx={{ mb: 0.5 }}>
                      <Typography
                        component={Link}
                        href={`/admin/companies/${company.id}/info`}
                        variant="subtitle1"
                        fontWeight={700}
                        sx={{
                          color: 'text.primary',
                          textDecoration: 'none',
                          '&:hover': { color: 'primary.main' },
                        }}
                      >
                        {company.name}
                      </Typography>
                      <StatusBadge status={company.data_status} />
                      <Typography variant="caption" color="text.secondary">
                        {sourceLabel(company.source_type)}
                        {company.is_provisional ? ' · 暫定' : ''}
                      </Typography>
                    </Stack>
                    <Typography variant="body2" color="text.secondary">
                      {[company.industry || '業種未設定', company.location || '所在地未設定'].join('  ·  ')}
                    </Typography>
                    {missing.length > 0 && (
                      <Typography variant="caption" sx={{ color: 'warning.dark', display: 'block', mt: 0.75 }}>
                        未取得: {missing.join(' · ')}
                      </Typography>
                    )}
                  </Box>

                  <Stack
                    direction="row"
                    spacing={0.75}
                    flexWrap="wrap"
                    useFlexGap
                    alignItems="center"
                    justifyContent={{ xs: 'flex-start', md: 'flex-end' }}
                    sx={{ flexShrink: 0 }}
                  >
                    <Button
                      variant="contained"
                      size="small"
                      color="secondary"
                      onClick={() => handleFetchAll(company.id)}
                      disabled={busy}
                      startIcon={fetching ? <CircularProgress size={14} color="inherit" /> : null}
                      disableElevation
                    >
                      {fetching ? '取得中…' : '主3種取得'}
                    </Button>
                    <Button
                      variant="text"
                      size="small"
                      color="inherit"
                      onClick={() => handleFetchAll(company.id, true)}
                      disabled={busy}
                    >
                      強制再取得
                    </Button>
                    <Button component={Link} href={`/admin/companies/${company.id}/info`} size="small" variant="text">
                      編集
                    </Button>
                    {company.data_status !== 'published' ? (
                      <>
                        <Button
                          variant="outlined"
                          color="success"
                          size="small"
                          onClick={() => handlePublish(company.id)}
                          disabled={busy}
                        >
                          承認
                        </Button>
                        <Button
                          variant="text"
                          color="error"
                          size="small"
                          onClick={() => handleReject(company.id)}
                          disabled={busy}
                        >
                          却下
                        </Button>
                      </>
                    ) : null}
                  </Stack>
                </Stack>
              </Box>
            )
          })}
        </Stack>

        {pageCount > 1 && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 2.5, borderTop: '1px solid', borderColor: 'divider' }}>
            <Pagination count={pageCount} page={page} onChange={handlePageChange} color="primary" />
          </Box>
        )}
      </AdminPanel>
    </PageContainer>
  )
}
