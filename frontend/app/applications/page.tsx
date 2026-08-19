'use client'

import { useState, useEffect, Suspense, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import {
  Box,
  Paper,
  Typography,
  Button,
  Card,
  CardContent,
  Stack,
  CircularProgress,
  Chip,
  Select,
  MenuItem,
  FormControl,
  InputLabel,
  TextField,
  Snackbar,
  Alert,
  IconButton,
} from '@mui/material'
import { ArrowBack, Edit, Check } from '@mui/icons-material'
import { authService } from '@/lib/auth'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { fetchWithTimeout } from '@/lib/fetch-timeout'
import {
  STATUS_COLORS,
  STATUS_LABELS,
  isTerminalStatus,
  userNextStatuses,
} from '@/lib/application-status'

interface Application {
  id: number
  company_id: number
  company_name: string
  company_industry: string
  match_id: number
  status: string
  notes: string
  applied_at: string | null
  status_updated_at: string | null
}

function ApplicationsContent() {
  const router = useRouter()

  const [applications, setApplications] = useState<Application[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editStatus, setEditStatus] = useState('')
  const [editNotes, setEditNotes] = useState('')
  const [saving, setSaving] = useState(false)
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({
    open: false,
    message: '',
    severity: 'success',
  })

  const loadApplications = useCallback(async () => {
    setLoading(true)
    setLoadError(null)
    try {
      const res = await fetchWithTimeout('/api/applications', {
        headers: authService.getUserFetchHeaders(),
        cache: 'no-store',
      })
      if (!res.ok) throw new Error('取得失敗')
      const data = await res.json()
      setApplications(data.applications || [])
    } catch {
      setLoadError('応募データの取得に失敗しました')
      setApplications([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user || user.is_guest) {
      router.replace('/login')
      return
    }
    void loadApplications()
  }, [router, loadApplications])

  const startEdit = (app: Application) => {
    const next = userNextStatuses(app.status)
    setEditingId(app.id)
    setEditStatus(next[0] ?? app.status)
    setEditNotes(app.notes || '')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditStatus('')
    setEditNotes('')
  }

  const saveEdit = async (appId: number) => {
    setSaving(true)
    try {
      const res = await fetch(`/api/applications/${appId}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getUserFetchHeaders(),
        },
        body: JSON.stringify({ status: editStatus, notes: editNotes }),
      })
      if (!res.ok) {
        let message = 'ステータスの更新に失敗しました'
        try {
          const body = await res.json()
          if (body?.error) message = body.error
        } catch { /* ignore parse error */ }
        throw new Error(message)
      }
      setApplications(prev =>
        prev.map(a => (a.id === appId ? { ...a, status: editStatus, notes: editNotes } : a))
      )
      setSnackbar({ open: true, message: 'ステータスを更新しました', severity: 'success' })
      cancelEdit()
    } catch (e) {
      setSnackbar({ open: true, message: e instanceof Error ? e.message : 'ステータスの更新に失敗しました', severity: 'error' })
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}>
        <CircularProgress />
      </Box>
    )
  }

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Stack direction="row" alignItems="center" spacing={1} mb={3}>
        <IconButton onClick={() => router.back()}>
          <ArrowBack />
        </IconButton>
        <Typography variant="h5" fontWeight="bold">
          選考管理
        </Typography>
      </Stack>

      {loadError ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Alert severity="error" sx={{ mb: 2 }}>{loadError}</Alert>
          <Button variant="contained" onClick={() => void loadApplications()}>
            再試行
          </Button>
        </Paper>
      ) : applications.length === 0 ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Typography color="text.secondary">応募した企業はまだありません</Typography>
          <Button variant="contained" sx={{ mt: 2 }} onClick={() => router.push(getResultsPathOrChat())}>
            マッチング結果に戻る
          </Button>
        </Paper>
      ) : (
        <Stack spacing={2}>
          {applications.map(app => (
            <Card key={app.id} variant="outlined">
              <CardContent>
                <Stack direction="row" justifyContent="space-between" alignItems="flex-start" flexWrap="wrap" gap={1}>
                  <Box sx={{ minWidth: 0, flex: 1 }}>
                    <Typography variant="h6" fontWeight="bold">
                      {app.company_name}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {app.company_industry}
                    </Typography>
                    {app.applied_at && (
                      <Typography variant="caption" color="text.secondary">
                        応募日: {new Date(app.applied_at).toLocaleDateString('ja-JP')}
                      </Typography>
                    )}
                  </Box>
                  <Chip
                    label={STATUS_LABELS[app.status] || app.status}
                    color={STATUS_COLORS[app.status] || 'default'}
                    size="small"
                    sx={{ flexShrink: 0 }}
                  />
                </Stack>

                {editingId === app.id ? (
                  <Box mt={2}>
                    <FormControl fullWidth size="small" sx={{ mb: 2 }}>
                      <InputLabel>選考ステータス</InputLabel>
                      <Select
                        value={editStatus}
                        label="選考ステータス"
                        onChange={e => setEditStatus(e.target.value)}
                      >
                        {userNextStatuses(app.status).map(value => (
                          <MenuItem key={value} value={value}>
                            {STATUS_LABELS[value] || value}
                          </MenuItem>
                        ))}
                      </Select>
                    </FormControl>
                    <TextField
                      fullWidth
                      size="small"
                      label="メモ"
                      multiline
                      rows={2}
                      value={editNotes}
                      onChange={e => setEditNotes(e.target.value)}
                      sx={{ mb: 2 }}
                    />
                    <Stack direction="row" spacing={1}>
                      <Button
                        variant="contained"
                        size="small"
                        startIcon={<Check />}
                        onClick={() => saveEdit(app.id)}
                        disabled={saving}
                      >
                        保存
                      </Button>
                      <Button variant="outlined" size="small" onClick={cancelEdit} disabled={saving}>
                        キャンセル
                      </Button>
                    </Stack>
                  </Box>
                ) : (
                  <Box mt={1}>
                    {app.notes && (
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
                        {app.notes}
                      </Typography>
                    )}
                    {!isTerminalStatus(app.status) && userNextStatuses(app.status).length > 0 && (
                      <Button
                        size="small"
                        startIcon={<Edit />}
                        onClick={() => startEdit(app)}
                      >
                        ステータスを更新
                      </Button>
                    )}
                  </Box>
                )}
              </CardContent>
            </Card>
          ))}
        </Stack>
      )}

      <Snackbar
        open={snackbar.open}
        autoHideDuration={4000}
        onClose={() => setSnackbar(prev => ({ ...prev, open: false }))}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert severity={snackbar.severity} onClose={() => setSnackbar(prev => ({ ...prev, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}

export default function ApplicationsPage() {
  return (
    <Suspense fallback={<Box sx={{ display: 'flex', justifyContent: 'center', mt: 8 }}><CircularProgress /></Box>}>
      <ApplicationsContent />
    </Suspense>
  )
}
