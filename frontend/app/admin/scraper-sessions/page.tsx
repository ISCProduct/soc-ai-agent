'use client'

import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import { authService } from '@/lib/auth'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

type ScraperSession = {
  id: number
  site_key: string
  cookies: string
  expires_at?: string
  updated_at: string
}

export default function AdminScraperSessionsPage() {
  const [sessions, setSessions] = useState<ScraperSession[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [siteKey, setSiteKey] = useState('')
  const [cookies, setCookies] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [saving, setSaving] = useState(false)

  const [deleteTarget, setDeleteTarget] = useState<string | null>(null)
  const [deleting, setDeleting] = useState(false)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const headers = authService.getAdminFetchHeaders()

  const loadSessions = async () => {
    setLoading(true)
    try {
      const res = await fetch('/api/admin/scraper-sessions', { headers })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || 'セッション一覧の取得に失敗しました')
        return
      }
      setSessions(data.sessions || [])
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadSessions()
  }, [])

  const handleUpsert = async () => {
    if (!siteKey || !cookies) return
    setSaving(true)
    setError('')
    setSuccess('')
    try {
      const body: Record<string, string> = { site_key: siteKey, cookies }
      if (expiresAt) body.expires_at = new Date(expiresAt).toISOString()

      const res = await fetch('/api/admin/scraper-sessions', {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || 'セッションの保存に失敗しました')
        return
      }
      setSuccess(`セッション "${siteKey}" を保存しました`)
      setSiteKey('')
      setCookies('')
      setExpiresAt('')
      await loadSessions()
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      const res = await fetch(`/api/admin/scraper-sessions/${encodeURIComponent(deleteTarget)}`, {
        method: 'DELETE',
        headers,
      })
      if (res.ok || res.status === 204) {
        setSuccess(`セッション "${deleteTarget}" を削除しました`)
        await loadSessions()
      } else {
        const data = await res.json().catch(() => ({}))
        setError(data.error || '削除に失敗しました')
      }
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setDeleting(false)
      setDeleteTarget(null)
    }
  }

  const isExpired = (expiresAt?: string) => {
    if (!expiresAt) return false
    return new Date(expiresAt) < new Date()
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="スクレイパーセッション管理"
        description="クローリングに使用するサイトごとのセッション（Cookie）を管理します。"
        backHref="/admin"
      />

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {success && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>{success}</Alert>}

      <Stack spacing={3}>
        <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>セッション一覧</Typography>
            {loading ? (
              <CircularProgress size={24} />
            ) : sessions.length === 0 ? (
              <Typography variant="body2" color="text.secondary">登録されたセッションはありません</Typography>
            ) : (
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>サイトキー</TableCell>
                    <TableCell>有効期限</TableCell>
                    <TableCell>最終更新</TableCell>
                    <TableCell>操作</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {sessions.map((s) => (
                    <TableRow key={s.site_key}>
                      <TableCell>{s.site_key}</TableCell>
                      <TableCell>
                        {s.expires_at ? (
                          <Chip
                            label={new Date(s.expires_at).toLocaleString('ja-JP')}
                            color={isExpired(s.expires_at) ? 'error' : 'success'}
                            size="small"
                          />
                        ) : (
                          <Typography variant="body2" color="text.secondary">なし</Typography>
                        )}
                      </TableCell>
                      <TableCell>{new Date(s.updated_at).toLocaleString('ja-JP')}</TableCell>
                      <TableCell>
                        <IconButton
                          size="small"
                          color="error"
                          onClick={() => setDeleteTarget(s.site_key)}
                        >
                          <DeleteIcon fontSize="small" />
                        </IconButton>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
          <CardContent>
            <Typography variant="h6" gutterBottom>セッションを追加・更新</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              同じサイトキーが存在する場合は上書きします。
            </Typography>
            <Stack spacing={2}>
              <TextField
                label="サイトキー"
                size="small"
                value={siteKey}
                onChange={(e) => setSiteKey(e.target.value)}
                placeholder="例: career_tasu"
              />
              <TextField
                label="Cookie文字列"
                size="small"
                multiline
                rows={4}
                value={cookies}
                onChange={(e) => setCookies(e.target.value)}
                placeholder="ブラウザのCookieをそのまま貼り付けてください"
              />
              <TextField
                label="有効期限（任意）"
                size="small"
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
                InputLabelProps={{ shrink: true }}
              />
              <Divider />
              <Button
                variant="contained"
                onClick={handleUpsert}
                disabled={saving || !siteKey || !cookies}
                startIcon={saving ? <CircularProgress size={16} /> : undefined}
                sx={{ alignSelf: 'flex-start' }}
              >
                保存
              </Button>
            </Stack>
          </CardContent>
        </Card>
      </Stack>

      <Dialog open={deleteTarget !== null} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>削除の確認</DialogTitle>
        <DialogContent>
          <Typography>
            セッション &quot;{deleteTarget}&quot; を削除します。よろしいですか？
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>キャンセル</Button>
          <Button
            color="error"
            variant="contained"
            onClick={handleDelete}
            disabled={deleting}
            startIcon={deleting ? <CircularProgress size={16} /> : undefined}
          >
            削除
          </Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
