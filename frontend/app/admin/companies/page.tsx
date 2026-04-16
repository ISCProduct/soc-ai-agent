'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Stack,
  Typography,
  Pagination,
} from '@mui/material'
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
}

const sourceLabel = (sourceType?: string) => {
  if (sourceType === 'official') return '公式'
  if (sourceType === 'job_site') return 'クローリング'
  return '手動'
}

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
  }, [])

  const handlePageChange = (_: React.ChangeEvent<unknown>, value: number) => {
    setPage(value)
    fetchCompanies(value)
  }

  const handlePublish = async (companyId: number) => {
    const res = await fetch(`/api/admin/companies/${companyId}/publish`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json()
      setError(data?.error || '承認に失敗しました')
      return
    }
    fetchCompanies(page)
  }

  const handleReject = async (companyId: number) => {
    const res = await fetch(`/api/admin/companies/${companyId}/reject`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json()
      setError(data?.error || '却下に失敗しました')
      return
    }
    fetchCompanies(page)
  }

  const filteredCompanies = companies.filter((c) => {
    if (filterStatus === 'all') return true
    return c.data_status === filterStatus
  })

  const pageCount = Math.ceil(total / PAGE_SIZE)

  return (
    <PageContainer maxWidth={1000}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography variant="h4" fontWeight="bold">
          企業管理
        </Typography>
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
        企業情報の公開ステータスを管理します。（全 {total.toLocaleString()} 件）
      </Typography>

      <ErrorAlert error={error} />

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
                  </Box>
                  <Stack direction="row" spacing={1}>
                    <Button
                      variant="outlined"
                      size="small"
                      component={Link}
                      href={`/admin/companies/${company.id}/edit`}
                    >
                      編集
                    </Button>
                    {company.data_status !== 'published' && (
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
