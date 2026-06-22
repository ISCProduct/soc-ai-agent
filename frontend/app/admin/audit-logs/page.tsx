'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import {
  Card,
  CardContent,
  Divider,
  IconButton,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'
import { PageContainer } from '@/components/admin/PageContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { AdminListCard } from '@/components/admin/AdminListCard'

type AuditLog = {
  id: number
  actor_email?: string
  action: string
  target_type: string
  target_id: number
  metadata?: string
  created_at: string
}

export default function AdminAuditLogsPage() {
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const loadLogs = async () => {
    setError('')
    const response = await fetch('/api/admin/audit-logs', {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await response.json()
    if (!response.ok) {
      setError(data?.error || '監査ログの取得に失敗しました')
      return
    }
    setLogs(data?.logs || [])
  }

  useEffect(() => {
    loadLogs()
  }, [])

  const filtered = useMemo(() => {
    if (!query) return logs
    const q = query.toLowerCase()
    return logs.filter((log) =>
      `${log.action} ${log.actor_email || ''} ${log.target_type}`.toLowerCase().includes(q),
    )
  }, [logs, query])

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
    <PageContainer maxWidth={1100}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
        <Typography variant="h4" fontWeight="bold">
          監査ログ
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        管理者操作の履歴を確認できます。
      </Typography>

      <ErrorAlert error={error} />

      <Card sx={{ mb: 3 }}>
        <CardContent>
          <TextField
            label="検索 (アクション/操作者/対象)"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            fullWidth
          />
        </CardContent>
      </Card>

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            最新の操作履歴
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={2}>
            {filtered.length === 0 && (
              <Typography variant="body2" color="text.secondary">
                ログがありません。
              </Typography>
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
        </CardContent>
      </Card>
    </PageContainer>
  )
}
