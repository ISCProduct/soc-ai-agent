'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Button,
  Stack,
  TableBody,
  TableCell,
  TableHead,
  TablePagination,
  TableRow,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { AdminTableWrapper } from '@/components/admin/AdminTableWrapper'
import { StatusBadge } from '@/components/admin/StatusBadge'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'

type InterviewSession = {
  id: number
  user_id: number
  status: string
  company_name?: string
  position?: string
  language?: string
  started_at?: string
  finished_at?: string
  created_at: string
}

const STATUS_LABEL: Record<string, { label: string; color: 'default' | 'primary' | 'success' | 'error' }> = {
  created: { label: '作成済み', color: 'default' },
  started: { label: '進行中', color: 'primary' },
  finished: { label: '完了', color: 'success' },
  error: { label: 'エラー', color: 'error' },
}

const PAGE_SIZE_OPTIONS = [10, 25, 50]

export default function AdminInterviewsPage() {
  const [sessions, setSessions] = useState<InterviewSession[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [error, setError] = useState('')
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  useEffect(() => {
    const fetchSessions = async () => {
      setError('')
      const params = new URLSearchParams({
        page: String(page + 1),
        limit: String(rowsPerPage),
      })
      if (schoolId !== undefined) params.set('school_id', String(schoolId))
      const response = await fetch(`/api/admin/interviews?${params}`, {
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await response.json()
      if (!response.ok) {
        setError(data?.error || '面接セッション一覧の取得に失敗しました')
        return
      }
      setSessions(data?.sessions || [])
      setTotal(data?.total ?? 0)
    }
    fetchSessions()
  }, [page, rowsPerPage, schoolId])

  const handleChangePage = (_: unknown, newPage: number) => setPage(newPage)
  const handleChangeRowsPerPage = (e: React.ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10))
    setPage(0)
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.wide}>
      <AdminPageHeader
        title="面接管理"
        description="全ユーザーの面接セッション一覧です。動画を確認するには詳細ページを開いてください。"
        backHref="/admin"
      />

      <ErrorAlert error={error} />

      <AdminPanel title="セッション一覧">
        <AdminPanelBody>
          <Stack sx={{ mb: 2 }}>
            <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(0) }} />
          </Stack>
          {sessions.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              面接セッションがありません。
            </Typography>
          ) : (
            <>
              <AdminTableWrapper>
                  <TableHead>
                    <TableRow>
                      <TableCell>ID</TableCell>
                      <TableCell>ユーザーID</TableCell>
                      <TableCell>志望企業</TableCell>
                      <TableCell>ポジション</TableCell>
                      <TableCell>ステータス</TableCell>
                      <TableCell>開始日時</TableCell>
                      <TableCell>作成日時</TableCell>
                      <TableCell align="right">操作</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {sessions.map((session) => {
                      const st = STATUS_LABEL[session.status] ?? { label: session.status, color: 'default' as const }
                      return (
                        <TableRow key={session.id}>
                          <TableCell>{session.id}</TableCell>
                          <TableCell>{session.user_id}</TableCell>
                          <TableCell>{session.company_name || '—'}</TableCell>
                          <TableCell>{session.position || '—'}</TableCell>
                          <TableCell>
                            <StatusBadge status={session.status} fallbackLabel={st.label} />
                          </TableCell>
                          <TableCell>
                            {session.started_at
                              ? new Date(session.started_at).toLocaleString('ja-JP')
                              : '—'}
                          </TableCell>
                          <TableCell>
                            {new Date(session.created_at).toLocaleString('ja-JP')}
                          </TableCell>
                          <TableCell align="right">
                            <Button
                              size="small"
                              variant="outlined"
                              component={Link}
                              href={`/admin/interviews/${session.id}`}
                            >
                              詳細・動画
                            </Button>
                          </TableCell>
                        </TableRow>
                      )
                    })}
                  </TableBody>
              </AdminTableWrapper>
              <TablePagination
                component="div"
                count={total}
                page={page}
                onPageChange={handleChangePage}
                rowsPerPage={rowsPerPage}
                onRowsPerPageChange={handleChangeRowsPerPage}
                rowsPerPageOptions={PAGE_SIZE_OPTIONS}
                labelRowsPerPage="表示件数:"
                labelDisplayedRows={({ from, to, count }) => `${from}–${to} / ${count}件`}
              />
            </>
          )}
        </AdminPanelBody>
      </AdminPanel>
    </PageContainer>
  )
}
