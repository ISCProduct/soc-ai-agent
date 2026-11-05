'use client'

import { useEffect, useMemo, useState } from 'react'
import {
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { useRequirePlatformAdmin } from '@/lib/admin/use-require-platform-admin'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { AdminListCard } from '@/components/admin/AdminListCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'

type AuditLog = {
  id: number
  actor_email?: string
  action: string
  target_type: string
  target_id: number
  metadata?: string
  created_at: string
}

export default function PageContent() {
  const platformReady = useRequirePlatformAdmin()
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  const [query, setQuery] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const loadLogs = async () => {
    setError('')
    setLoading(true)
    try {
      const response = await fetch('/api/admin/audit-logs', {
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await response.json()
      if (!response.ok) {
        setError(data?.error || '監査ログの取得に失敗しました')
        return
      }
      setLogs(data?.logs || [])
    } catch {
      setError('監査ログの取得に失敗しました。通信状況を確認して再読み込みしてください')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!platformReady) return
    loadLogs()
  }, [platformReady])

  const filtered = useMemo(() => {
    if (!query) return logs
    const q = query.toLowerCase()
    return logs.filter((log) =>
      `${log.action} ${log.actor_email || ''} ${log.target_type}`.toLowerCase().includes(q),
    )
  }, [logs, query])

  // フック呼び出し順を変えないため、ガードは全フックの後に置く
  if (!platformReady) return null

  const renderMetadata = (raw?: string) => {
    if (!raw) return '-'
    try {
      const parsed = JSON.parse(raw)
      return JSON.stringify(parsed)
    } catch {
      return raw
    }
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="監査ログ"
        description="管理者操作の履歴を確認できます。"
        backHref="/admin"
      />

      <ErrorAlert error={error} />

      <AdminPanel title="検索" sx={{ mb: 3 }}>
        <AdminPanelBody>
          <TextField
            label="検索 (アクション/操作者/対象)"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            fullWidth
          />
        </AdminPanelBody>
      </AdminPanel>

      <AdminPanel title="最新の操作履歴">
        <AdminPanelBody>
          <Stack spacing={2}>
            {loading && <ListSkeleton rows={3} label="操作履歴を読み込み中" />}
            {!loading && !error && filtered.length === 0 && (
              query ? (
                <EmptyState
                  title="検索に一致する操作履歴はありません"
                  description="検索語を短くするか、消して全件を表示してください。"
                />
              ) : (
                <EmptyState title="まだ操作履歴はありません" description="管理画面での操作が記録されると、ここに表示されます。" />
              )
            )}
            {filtered.map((log) => (
              <AdminListCard key={log.id}>
                <Stack spacing={0.5}>
                  <Typography variant="subtitle2" fontWeight="bold">
                    {log.action}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    操作者: {log.actor_email || '-'} / 対象: {log.target_type} #{log.target_id}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    メタデータ: {renderMetadata(log.metadata)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {log.created_at}
                  </Typography>
                </Stack>
              </AdminListCard>
            ))}
          </Stack>
        </AdminPanelBody>
      </AdminPanel>
    </PageContainer>
  )
}
