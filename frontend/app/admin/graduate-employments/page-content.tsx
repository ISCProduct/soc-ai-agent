'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Box,
  Button,
  Stack,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { AdminListCard } from '@/components/admin/AdminListCard'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { placementCsv, type PlacementBucket, type PlacementSummary } from '@/lib/admin/graduate-summary'

type GraduateEmployment = {
  id: number
  company_id: number
  job_position_id?: number
  graduate_name?: string
  graduation_year?: number
  school_name?: string
  department?: string
  hired_at?: string
  note?: string
  company?: { id: number; name: string }
  job_position?: { id: number; title: string }
}

function BucketLine({ title, rows }: { title: string; rows: PlacementBucket[] }) {
  if (!rows || rows.length === 0) {
    return <Typography variant="body2">{title}: なし</Typography>
  }
  return (
    <Typography variant="body2">
      {title}: {rows.map((row) => `${row.label} ${row.count}件（${row.share_percent}%）`).join('、')}
    </Typography>
  )
}

export default function PageContent() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin && !user?.is_staff) {
      window.location.href = '/'
    }
  }, [])

  const [graduateEntries, setGraduateEntries] = useState<GraduateEmployment[]>([])
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [summary, setSummary] = useState<PlacementSummary | null>(null)
  const [summaryError, setSummaryError] = useState('')
  const [summaryLoading, setSummaryLoading] = useState(true)

  useEffect(() => {
    // 学校を素早く切り替えたとき、後着の古い応答で一覧を上書きしない
    let stale = false
    setLoading(true)
    setError('')
    const params = new URLSearchParams({ limit: '100' })
    if (schoolId !== undefined) params.set('school_id', String(schoolId))
    fetch(`/api/admin/graduate-employments?${params}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then(async (r) => {
        const data = await r.json()
        if (!r.ok) throw new Error(data?.error || '就職情報の取得に失敗しました')
        if (!stale) setGraduateEntries(data?.entries || [])
      })
      .catch((e: unknown) => {
        if (!stale) setError(e instanceof Error ? e.message : '就職情報の取得に失敗しました')
      })
      .finally(() => {
        if (!stale) setLoading(false)
      })
    return () => { stale = true }
  }, [schoolId])

  useEffect(() => {
    let stale = false
    setSummaryLoading(true)
    setSummaryError('')
    const params = new URLSearchParams()
    if (schoolId !== undefined) params.set('school_id', String(schoolId))
    const query = params.toString()
    fetch(`/api/admin/graduate-employments/summary${query ? `?${query}` : ''}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then(async (r) => {
        const data = await r.json()
        if (!r.ok) throw new Error(data?.error || '就職先の内訳を取得できませんでした')
        if (!stale) setSummary(data as PlacementSummary)
      })
      .catch((e: unknown) => {
        if (!stale) setSummaryError(e instanceof Error ? e.message : '就職先の内訳を取得できませんでした')
      })
      .finally(() => {
        if (!stale) setSummaryLoading(false)
      })
    return () => { stale = true }
  }, [schoolId])

  const downloadCsv = () => {
    if (!summary) return
    const blob = new Blob([placementCsv(summary)], { type: 'text/csv;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'graduate-placement-summary.csv'
    link.click()
    URL.revokeObjectURL(url)
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="卒業生の就職情報管理"
        description="卒業生の就職先情報を確認・編集します。"
        backHref="/admin"
        actions={
          <>
            <Button variant="outlined" size="small" component={Link} href="/admin/companies">
              企業管理
            </Button>
            <Button variant="outlined" size="small" component={Link} href="/admin/job-positions">
              求人管理
            </Button>
            <Button
              variant="contained"
              size="small"
              component={Link}
              href="/admin/graduate-employments/new"
              disableElevation
            >
              + 新規登録
            </Button>
          </>
        }
      />

      <AdminPanel title="就職先の内訳">
        <AdminPanelBody>
          {summaryLoading ? (
            <ListSkeleton rows={2} label="就職先の内訳を読み込み中" />
          ) : summaryError ? (
            <ErrorAlert error={summaryError} />
          ) : summary && summary.recorded_count === 0 ? (
            <EmptyState
              title="集計できる就職先がありません"
              description="就職先を登録すると、年度・業界・学校の内訳がここに出ます。"
            />
          ) : summary ? (
            <Stack spacing={1}>
              <Typography variant="body2">{summary.note}</Typography>
              <Typography variant="body2">登録 {summary.recorded_count} 件</Typography>
              <BucketLine title="年度" rows={summary.by_year} />
              <BucketLine title="業界" rows={summary.by_industry} />
              <BucketLine title="学校" rows={summary.by_school} />
              <Button variant="outlined" size="small" onClick={downloadCsv} sx={{ alignSelf: 'flex-start' }}>
                内訳をCSVでダウンロード
              </Button>
            </Stack>
          ) : null}
        </AdminPanelBody>
      </AdminPanel>

      <AdminPanel title="就職情報一覧">
        <AdminPanelBody>
          <Stack spacing={2}>
            <SchoolFilterSelect value={schoolId} onChange={setSchoolId} />
            <Stack spacing={1}>
            <ErrorAlert error={error} />
            {loading ? (
              <ListSkeleton rows={3} label="就職情報を読み込み中" />
            ) : graduateEntries.length === 0 ? (
              error ? null : (
                <EmptyState
                  title="まだ就職情報がありません"
                  description="卒業生の就職先を登録すると、学生が企業を選ぶときの参考になります。"
                  action={<Button variant="contained" component={Link} href="/admin/graduate-employments/new" disableElevation>就職情報を登録</Button>}
                />
              )
            ) : (
              graduateEntries.map((entry) => (
                <AdminListCard key={entry.id}>
                  <Stack direction="row" alignItems="center" justifyContent="space-between">
                    <Box>
                      <Typography variant="subtitle2" fontWeight="bold">
                        {entry.company?.name || `企業ID ${entry.company_id}`}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {entry.graduate_name || '氏名未設定'} / {entry.school_name || '学校未設定'}
                        {entry.graduation_year ? ` (${entry.graduation_year}卒)` : ''}
                      </Typography>
                      {entry.job_position?.title && (
                        <Typography variant="caption" color="text.secondary">
                          職種: {entry.job_position.title}
                        </Typography>
                      )}
                    </Box>
                    <Button
                      size="small"
                      variant="outlined"
                      component={Link}
                      href={`/admin/graduate-employments/${entry.id}/edit`}
                    >
                      編集
                    </Button>
                  </Stack>
                </AdminListCard>
              ))
            )}
            </Stack>
          </Stack>
        </AdminPanelBody>
      </AdminPanel>
    </PageContainer>
  )
}
