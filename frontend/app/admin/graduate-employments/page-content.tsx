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

export default function PageContent() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [graduateEntries, setGraduateEntries] = useState<GraduateEmployment[]>([])
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

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
