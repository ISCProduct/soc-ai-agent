'use client'

import { useState, useEffect, Suspense } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
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

const STATUS_LABELS: Record<string, string> = {
  applied: '応募済み',
  document_passed: '書類通過',
  interview: '面接中',
  offered: '内定',
  accepted: '内定承諾',
  declined: '辞退',
  rejected: '不合格',
}

const STATUS_COLORS: Record<string, 'default' | 'primary' | 'secondary' | 'error' | 'info' | 'success' | 'warning'> = {
  applied: 'default',
  document_passed: 'info',
  interview: 'primary',
  offered: 'success',
  accepted: 'success',
  declined: 'default',
  rejected: 'error',
}

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
  const searchParams = useSearchParams()
  const userId = searchParams.get('user_id')

  const [applications, setApplications] = useState<Application[]>([])
  const [loading, setLoading] = useState(true)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [editStatus, setEditStatus] = useState('')
  const [editNotes, setEditNotes] = useState('')
  const [saving, setSaving] = useState(false)
  const [snackbar, setSnackbar] = useState<{ open: boolean; message: string; severity: 'success' | 'error' }>({
    open: false,
    message: '',
    severity: 'success',
  })

  useEffect(() => {
    if (!userId) return
    const load = async () => {
      try {
        const res = await fetch(`/api/applications?user_id=${userId}`)
        if (!res.ok) throw new Error('取得失敗')
        const data = await res.json()
        setApplications(data.applications || [])
      } catch {
        setSnackbar({ open: true, message: '応募データの取得に失敗しました', severity: 'error' })
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [userId])

  const startEdit = (app: Application) => {
    setEditingId(app.id)
    setEditStatus(app.status)
    setEditNotes(app.notes || '')
  }

  const cancelEdit = () => {
    setEditingId(null)
    setEditStatus('')
    setEditNotes('')
  }

  const saveEdit = async (appId: number) => {
    if (!userId) return
    setSaving(true)
    try {
      const res = await fetch(`/api/applications/${appId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ user_id: Number(userId), status: editStatus, notes: editNotes }),
      })
      if (!res.ok) throw new Error('更新失敗')
      setApplications(prev =>
        prev.map(a => (a.id === appId ? { ...a, status: editStatus, notes: editNotes } : a))
      )
      setSnackbar({ open: true, message: 'ステータスを更新しました', severity: 'success' })
      cancelEdit()
    } catch {
      setSnackbar({ open: true, message: 'ステータスの更新に失敗しました', severity: 'error' })
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

      {applications.length === 0 ? (
        <Paper sx={{ p: 4, textAlign: 'center' }}>
          <Typography color="text.secondary">応募した企業はまだありません</Typography>
          <Button variant="contained" sx={{ mt: 2 }} onClick={() => router.push(`/results?user_id=${userId}`)}>
            マッチング結果に戻る
          </Button>
        </Paper>
      ) : (
        <Stack spacing={2}>
          {applications.map(app => (
            <Card key={app.id} variant="outlined">
              <CardContent>
                <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                  <Box>
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
                        {Object.entries(STATUS_LABELS).map(([value, label]) => (
                          <MenuItem key={value} value={value}>
                            {label}
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
                    <Button
                      size="small"
                      startIcon={<Edit />}
                      onClick={() => startEdit(app)}
                    >
                      ステータスを更新
                    </Button>
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
