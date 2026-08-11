'use client'

import { useEffect, useState } from 'react'
import {
  Button,
  Chip,
  Stack,
  TableBody,
  TableCell,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { AdminTableWrapper } from '@/components/admin/AdminTableWrapper'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'

type AdminUser = {
  id: number
  email: string
  name: string
  is_guest: boolean
  is_admin: boolean
  target_level?: string
  school_name?: string
  created_at: string
  updated_at: string
}

const PAGE_SIZE_OPTIONS = [10, 25, 50]

export default function AdminUsersPage() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [total, setTotal] = useState(0)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [loading, setLoading] = useState(false)
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  useEffect(() => {
    let cancelled = false
    const timer = setTimeout(async () => {
      setError('')
      const params = new URLSearchParams({
        limit: String(rowsPerPage),
        offset: String(page * rowsPerPage),
      })
      if (query.trim()) params.set('q', query.trim())
      if (schoolId !== undefined) params.set('school_id', String(schoolId))

      const response = await fetch(`/api/admin/users?${params}`, {
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await response.json()
      if (cancelled) return
      if (!response.ok) {
        setError(data?.error || 'ユーザー一覧の取得に失敗しました')
        return
      }
      setUsers(data?.users || [])
      setTotal(data?.total ?? 0)
    }, query ? 400 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [page, rowsPerPage, query, schoolId])

  const handleToggleAdmin = async (user: AdminUser) => {
    const admin = authService.getStoredUser()
    if (!admin?.email) return
    setLoading(true)
    const response = await fetch(`/api/admin/users/${user.id}`, {
      method: 'PUT',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_admin: !user.is_admin }),
    })
    const data = await response.json()
    if (!response.ok) {
      setError(data?.error || '権限更新に失敗しました')
      setLoading(false)
      return
    }
    setUsers((prev) => prev.map((item) => (item.id === user.id ? { ...item, ...data } : item)))
    setLoading(false)
  }

  const handleDeleteUser = async (user: AdminUser) => {
    if (!window.confirm(`${user.email} を完全削除します。履歴書・面接動画を含む個人データは復元できません。よろしいですか？`)) {
      return
    }
    setLoading(true)
    setError('')
    const response = await fetch(`/api/admin/users/${user.id}`, {
      method: 'DELETE',
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await response.json().catch(() => ({}))
    if (!response.ok) {
      setError((data as { error?: string })?.error || 'ユーザー削除に失敗しました')
      setLoading(false)
      return
    }
    setUsers((prev) => prev.filter((item) => item.id !== user.id))
    setTotal((prev) => Math.max(0, prev - 1))
    setLoading(false)
  }

  const handleQueryChange = (value: string) => {
    setQuery(value)
    setPage(0) // 検索時はページを先頭に戻す
  }

  const handleChangePage = (_: unknown, newPage: number) => setPage(newPage)

  const handleChangeRowsPerPage = (e: React.ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(e.target.value, 10))
    setPage(0)
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="ユーザー管理"
        description="管理者権限の付与やユーザー情報の確認を行います。"
        backHref="/admin"
      />

      <ErrorAlert error={error} />

      <AdminPanel title="検索" sx={{ mb: 3 }}>
        <AdminPanelBody>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
            <TextField
              label="検索 (メール/名前/学校名)"
              value={query}
              onChange={(e) => handleQueryChange(e.target.value)}
              fullWidth
            />
            <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(0) }} />
          </Stack>
        </AdminPanelBody>
      </AdminPanel>

      <AdminPanel title="ユーザー一覧">
        <AdminPanelBody>
          {users.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              該当するユーザーがいません。
            </Typography>
          ) : (
            <>
              <AdminTableWrapper>
                  <TableHead>
                    <TableRow>
                      <TableCell>ユーザー</TableCell>
                      <TableCell>区分</TableCell>
                      <TableCell>学校名</TableCell>
                      <TableCell>権限</TableCell>
                      <TableCell>更新日時</TableCell>
                      <TableCell align="right">操作</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {users.map((user) => (
                      <TableRow key={user.id}>
                        <TableCell>
                          <Typography variant="subtitle2" fontWeight="bold">
                            {user.name}
                          </Typography>
                          <Typography variant="caption" color="text.secondary">
                            {user.email}
                          </Typography>
                        </TableCell>
                        <TableCell>{user.target_level || '未設定'}</TableCell>
                        <TableCell>{user.school_name || '未設定'}</TableCell>
                        <TableCell>
                          <Chip
                            label={user.is_admin ? '管理者' : user.is_guest ? 'ゲスト' : '一般'}
                            size="small"
                            color={user.is_admin ? 'success' : 'default'}
                          />
                        </TableCell>
                        <TableCell>{user.updated_at}</TableCell>
                        <TableCell align="right">
                          <Stack direction="row" spacing={1} justifyContent="flex-end">
                            <Button
                              size="small"
                              variant="outlined"
                              disabled={loading}
                              onClick={() => handleToggleAdmin(user)}
                            >
                              {user.is_admin ? '管理者権限を外す' : '管理者にする'}
                            </Button>
                            <Button
                              size="small"
                              variant="outlined"
                              color="error"
                              disabled={loading}
                              onClick={() => handleDeleteUser(user)}
                            >
                              削除
                            </Button>
                          </Stack>
                        </TableCell>
                      </TableRow>
                    ))}
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
