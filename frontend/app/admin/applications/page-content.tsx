'use client'

import { useEffect, useState } from 'react'
import {
  Button,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  TableBody,
  TableCell,
  TableHead,
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
import { StatusBadge } from '@/components/admin/StatusBadge'
import { APPLICATION_STATUSES, STATUS_LABELS, adminNextStatuses } from '@/lib/application-status'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'

type AdminApplication = {
  id: number
  user_id: number
  company_id: number
  company_name: string
  status: string
  notes: string
}

export default function PageContent() {
  const [applications, setApplications] = useState<AdminApplication[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [error, setError] = useState('')
  const [pending, setPending] = useState<Record<number, { status: string; notes: string }>>({})
  // 担当校を持つ管理者(先生)は school_id が必須（未指定は 400）。
  // 解決前に一覧を取ると必ず失敗するため、scopeReady で初回ロードを待たせる(#1157)
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [scopeReady, setScopeReady] = useState(false)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  // 学校アクセスの解決は SchoolFilterSelect に任せ、完了通知を受けて初回ロードする。
  // ページ側でも fetch すると /api/admin/me/school-access を二重に叩くことになる。
  const handleScopeResolved = (failure?: string) => {
    if (failure) setError(failure)
    setScopeReady(true)
  }

  const load = async (status = statusFilter, school = schoolId) => {
    setError('')
    const params = new URLSearchParams()
    if (status) params.set('status', status)
    if (school !== undefined) params.set('school_id', String(school))
    const qs = params.toString()
    const res = await fetch(`/api/admin/applications${qs ? `?${qs}` : ''}`, {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '応募一覧の取得に失敗しました')
      return
    }
    setApplications(data?.applications || [])
  }

  useEffect(() => {
    if (!scopeReady) return
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusFilter, schoolId, scopeReady])

  const save = async (app: AdminApplication) => {
    const next = pending[app.id] ?? { status: app.status, notes: app.notes || '' }
    const body: { status: string; notes?: string } = { status: next.status }
    if (next.notes !== (app.notes || '')) {
      body.notes = next.notes
    }
    const res = await fetch(`/api/admin/applications/${app.id}/status`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAdminFetchHeaders(),
      },
      body: JSON.stringify(body),
    })
    const data = await res.json().catch(() => null)
    if (!res.ok) {
      setError(data?.error || data?.message || 'ステータスの更新に失敗しました')
      return
    }
    setPending((prev) => {
      const copy = { ...prev }
      delete copy[app.id]
      return copy
    })
    await load()
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.wide}>
      <AdminPageHeader
        title="応募・選考管理"
        description="学生の応募一覧を確認し、許可された遷移でステータスを更新します。"
        backHref="/admin"
      />
      <ErrorAlert error={error} />
      <AdminPanel title="応募一覧">
        <AdminPanelBody>
          <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2, flexWrap: 'wrap' }}>
            <FormControl size="small" sx={{ minWidth: 200 }}>
              <InputLabel>ステータス</InputLabel>
              <Select
                value={statusFilter}
                label="ステータス"
                onChange={(e) => setStatusFilter(e.target.value)}
              >
                <MenuItem value="">すべて</MenuItem>
                {APPLICATION_STATUSES.map((s) => (
                  <MenuItem key={s} value={s}>
                    {STATUS_LABELS[s] || s}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
            {/* 担当校が1校の管理者には表示されず自動適用される(#798 の既存コンポーネント) */}
            <SchoolFilterSelect value={schoolId} onChange={setSchoolId} onResolved={handleScopeResolved} />
          </Stack>
          {applications.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              応募はありません。
            </Typography>
          ) : (
            <AdminTableWrapper>
              <TableHead>
                <TableRow>
                  <TableCell>ID</TableCell>
                  <TableCell>ユーザー</TableCell>
                  <TableCell>企業</TableCell>
                  <TableCell>ステータス</TableCell>
                  <TableCell>メモ</TableCell>
                  <TableCell />
                </TableRow>
              </TableHead>
              <TableBody>
                {applications.map((app) => {
                  const edit = pending[app.id] ?? { status: app.status, notes: app.notes || '' }
                  const nexts = adminNextStatuses(app.status)
                  return (
                    <TableRow key={app.id}>
                      <TableCell>{app.id}</TableCell>
                      <TableCell>{app.user_id}</TableCell>
                      <TableCell>{app.company_name || app.company_id}</TableCell>
                      <TableCell>
                        {nexts.length === 0 ? (
                          <StatusBadge status={app.status} fallbackLabel={STATUS_LABELS[app.status] || app.status} />
                        ) : (
                          <Select
                            size="small"
                            value={edit.status}
                            onChange={(e) =>
                              setPending((prev) => ({
                                ...prev,
                                [app.id]: { ...edit, status: e.target.value },
                              }))
                            }
                          >
                            <MenuItem value={app.status}>{STATUS_LABELS[app.status] || app.status}</MenuItem>
                            {nexts.map((s) => (
                              <MenuItem key={s} value={s}>
                                {STATUS_LABELS[s] || s}
                              </MenuItem>
                            ))}
                          </Select>
                        )}
                      </TableCell>
                      <TableCell>
                        <TextField
                          size="small"
                          value={edit.notes}
                          onChange={(e) =>
                            setPending((prev) => ({
                              ...prev,
                              [app.id]: { ...edit, notes: e.target.value },
                            }))
                          }
                        />
                      </TableCell>
                      <TableCell>
                        <Button size="small" variant="contained" onClick={() => void save(app)}>
                          更新
                        </Button>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </AdminTableWrapper>
          )}
        </AdminPanelBody>
      </AdminPanel>
    </PageContainer>
  )
}
