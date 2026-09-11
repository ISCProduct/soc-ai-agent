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
  Checkbox,
  Chip,
  CircularProgress,
  LinearProgress,
  FormControl,
  FormControlLabel,
  IconButton,
  InputAdornment,
  InputLabel,
  Menu,
  MenuItem,
  Select,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
  Pagination,
  ListItemIcon,
  ListItemText,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import MoreVertIcon from '@mui/icons-material/MoreVert'
import RefreshIcon from '@mui/icons-material/Refresh'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { companyAspectHref, type CompanyAspect } from '@/components/admin/CompanyAspectTabs'
import { fetchCompanyPrimary, formatFetchPrimarySummary, formatFetchPrimaryEmptyAspects, hasActionableSoftEmpty } from '@/lib/admin-company-fetch'
import {
  applyBatchWave,
  batchItemFailuresFromResponse,
  batchProgressPercent,
  batchWaveFromResponse,
  candidateCountFromResponse,
  emptyBatchProgress,
  formatBatchFailureDetail,
  formatBatchProgressLabel,
  shouldContinueBatch,
  type BatchItemFailure,
  type BatchProgress,
} from '@/lib/admin-company-batch-progress'
import { resolveIndustryFieldProfile } from '@/lib/admin-company-field-profile'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'

const PAGE_SIZE = 50
/** 企業間並列。I/O待ちなので 8 まで。それ以上は OpenAI RPM が先に壊れる。 */
const GLOBAL_BATCH_CONCURRENCY = 8
/** 1リクエスト内でワーカープールを回す。6社区切りだと早い社が次を待てない。 */
const GLOBAL_BATCH_WAVE_LIMIT = 24
const GLOBAL_BATCH_PREVIEW_LIMIT = 50
const GLOBAL_BATCH_MAX_ROUNDS = 20
const WARM_L1_WAVE_LIMIT = 30
/** Backend の業界未設定フィルタ用センチネル */
const INDUSTRY_UNSET = '__unset__'

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

type FilterStatus = 'all' | 'draft' | 'published'
type FilterReadiness = 'all' | 'ready' | 'missing'

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

type AspectKey = 'info' | 'tech' | 'relations' | 'jobs'

const ASPECT_LABEL: Record<AspectKey, string> = {
  info: '会社概要',
  tech: '技術情報',
  relations: '関連企業',
  jobs: '求人',
}

const ASPECT_PAGE: Partial<Record<AspectKey, CompanyAspect>> = {
  info: 'info',
  tech: 'tech',
  relations: 'relations',
}

function missingAspects(c: Company): AspectKey[] {
  const profile = resolveIndustryFieldProfile(c.industry)
  const missing: AspectKey[] = []
  if (!c.info_fetched_at || !c.description || !c.website_url) missing.push('info')
  const tech = (c.tech_stack ?? '').trim()
  if (
    profile.requireTechForPublish &&
    (!c.tech_fetched_at || !tech || tech === '[]' || tech === 'null' || tech === '{}')
  ) {
    missing.push('tech')
  }
  if (!c.relations_fetched_at) missing.push('relations')
  if (!c.jobs_fetched_at) missing.push('jobs')
  return missing
}

/** 公開前かつ主情報がそろっている企業だけ一括公開の対象にする。 */
function canSelectForPublish(c: Company): boolean {
  if (c.data_status === 'published') return false
  return missingAspects(c).filter((k) => k !== 'jobs').length === 0
}

function aspectLabel(key: AspectKey, industry?: string): string {
  if (key === 'tech') {
    return resolveIndustryFieldProfile(industry).techAspectLabel
  }
  return ASPECT_LABEL[key]
}

function subtitleLine(c: Company): string {
  const parts = [c.industry, c.location].filter((v) => Boolean(v && v.trim()))
  return parts.join('  ·  ')
}

function statusLabel(status?: string): { label: string; color: 'warning' | 'success' | 'error' | 'default' } {
  if (status === 'published') return { label: '学生に公開中', color: 'success' }
  if (status === 'rejected') return { label: '非公開', color: 'error' }
  return { label: '公開前', color: 'warning' }
}

function pct(rate: number) {
  return `${Math.round((rate || 0) * 100)}%`
}

function industryGroupLabel(industry?: string): string {
  const trimmed = industry?.trim()
  return trimmed ? trimmed : '業界未設定'
}

function groupCompaniesByIndustry(companies: Company[]): { key: string; label: string; items: Company[] }[] {
  const groups: { key: string; label: string; items: Company[] }[] = []
  const indexByKey = new Map<string, number>()
  for (const company of companies) {
    const key = company.industry?.trim() || INDUSTRY_UNSET
    const existing = indexByKey.get(key)
    if (existing === undefined) {
      indexByKey.set(key, groups.length)
      groups.push({ key, label: industryGroupLabel(company.industry), items: [company] })
    } else {
      groups[existing].items.push(company)
    }
  }
  return groups
}

export default function PageContent() {
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
  const [filterStatus, setFilterStatus] = useState<FilterStatus>('all')
  const [filterIndustry, setFilterIndustry] = useState('')
  const [filterReadiness, setFilterReadiness] = useState<FilterReadiness>('all')
  const [groupByIndustry, setGroupByIndustry] = useState(false)
  const [industries, setIndustries] = useState<string[]>([])
  const [searchInput, setSearchInput] = useState('')
  const [searchName, setSearchName] = useState('')
  const [coverage, setCoverage] = useState<L1Coverage | null>(null)
  const [warming, setWarming] = useState(false)
  const [warmMessage, setWarmMessage] = useState('')
  const [fetchPrimaryId, setFetchPrimaryId] = useState<number | null>(null)
  const [fetchMessage, setFetchMessage] = useState('')
  const [fetchSeverity, setFetchSeverity] = useState<'success' | 'warning'>('success')
  const [missingBatchLoading, setMissingBatchLoading] = useState(false)
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null)
  const [menuAnchor, setMenuAnchor] = useState<{ el: HTMLElement; company: Company } | null>(null)
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [bulkPublishing, setBulkPublishing] = useState(false)
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [approvedCompanyIds, setApprovedCompanyIds] = useState<Set<number>>(new Set())
  const [approvalBusyId, setApprovalBusyId] = useState<number | null>(null)

  const fetchCoverage = useCallback(async () => {
    const res = await fetch('/api/admin/companies/l1-coverage', {
      headers: authService.getAdminFetchHeaders(),
      cache: 'no-store',
    })
    if (!res.ok) return
    const data = await res.json()
    setCoverage(data)
  }, [])

  const fetchIndustries = useCallback(async () => {
    const res = await fetch('/api/admin/companies/industries', {
      headers: authService.getAdminFetchHeaders(),
      cache: 'no-store',
    })
    if (!res.ok) return
    const data = await res.json()
    setIndustries(Array.isArray(data?.industries) ? data.industries : [])
  }, [])

  const fetchCompanies = useCallback(
    async (
      p: number,
      name: string,
      status: FilterStatus,
      industry: string,
      readiness: FilterReadiness,
      groupIndustry: boolean,
      schoolFilter?: number,
    ) => {
      setError('')
      const offset = (p - 1) * PAGE_SIZE
      const params = new URLSearchParams({
        limit: String(PAGE_SIZE),
        offset: String(offset),
      })
      const trimmed = name.trim()
      if (trimmed) params.set('name', trimmed)
      if (status !== 'all') params.set('status', status)
      if (industry) params.set('industry', industry)
      if (readiness !== 'all') params.set('readiness', readiness)
      if (groupIndustry) params.set('order', 'industry')
      // 企業カタログは共有のため一覧自体は絞り込まない。schoolFilterは承認トグルの対象校選択にのみ使う。
      void schoolFilter
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
    },
    [],
  )

  useEffect(() => {
    const timer = setTimeout(() => {
      const next = searchInput.trim()
      setSearchName((prev) => {
        if (prev !== next) setPage(1)
        return next
      })
    }, 350)
    return () => clearTimeout(timer)
  }, [searchInput])

  useEffect(() => {
    void fetchCompanies(page, searchName, filterStatus, filterIndustry, filterReadiness, groupByIndustry, schoolId)
  }, [fetchCompanies, page, searchName, filterStatus, filterIndustry, filterReadiness, groupByIndustry, schoolId])

  useEffect(() => {
    setSelectedIds([])
  }, [page, searchName, filterStatus, filterIndustry, filterReadiness, groupByIndustry, schoolId])

  useEffect(() => {
    // 取得後に情報が足りなくなった選択は外す（中身が同じなら state を更新しない）
    setSelectedIds((prev) => {
      const next = prev.filter((id) => {
        const company = companies.find((c) => c.id === id)
        return company ? canSelectForPublish(company) : false
      })
      if (next.length === prev.length && next.every((id, i) => id === prev[i])) return prev
      return next
    })
  }, [companies])

  useEffect(() => {
    void fetchCoverage()
    void fetchIndustries()
  }, [fetchCoverage, fetchIndustries])

  useEffect(() => {
    if (schoolId === undefined) {
      setApprovedCompanyIds(new Set())
      return
    }
    let cancelled = false
    fetch(`/api/admin/schools/${schoolId}/company-approvals`, {
      headers: authService.getAdminFetchHeaders(),
      cache: 'no-store',
    })
      .then((r) => r.json())
      .then((data) => {
        if (!cancelled) setApprovedCompanyIds(new Set(data?.company_ids || []))
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [schoolId])

  const toggleCompanyApproval = async (companyId: number, approved: boolean) => {
    if (schoolId === undefined) return
    setApprovalBusyId(companyId)
    try {
      const res = await fetch(`/api/admin/schools/${schoolId}/company-approvals${approved ? `/${companyId}` : ''}`, {
        method: approved ? 'DELETE' : 'POST',
        headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
        body: approved ? undefined : JSON.stringify({ company_id: companyId }),
      })
      if (!res.ok) return
      setApprovedCompanyIds((prev) => {
        const next = new Set(prev)
        if (approved) next.delete(companyId)
        else next.add(companyId)
        return next
      })
    } finally {
      setApprovalBusyId(null)
    }
  }

  const handlePageChange = (_: React.ChangeEvent<unknown>, value: number) => {
    setPage(value)
  }

  const reloadCurrentList = async () => {
    await fetchCompanies(page, searchName, filterStatus, filterIndustry, filterReadiness, groupByIndustry, schoolId)
    await fetchIndustries()
  }

  const resetFilters = () => {
    setSearchInput('')
    setSearchName('')
    setFilterStatus('all')
    setFilterIndustry('')
    setFilterReadiness('all')
    setGroupByIndustry(false)
    setPage(1)
  }

  const hasActiveFilters =
    Boolean(searchName) ||
    filterStatus !== 'all' ||
    Boolean(filterIndustry) ||
    filterReadiness !== 'all' ||
    groupByIndustry

  const companyGroups = groupByIndustry
    ? groupCompaniesByIndustry(companies)
    : [{ key: 'all', label: '', items: companies }]

  const filterSummaryParts: string[] = []
  if (searchName) filterSummaryParts.push(`「${searchName}」`)
  if (filterStatus === 'draft') filterSummaryParts.push('公開前')
  if (filterStatus === 'published') filterSummaryParts.push('学生に公開中')
  if (filterIndustry === INDUSTRY_UNSET) filterSummaryParts.push('業界未設定')
  else if (filterIndustry) filterSummaryParts.push(filterIndustry)
  if (filterReadiness === 'ready') filterSummaryParts.push('情報がそろっている')
  if (filterReadiness === 'missing') filterSummaryParts.push('情報が足りない')
  if (groupByIndustry) filterSummaryParts.push('業界別に表示')
  const filterSummary = filterSummaryParts.join(' / ')

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
        setError(data?.error || data?.message || 'マッチング用データの準備に失敗しました')
        return
      }
      setWarmMessage(
        `準備完了: 新規 ${data.created ?? 0} 社 / 更新 ${data.updated ?? 0} 社 / 変更なし ${data.skipped ?? 0} 社`,
      )
      await fetchCoverage()
      await reloadCurrentList()
    } finally {
      setWarming(false)
    }
  }

  const postCompanyBatch = async (
    path: '/api/admin/companies/fetch-missing-batch' | '/api/admin/companies/warm-l1',
    body: Record<string, unknown>,
  ): Promise<{ ok: boolean; status: number; data: Record<string, unknown> }> => {
    const res = await fetch(path, {
      method: 'POST',
      headers: {
        ...authService.getAdminFetchHeaders(),
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })
    const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
    return { ok: res.ok, status: res.status, data }
  }

  const handleWarmL1 = async (dryRun: boolean) => {
    setWarming(true)
    setWarmMessage('')
    setBatchProgress(null)
    setError('')
    try {
      if (dryRun) {
        const { ok, data } = await postCompanyBatch('/api/admin/companies/warm-l1', {
          limit: 100,
          dry_run: true,
          include_info: true,
          include_persona: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || 'マッチング用データの更新に失敗しました'))
          return
        }
        setWarmMessage(`確認のみ: 対象は約 ${candidateCountFromResponse(data)} 社です（一度に最大 ${data.limit ?? 100} 社）`)
        if (data.coverage && typeof data.coverage === 'object') {
          setCoverage(data.coverage as L1Coverage)
        }
        return
      }

      const preview = await postCompanyBatch('/api/admin/companies/warm-l1', {
        limit: 100,
        dry_run: true,
        include_info: true,
        include_persona: true,
      })
      let progress = emptyBatchProgress(
        Math.max(candidateCountFromResponse(preview.data), coverage?.needs_warm ?? 0),
      )
      setBatchProgress(progress)
      setWarmMessage(formatBatchProgressLabel(progress, true))

      for (;;) {
        const { ok, data } = await postCompanyBatch('/api/admin/companies/warm-l1', {
          limit: WARM_L1_WAVE_LIMIT,
          dry_run: false,
          include_info: true,
          include_persona: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || 'マッチング用データの更新に失敗しました'))
          break
        }
        const wave = {
          ...batchWaveFromResponse(data),
          persona_ok: Number(data.persona_ok ?? 0),
        }
        progress = applyBatchWave(progress, wave, WARM_L1_WAVE_LIMIT)
        setBatchProgress(progress)
        setWarmMessage(formatBatchProgressLabel(progress, true))
        if (data.coverage && typeof data.coverage === 'object') {
          setCoverage(data.coverage as L1Coverage)
        }
        if (!shouldContinueBatch(wave, WARM_L1_WAVE_LIMIT, progress.rounds, GLOBAL_BATCH_MAX_ROUNDS)) {
          break
        }
      }
      setWarmMessage(formatBatchProgressLabel(progress, false))
      await fetchCoverage()
    } finally {
      setWarming(false)
    }
  }

  const handleFetchMissingBatch = async (dryRun: boolean) => {
    setMissingBatchLoading(true)
    setFetchMessage('')
    setFetchSeverity('success')
    setBatchProgress(null)
    setError('')
    try {
      if (dryRun) {
        const { ok, status, data } = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
          limit: GLOBAL_BATCH_PREVIEW_LIMIT,
          dry_run: true,
          primary_only: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || `まとめて取得に失敗しました (${status})`))
          return
        }
        const names = Array.isArray(data.items)
          ? (data.items as { name?: string }[])
              .slice(0, 5)
              .map((i) => i.name)
              .filter(Boolean)
              .join('、')
          : ''
        const n = candidateCountFromResponse(data)
        if (n === 0) {
          setFetchSeverity('warning')
          setFetchMessage('いま取得対象になる企業はありません（会社概要・技術情報・関連企業がすでにそろっているか、対象候補が見つかりませんでした）。')
        } else {
          setFetchMessage(
            `情報が足りない企業は ${n}${n >= GLOBAL_BATCH_PREVIEW_LIMIT ? ' 社以上' : ' 社'}です` +
              (names ? `（例: ${names}${n > 5 ? ' など' : ''}）` : '') +
              '。取得を始めると件数の進捗が表示されます。',
          )
        }
        return
      }

      const preview = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
        limit: GLOBAL_BATCH_PREVIEW_LIMIT,
        dry_run: true,
        primary_only: true,
        concurrency: GLOBAL_BATCH_CONCURRENCY,
      })
      let progress = emptyBatchProgress(candidateCountFromResponse(preview.data))
      setBatchProgress(progress)
      setFetchMessage(formatBatchProgressLabel(progress, true))
      const failures: BatchItemFailure[] = []

      for (;;) {
        const { ok, status, data } = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
          limit: GLOBAL_BATCH_WAVE_LIMIT,
          dry_run: false,
          primary_only: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || `まとめて取得に失敗しました (${status})`))
          break
        }
        const wave = batchWaveFromResponse(data)
        progress = applyBatchWave(progress, wave, GLOBAL_BATCH_WAVE_LIMIT)
        failures.push(...batchItemFailuresFromResponse(data))
        setBatchProgress(progress)
        setFetchSeverity(progress.errors > 0 ? 'warning' : 'success')
        setFetchMessage(formatBatchProgressLabel(progress, true))
        if (!shouldContinueBatch(wave, GLOBAL_BATCH_WAVE_LIMIT, progress.rounds, GLOBAL_BATCH_MAX_ROUNDS)) {
          break
        }
      }

      if (progress.processed === 0) {
        setFetchSeverity('warning')
        setFetchMessage('取得対象の企業が見つかりませんでした。一覧で「まだ足りない情報があります」と出ている企業がある場合は、時間をおいて再度お試しください。')
      } else {
        setFetchSeverity(progress.errors > 0 ? 'warning' : 'success')
        setFetchMessage(formatBatchProgressLabel(progress, false) + '。内容を確認してから公開してください。')
        if (progress.errors > 0) {
          setError(
            formatBatchFailureDetail(failures) ||
              `${progress.errors} 社で取得に失敗しました。時間をおいて再度お試しください。`,
          )
        }
      }
      await reloadCurrentList()
      await fetchCoverage()
    } finally {
      setMissingBatchLoading(false)
    }
  }

  const publishCompany = async (companyId: number): Promise<boolean> => {
    const res = await fetch(`/api/admin/companies/${companyId}/publish`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    return res.ok
  }

  const handlePublish = async (companyId: number) => {
    setError('')
    setMenuAnchor(null)
    const ok = await publishCompany(companyId)
    if (!ok) {
      setError('公開に失敗しました')
      return
    }
    setFetchSeverity('success')
    setFetchMessage('企業を学生に公開しました。')
    setSelectedIds((prev) => prev.filter((id) => id !== companyId))
    await reloadCurrentList()
  }

  const handleBulkPublish = async () => {
    const targets = companies.filter((c) => selectedIds.includes(c.id) && canSelectForPublish(c))
    if (targets.length === 0) {
      setFetchSeverity('warning')
      setFetchMessage('公開できる企業（情報がそろった公開前）が選択されていません。')
      return
    }

    const ok = window.confirm(`選択した ${targets.length} 社を学生に公開しますか？`)
    if (!ok) return

    setBulkPublishing(true)
    setError('')
    setFetchMessage('')
    try {
      let success = 0
      let failed = 0
      for (const company of targets) {
        if (await publishCompany(company.id)) success += 1
        else failed += 1
      }
      setSelectedIds([])
      if (failed > 0) {
        setFetchSeverity('warning')
        setFetchMessage(`${success} 社を公開しました。${failed} 社は公開できませんでした。`)
        setError(`${failed} 社の公開に失敗しました。`)
      } else {
        setFetchSeverity('success')
        setFetchMessage(`${success} 社を学生に公開しました。`)
      }
      await reloadCurrentList()
    } finally {
      setBulkPublishing(false)
    }
  }

  const toggleSelect = (companyId: number) => {
    const company = companies.find((c) => c.id === companyId)
    if (!company || !canSelectForPublish(company)) return
    setSelectedIds((prev) =>
      prev.includes(companyId) ? prev.filter((id) => id !== companyId) : [...prev, companyId],
    )
  }

  const selectableOnPage = companies.filter(canSelectForPublish)
  const selectedSelectableCount = selectableOnPage.filter((c) => selectedIds.includes(c.id)).length
  const allSelectableSelected =
    selectableOnPage.length > 0 && selectedSelectableCount === selectableOnPage.length

  const toggleSelectAllSelectable = () => {
    if (allSelectableSelected) {
      setSelectedIds((prev) => prev.filter((id) => !selectableOnPage.some((c) => c.id === id)))
      return
    }
    setSelectedIds((prev) => {
      const next = new Set(prev)
      for (const c of selectableOnPage) next.add(c.id)
      return Array.from(next)
    })
  }

  const handleReject = async (companyId: number) => {
    setError('')
    setMenuAnchor(null)
    const res = await fetch(`/api/admin/companies/${companyId}/reject`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(data?.error || `非公開への変更に失敗しました (${res.status})`)
      return
    }
    setFetchSeverity('success')
    setFetchMessage('企業を非公開にしました。学生には表示されません。')
    await reloadCurrentList()
  }

  const handleFetchPrimary = async (companyId: number, force = false) => {
    setError('')
    setFetchMessage('')
    setFetchSeverity('success')
    setFetchPrimaryId(companyId)
    setMenuAnchor(null)
    const knownIndustry = companies.find((c) => c.id === companyId)?.industry
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
      const steps = [data.info_step, data.tech_step, data.relations_step]
      const fetched = steps.filter((s) => s?.status === 'fetched').length
      const allSkipped = steps.every((s) => s?.status === 'skipped')
      const hardFailed = steps.some((s) => s?.status === 'error')
      const softEmpty = hasActionableSoftEmpty(data, knownIndustry)
      const budgetHit = steps.some((s) => s?.detail === 'budget') ||
        (Array.isArray(data.errors) && data.errors.some((e) => e.includes('budget')))
      const summary = formatFetchPrimarySummary(data, knownIndustry)

      if (budgetHit) {
        setFetchSeverity('warning')
        setError('月次の情報取得上限に達しているため、新しい取得ができませんでした。コスト画面を確認するか、時間をおいて再度お試しください。')
        setFetchMessage(summary || '予算超過のため取得をスキップしました。')
      } else if (hardFailed) {
        setFetchSeverity('warning')
        setError(
          Array.isArray(data.errors) && data.errors.length > 0
            ? `一部うまくいきませんでした: ${data.errors.join(' / ')}`
            : '一部の情報取得に失敗しました。',
        )
        setFetchMessage(
          `情報の取得が一部失敗しました（${summary}）。「最新の情報に更新」で再試行できます。`,
        )
      } else if (softEmpty) {
        const emptyAspects = formatFetchPrimaryEmptyAspects(data, knownIndustry)
        setFetchSeverity('warning')
        setFetchMessage(
          emptyAspects
            ? `${emptyAspects}は公開情報から特定できませんでした（${summary}）。手入力するか、時間をおいて再度お試しください。`
            : `取得を試しましたが、公開情報から特定できない項目がありました（${summary}）。手入力するか、時間をおいて再度お試しください。`,
        )
      } else if (allSkipped) {
        setFetchSeverity('warning')
        setFetchMessage(`すでに新しい情報があるか、業種により対象外のためスキップしました（${summary}）。`)
      } else {
        setFetchSeverity('success')
        setFetchMessage(
          force
            ? `情報を更新しました（${fetched} 項目）。${summary}`
            : `情報を取得しました（${fetched} 項目）。${summary} 内容を確認して問題なければ「学生に公開」できます。`,
        )
      }
      await reloadCurrentList()
      await fetchCoverage()
    } catch {
      setError('企業情報の取得中に通信エラーが発生しました。時間をおいて再度お試しください。')
    } finally {
      setFetchPrimaryId(null)
    }
  }

  const pageCount = Math.ceil(total / PAGE_SIZE)
  const busy = warming || missingBatchLoading || fetchPrimaryId !== null || bulkPublishing
  const selectedCount = selectedIds.length

  useEffect(() => {
    if (!busy) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    const onClick = (e: MouseEvent) => {
      const el = e.target
      if (!(el instanceof Element)) return
      if (!el.closest('a[href]')) return
      e.preventDefault()
      e.stopPropagation()
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    document.addEventListener('click', onClick, true)
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload)
      document.removeEventListener('click', onClick, true)
    }
  }, [busy])

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="企業情報の管理"
        description={
          filterSummary
            ? `${filterSummary} の結果 ${total.toLocaleString()} 件`
            : `学生課の先生が、学生に見せる企業情報を登録・確認・公開するための画面です。登録 ${total.toLocaleString()} 社`
        }
        backHref={busy ? undefined : '/admin'}
        actions={
          <Button
            variant="contained"
            component={Link}
            href="/admin/companies/new"
            disableElevation
            disabled={busy}
          >
            企業を追加
          </Button>
        }
      />

      <ErrorAlert error={error} />
      {fetchMessage && (
        <Alert severity={fetchSeverity} sx={{ mb: 2 }} onClose={() => setFetchMessage('')}>
          {fetchMessage}
        </Alert>
      )}

      <Box
        sx={{
          mb: 2,
          p: 2,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          bgcolor: 'grey.50',
        }}
      >
        <Typography fontWeight={700} sx={{ mb: 1 }}>
          使い方（3ステップ）
        </Typography>
        <Stack spacing={0.5}>
          <Typography variant="body2" color="text.secondary">
            1. 企業名で探し、情報が足りなければ「情報を取得」します
          </Typography>
          <Typography variant="body2" color="text.secondary">
            2. 「会社概要」「技術情報」「関連企業」で内容を確認・修正します
          </Typography>
          <Typography variant="body2" color="text.secondary">
            3. 情報がそろった企業を選んで「学生に公開」します（足りない企業は選べません）
          </Typography>
        </Stack>
      </Box>

      <Box
        sx={{
          mb: 2,
          p: 2.5,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          bgcolor: 'background.paper',
        }}
      >
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={2}
          alignItems={{ sm: 'center' }}
          justifyContent="space-between"
        >
          <Box sx={{ minWidth: 0 }}>
            <Typography fontWeight={700} sx={{ mb: 0.5 }}>
              足りない情報をまとめて取得
            </Typography>
            <Typography variant="body2" color="text.secondary">
              まだそろっていない会社概要・技術情報・関連企業を自動で集めます。
              処理した社数と内訳が進捗バーに出ます。足りなければ続きから再度実行できます。
              取得後は必ず内容を確認してから公開してください。
            </Typography>
            {missingBatchLoading && batchProgress && (
              <Box sx={{ mt: 1.5 }}>
                <LinearProgress
                  variant="determinate"
                  value={batchProgressPercent(batchProgress)}
                  sx={{ height: 8, borderRadius: 4 }}
                />
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.75 }}>
                  {formatBatchProgressLabel(batchProgress, true)}
                </Typography>
                <Typography variant="caption" color="warning.main" sx={{ display: 'block', mt: 0.25 }}>
                  取得中はページを更新・移動しないでください。途中結果の表示が消えます。
                </Typography>
              </Box>
            )}
          </Box>
          <Stack direction="row" spacing={1} flexShrink={0}>
            <Button variant="outlined" disabled={busy} onClick={() => handleFetchMissingBatch(true)}>
              対象を確認
            </Button>
            <Button
              variant="contained"
              color="secondary"
              disabled={busy}
              onClick={() => handleFetchMissingBatch(false)}
              startIcon={missingBatchLoading ? <CircularProgress size={16} color="inherit" /> : <RefreshIcon />}
              disableElevation
            >
              {missingBatchLoading ? `取得中 ${batchProgressPercent(batchProgress ?? emptyBatchProgress())}%` : 'まとめて取得'}
            </Button>
          </Stack>
        </Stack>
      </Box>

      <AdminPanel title="企業一覧">
        <Box
          sx={{
            px: 2.5,
            py: 1.75,
            borderBottom: '1px solid',
            borderColor: 'divider',
            bgcolor: 'grey.50',
          }}
        >
          <Stack spacing={1.5}>
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} alignItems={{ sm: 'center' }}>
              <TextField
                size="small"
                placeholder="企業名で探す"
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                sx={{ minWidth: { sm: 240 }, bgcolor: 'background.paper' }}
                InputProps={{
                  startAdornment: (
                    <InputAdornment position="start">
                      <SearchIcon fontSize="small" color="action" />
                    </InputAdornment>
                  ),
                }}
              />
              <FormControl size="small" sx={{ minWidth: { sm: 220 }, bgcolor: 'background.paper' }}>
                <InputLabel id="filter-industry-label">業界</InputLabel>
                <Select
                  labelId="filter-industry-label"
                  label="業界"
                  value={filterIndustry}
                  onChange={(e) => {
                    setFilterIndustry(e.target.value)
                    setPage(1)
                  }}
                >
                  <MenuItem value="">すべての業界</MenuItem>
                  <MenuItem value={INDUSTRY_UNSET}>業界未設定</MenuItem>
                  {industries.map((industry) => (
                    <MenuItem key={industry} value={industry}>
                      {industry}
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>
              <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(1) }} />
              {hasActiveFilters && (
                <Button size="small" variant="text" onClick={resetFilters} sx={{ alignSelf: { xs: 'flex-start', sm: 'center' } }}>
                  絞り込みを解除
                </Button>
              )}
            </Stack>
            <Stack
              direction={{ xs: 'column', md: 'row' }}
              spacing={1.5}
              alignItems={{ md: 'center' }}
              flexWrap="wrap"
              useFlexGap
            >
              <Box>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                  公開の状態
                </Typography>
                <ToggleButtonGroup
                  exclusive
                  size="small"
                  value={filterStatus}
                  onChange={(_, v: FilterStatus | null) => {
                    if (!v) return
                    setFilterStatus(v)
                    setPage(1)
                  }}
                  sx={{
                    bgcolor: 'background.paper',
                    '& .MuiToggleButton-root': {
                      px: 1.5,
                      textTransform: 'none',
                      borderColor: 'divider',
                    },
                  }}
                >
                  <ToggleButton value="all">すべて</ToggleButton>
                  <ToggleButton value="draft">公開前</ToggleButton>
                  <ToggleButton value="published">学生に公開中</ToggleButton>
                </ToggleButtonGroup>
              </Box>
              <Box>
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.5 }}>
                  情報のそろい具合
                </Typography>
                <ToggleButtonGroup
                  exclusive
                  size="small"
                  value={filterReadiness}
                  onChange={(_, v: FilterReadiness | null) => {
                    if (!v) return
                    setFilterReadiness(v)
                    setPage(1)
                  }}
                  sx={{
                    bgcolor: 'background.paper',
                    '& .MuiToggleButton-root': {
                      px: 1.5,
                      textTransform: 'none',
                      borderColor: 'divider',
                    },
                  }}
                >
                  <ToggleButton value="all">すべて</ToggleButton>
                  <ToggleButton value="missing">足りない</ToggleButton>
                  <ToggleButton value="ready">そろっている</ToggleButton>
                </ToggleButtonGroup>
              </Box>
              <FormControlLabel
                control={
                  <Checkbox
                    checked={groupByIndustry}
                    onChange={(e) => {
                      setGroupByIndustry(e.target.checked)
                      setPage(1)
                    }}
                    size="small"
                  />
                }
                label="業界別に分けて表示"
                sx={{ ml: 0, mr: 0, alignSelf: { xs: 'flex-start', md: 'flex-end' }, pb: { md: 0.25 } }}
              />
            </Stack>
          </Stack>
        </Box>
        {selectableOnPage.length > 0 ? (
          <Box
            sx={{
              px: 2.5,
              py: 1.25,
              borderBottom: '1px solid',
              borderColor: 'divider',
              bgcolor: selectedCount > 0 ? 'action.selected' : 'grey.50',
            }}
          >
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              spacing={1.5}
              alignItems={{ sm: 'center' }}
              justifyContent="space-between"
            >
              <FormControlLabel
                control={
                  <Checkbox
                    checked={allSelectableSelected}
                    indeterminate={selectedSelectableCount > 0 && !allSelectableSelected}
                    onChange={toggleSelectAllSelectable}
                    disabled={busy}
                  />
                }
                label={
                  selectedSelectableCount > 0
                    ? `公開できる企業 ${selectedSelectableCount} 社を選択中`
                    : `このページで公開できる企業（${selectableOnPage.length} 社）を選択`
                }
              />
              <Stack direction="row" spacing={1}>
                {selectedCount > 0 && (
                  <Button size="small" variant="text" disabled={busy} onClick={() => setSelectedIds([])}>
                    選択解除
                  </Button>
                )}
                <Button
                  size="small"
                  variant="contained"
                  color="success"
                  disabled={busy || selectedSelectableCount === 0}
                  onClick={() => handleBulkPublish()}
                  startIcon={bulkPublishing ? <CircularProgress size={14} color="inherit" /> : null}
                  disableElevation
                >
                  {bulkPublishing
                    ? '公開中…'
                    : selectedSelectableCount > 0
                      ? `選択した ${selectedSelectableCount} 社を学生に公開`
                      : '選択して学生に公開'}
                </Button>
              </Stack>
            </Stack>
          </Box>
        ) : (
          <Box
            sx={{
              px: 2.5,
              py: 1.25,
              borderBottom: '1px solid',
              borderColor: 'divider',
              bgcolor: 'grey.50',
            }}
          >
            <Typography variant="body2" color="text.secondary">
              このページに、まとめて公開できる企業はありません（公開前かつ情報がそろっている企業だけ選べます）。
            </Typography>
          </Box>
        )}

        <Stack divider={<Box sx={{ borderBottom: '1px solid', borderColor: 'divider' }} />}>
          {companies.length === 0 && (
            <Box sx={{ px: 2.5, py: 6, textAlign: 'center' }}>
              <Typography color="text.secondary" sx={{ mb: 1 }}>
                {hasActiveFilters
                  ? '条件に一致する企業がありません。絞り込みを変えてみてください。'
                  : '該当する企業がありません'}
              </Typography>
              {hasActiveFilters ? (
                <Button variant="outlined" size="small" onClick={resetFilters}>
                  絞り込みを解除
                </Button>
              ) : (
                <Button component={Link} href="/admin/companies/new" variant="outlined" size="small">
                  最初の企業を追加
                </Button>
              )}
            </Box>
          )}

          {companyGroups.map((group) => (
            <Box key={group.key}>
              {groupByIndustry && group.label ? (
                <Box
                  sx={{
                    px: 2.5,
                    py: 1,
                    bgcolor: 'grey.100',
                    borderBottom: '1px solid',
                    borderColor: 'divider',
                    position: 'sticky',
                    top: 0,
                    zIndex: 1,
                  }}
                >
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography variant="subtitle2" fontWeight={700}>
                      {group.label}
                    </Typography>
                    <Chip size="small" label={`${group.items.length} 社`} variant="outlined" />
                  </Stack>
                </Box>
              ) : null}

              <Stack divider={<Box sx={{ borderBottom: '1px solid', borderColor: 'divider' }} />}>
                {group.items.map((company) => {
                  const missing = missingAspects(company)
                  const primaryMissing = missing.filter((k) => k !== 'jobs')
                  const ready = primaryMissing.length === 0
                  const fetching = fetchPrimaryId === company.id
                  const isDraft = company.data_status !== 'published'
                  const industryLabel = company.industry?.trim() || ''
                  const locationLabel = company.location?.trim() || ''
                  const meta =
                    groupByIndustry || !industryLabel
                      ? subtitleLine(company)
                      : locationLabel
                  const status = statusLabel(company.data_status)
                  const selected = selectedIds.includes(company.id)

                  return (
                    <Box
                      key={company.id}
                      sx={{
                        px: 2.5,
                        py: 2,
                        bgcolor: selected ? 'action.selected' : undefined,
                        '&:hover': { bgcolor: selected ? 'action.selected' : 'action.hover' },
                      }}
                    >
                      <Stack
                        direction={{ xs: 'column', md: 'row' }}
                        spacing={1.5}
                        alignItems={{ md: 'center' }}
                        justifyContent="space-between"
                      >
                        <Stack direction="row" spacing={1} alignItems="flex-start" sx={{ minWidth: 0, flex: 1 }}>
                          <Checkbox
                            checked={selected}
                            onChange={() => toggleSelect(company.id)}
                            disabled={busy || !canSelectForPublish(company)}
                            inputProps={{ 'aria-label': `${company.name}を選択` }}
                            sx={{ mt: -0.5 }}
                          />
                          <Box sx={{ minWidth: 0, flex: 1 }}>
                            <Stack
                              direction="row"
                              spacing={1}
                              alignItems="center"
                              flexWrap="wrap"
                              useFlexGap
                              sx={{ mb: 0.5 }}
                            >
                              <Typography
                                component={Link}
                                href={companyAspectHref(company.id, 'info')}
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
                              <Chip label={status.label} color={status.color} size="small" />
                              {schoolId !== undefined ? (
                                <Chip
                                  label={approvedCompanyIds.has(company.id) ? '承認済み' : '未承認'}
                                  color={approvedCompanyIds.has(company.id) ? 'success' : 'default'}
                                  variant={approvedCompanyIds.has(company.id) ? 'filled' : 'outlined'}
                                  size="small"
                                  disabled={approvalBusyId === company.id}
                                  onClick={() => toggleCompanyApproval(company.id, approvedCompanyIds.has(company.id))}
                                  sx={{ cursor: 'pointer' }}
                                />
                              ) : null}
                              {!groupByIndustry && industryLabel ? (
                                <Chip
                                  size="small"
                                  label={industryLabel}
                                  variant="outlined"
                                  onClick={() => {
                                    setFilterIndustry(industryLabel)
                                    setPage(1)
                                  }}
                                  sx={{ cursor: 'pointer' }}
                                />
                              ) : null}
                            </Stack>

                            {meta ? (
                              <Typography variant="body2" color="text.secondary" sx={{ mb: 0.75 }}>
                                {meta}
                              </Typography>
                            ) : null}

                            <Stack
                              direction="row"
                              spacing={0.75}
                              alignItems="center"
                              flexWrap="wrap"
                              useFlexGap
                              sx={{ mb: 1 }}
                            >
                              {ready ? (
                                <Chip
                                  size="small"
                                  icon={<CheckCircleOutlineIcon />}
                                  label="公開の準備ができています"
                                  color="success"
                                  variant="outlined"
                                />
                              ) : (
                                <>
                                  <Chip
                                    size="small"
                                    icon={<ErrorOutlineIcon />}
                                    label="まだ足りない情報があります"
                                    color="warning"
                                    variant="outlined"
                                  />
                                  {primaryMissing.map((key) => {
                                    const page = ASPECT_PAGE[key]
                                    const label = aspectLabel(key, company.industry)
                                    if (!page) {
                                      return (
                                        <Chip key={key} size="small" label={label} variant="outlined" />
                                      )
                                    }
                                    return (
                                      <Chip
                                        key={key}
                                        size="small"
                                        label={label}
                                        variant="outlined"
                                        component={Link}
                                        href={companyAspectHref(company.id, page)}
                                        clickable
                                      />
                                    )
                                  })}
                                </>
                              )}
                            </Stack>

                            <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
                              <Button
                                component={Link}
                                href={companyAspectHref(company.id, 'info')}
                                size="small"
                                variant="text"
                              >
                                会社概要
                              </Button>
                              <Button
                                component={Link}
                                href={companyAspectHref(company.id, 'tech')}
                                size="small"
                                variant="text"
                              >
                                技術情報
                              </Button>
                              <Button
                                component={Link}
                                href={companyAspectHref(company.id, 'relations')}
                                size="small"
                                variant="text"
                              >
                                関連企業
                              </Button>
                            </Stack>
                          </Box>
                        </Stack>

                        <Stack
                          direction="row"
                          spacing={1}
                          alignItems="center"
                          justifyContent={{ xs: 'flex-start', md: 'flex-end' }}
                          sx={{ flexShrink: 0, pl: { xs: 5, md: 0 } }}
                        >
                          {!ready ? (
                            <Button
                              variant="contained"
                              size="small"
                              color="secondary"
                              onClick={() => handleFetchPrimary(company.id, false)}
                              disabled={busy}
                              startIcon={fetching ? <CircularProgress size={14} color="inherit" /> : null}
                              disableElevation
                            >
                              {fetching ? '取得中…' : '情報を取得'}
                            </Button>
                          ) : isDraft ? (
                            <Button
                              variant="contained"
                              size="small"
                              color="success"
                              onClick={() => handlePublish(company.id)}
                              disabled={busy}
                              disableElevation
                            >
                              学生に公開
                            </Button>
                          ) : null}

                          <IconButton
                            size="small"
                            aria-label={`${company.name}のその他の操作`}
                            disabled={busy && !fetching}
                            onClick={(e) => setMenuAnchor({ el: e.currentTarget, company })}
                          >
                            <MoreVertIcon fontSize="small" />
                          </IconButton>
                        </Stack>
                      </Stack>
                    </Box>
                  )
                })}
              </Stack>
            </Box>
          ))}
        </Stack>

        {pageCount > 1 && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 2.5, borderTop: '1px solid', borderColor: 'divider' }}>
            <Pagination count={pageCount} page={page} onChange={handlePageChange} color="primary" />
          </Box>
        )}
      </AdminPanel>

      <Menu
        anchorEl={menuAnchor?.el}
        open={Boolean(menuAnchor)}
        onClose={() => setMenuAnchor(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <MenuItem
          disabled={busy}
          onClick={() => menuAnchor && handleFetchPrimary(menuAnchor.company.id, false)}
        >
          <ListItemIcon>
            <RefreshIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>情報を取得</ListItemText>
        </MenuItem>
        <MenuItem
          disabled={busy}
          onClick={() => menuAnchor && handleFetchPrimary(menuAnchor.company.id, true)}
        >
          <ListItemIcon>
            <RefreshIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText>最新の情報に更新</ListItemText>
        </MenuItem>
        {menuAnchor?.company.data_status !== 'published' ? (
          [
            <MenuItem
              key="publish"
              disabled={busy}
              onClick={() => menuAnchor && handlePublish(menuAnchor.company.id)}
            >
              <ListItemText>学生に公開</ListItemText>
            </MenuItem>,
            <MenuItem
              key="reject"
              disabled={busy}
              onClick={() => menuAnchor && handleReject(menuAnchor.company.id)}
            >
              <ListItemText sx={{ color: 'error.main' }}>公開しない</ListItemText>
            </MenuItem>,
          ]
        ) : (
          <MenuItem disabled={busy} onClick={() => menuAnchor && handleReject(menuAnchor.company.id)}>
            <ListItemText sx={{ color: 'error.main' }}>非公開にする</ListItemText>
          </MenuItem>
        )}
      </Menu>

      <Accordion
        disableGutters
        elevation={0}
        sx={{
          mt: 2,
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px !important',
          '&:before': { display: 'none' },
          overflow: 'hidden',
        }}
      >
        <AccordionSummary expandIcon={<ExpandMoreIcon />}>
          <Box>
            <Typography fontWeight={700}>高度な設定（通常は使いません）</Typography>
            <Typography variant="body2" color="text.secondary">
              システム担当向け。マッチング用データの更新や、求人・就職情報への移動
            </Typography>
          </Box>
        </AccordionSummary>
        <AccordionDetails sx={{ pt: 0 }}>
          <Stack spacing={2}>
            <Box>
              <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
                マッチング用データの更新
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                公開中の企業について、学生とのマッチングに使う情報を最新にします。
                {coverage
                  ? `（公開 ${coverage.published_total} 社 / 会社情報 ${pct(coverage.info_rate)} / マッチング情報 ${pct(coverage.profile_rate)}）`
                  : ''}
              </Typography>
              {warmMessage && (
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                  {warmMessage}
                </Typography>
              )}
              {warming && batchProgress && (
                <Box sx={{ mb: 1 }}>
                  <LinearProgress
                    variant="determinate"
                    value={batchProgressPercent(batchProgress)}
                    sx={{ height: 6, borderRadius: 3 }}
                  />
                  <Typography variant="caption" color="warning.main" sx={{ display: 'block', mt: 0.5 }}>
                    更新中はページを更新・移動しないでください。途中結果の表示が消えます。
                  </Typography>
                </Box>
              )}
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Button variant="text" size="small" disabled={busy} onClick={() => handleSeedL1()}>
                  初期データを用意
                </Button>
                <Button variant="outlined" size="small" disabled={busy} onClick={() => handleWarmL1(true)}>
                  対象を確認
                </Button>
                <Button
                  variant="contained"
                  size="small"
                  disabled={busy}
                  onClick={() => handleWarmL1(false)}
                  startIcon={warming ? <CircularProgress size={14} color="inherit" /> : undefined}
                  disableElevation
                >
                  マッチング情報を更新
                </Button>
              </Stack>
            </Box>

            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <Button component={Link} href="/admin/job-positions" size="small" variant="outlined">
                求人一覧へ
              </Button>
              <Button component={Link} href="/admin/graduate-employments" size="small" variant="outlined">
                就職情報へ
              </Button>
            </Stack>
          </Stack>
        </AccordionDetails>
      </Accordion>
    </PageContainer>
  )
}
