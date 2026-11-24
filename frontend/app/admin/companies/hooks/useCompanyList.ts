'use client'

// 企業一覧の取得・絞り込み・一括公開用の選択を受け持つ。
// effect の宣言順は分割前の page-content.tsx と同じに保つ（取得の順序が変わらないように）。
import { useCallback, useEffect, useState } from 'react'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'
import { runCompanyListReload, type CompanyListReloadKind } from '../company-list-reload'
import {
  INDUSTRY_UNSET,
  PAGE_SIZE,
  canSelectForPublish,
  groupCompaniesByIndustry,
  type Company,
  type FilterReadiness,
  type FilterStatus,
  type L1Coverage,
} from '../company-list-model'

export function useCompanyList() {
  const [isPlatform, setIsPlatform] = useState(false)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin && !user?.is_staff) {
      window.location.href = '/'
      return
    }
    void getAdminSchoolAccess()
      .then((access) => setIsPlatform(!access.restricted))
      .catch(() => setIsPlatform(false))
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
  const [selectedIds, setSelectedIds] = useState<number[]>([])
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)

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
    if (isPlatform) void fetchCoverage()
    void fetchIndustries()
  }, [isPlatform, fetchCoverage, fetchIndustries])

  const handlePageChange = (_: React.ChangeEvent<unknown>, value: number) => {
    setPage(value)
  }

  const reloadCurrentList = (kind: CompanyListReloadKind) =>
    runCompanyListReload(kind, {
      companies: () =>
        fetchCompanies(page, searchName, filterStatus, filterIndustry, filterReadiness, groupByIndustry, schoolId),
      industries: () => fetchIndustries(),
      coverage: () => fetchCoverage(),
    })

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

  const toggleSelect = (companyId: number) => {
    const company = companies.find((c) => c.id === companyId)
    if (!company || !canSelectForPublish(company)) return
    setSelectedIds((prev) =>
      prev.includes(companyId) ? prev.filter((id) => id !== companyId) : [...prev, companyId],
    )
  }

  // 公開操作はシステム管理者専用なので、一括公開の選択UIも担当校管理者には出さない
  const selectableOnPage = isPlatform ? companies.filter(canSelectForPublish) : []
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

  return {
    isPlatform,
    companies,
    total,
    page,
    error,
    setError,
    filterStatus,
    setFilterStatus,
    filterIndustry,
    setFilterIndustry,
    filterReadiness,
    setFilterReadiness,
    groupByIndustry,
    setGroupByIndustry,
    industries,
    searchInput,
    setSearchInput,
    coverage,
    setCoverage,
    selectedIds,
    setSelectedIds,
    schoolId,
    setSchoolId,
    setPage,
    fetchCoverage,
    handlePageChange,
    reloadCurrentList,
    resetFilters,
    hasActiveFilters,
    companyGroups,
    filterSummary,
    toggleSelect,
    selectableOnPage,
    selectedSelectableCount,
    allSelectableSelected,
    toggleSelectAllSelectable,
  }
}
