'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Dialog,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  InputAdornment,
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import SearchIcon from '@mui/icons-material/Search'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { authService } from '@/lib/auth'

type UserSummary = {
  user_id: number
  name: string
  email: string
  role: string
  registered_at: string
  session_count: number
  last_session_at: string | null
  avg_score: number | null | undefined
}

type SessionEntry = {
  session_id: number
  ended_at: string | null
  avg_score: number | null | undefined
  scores: Record<string, number> | null
}

const SCORE_LABELS: Record<string, string> = {
  logic: '論理性',
  specificity: '具体性',
  ownership: '当事者意識',
  communication: '伝達力',
  enthusiasm: '熱意',
}

const SORT_OPTIONS = [
  { value: 'registered_desc', label: '登録日（新しい順）' },
  { value: 'avg_score_desc', label: '平均スコア（高い順）' },
  { value: 'avg_score_asc', label: '平均スコア（低い順）' },
  { value: 'session_count_desc', label: '練習回数（多い順）' },
]

function ScoreBadge({ score }: { score: number | null | undefined }) {
  if (score == null) return <Typography color="text.disabled" fontSize="0.85rem">—</Typography>
  const color = score >= 4 ? '#388e3c' : score >= 3 ? '#f57c00' : '#d32f2f'
  return (
    <Box sx={{
      display: 'inline-block', px: 1, py: 0.3, borderRadius: 1,
      bgcolor: color + '18', color, fontWeight: 700, fontSize: '0.85rem',
    }}>
      {score.toFixed(1)}
    </Box>
  )
}

export default function AdminScoreDashboardPage() {
  const [adminEmail, setAdminEmail] = useState('')
  const [users, setUsers] = useState<UserSummary[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [query, setQuery] = useState('')
  const [sort, setSort] = useState('registered_desc')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const [detailUser, setDetailUser] = useState<UserSummary | null>(null)
  const [detailSessions, setDetailSessions] = useState<SessionEntry[]>([])
  const [detailLoading, setDetailLoading] = useState(false)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
    setAdminEmail(user.email)
  }, [])

  const fetchUsers = useCallback(async () => {
    if (!adminEmail) return
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({
        page: String(page + 1),
        limit: String(rowsPerPage),
        sort,
        ...(query ? { query } : {}),
      })
      const res = await fetch(`/api/admin/dashboard/users?${params}`, {
        headers: { 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' },
      })
      if (!res.ok) throw new Error('データの取得に失敗しました')
      const data = await res.json()
      setUsers(data.users ?? [])
      setTotal(data.total ?? 0)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [adminEmail, page, rowsPerPage, sort, query])

  useEffect(() => {
    fetchUsers()
  }, [fetchUsers])

  const handleExport = () => {
    const link = document.createElement('a')
    link.href = '/api/admin/dashboard/export'
    const headers = new Headers({ 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' })
    fetch('/api/admin/dashboard/export', { headers })
      .then(res => res.blob())
      .then(blob => {
        const url = URL.createObjectURL(blob)
        link.href = url
        link.download = 'user_scores.csv'
        link.click()
        URL.revokeObjectURL(url)
      })
  }

  const handleOpenDetail = async (user: UserSummary) => {
    setDetailUser(user)
    setDetailSessions([])
    setDetailLoading(true)
    try {
      const res = await fetch(`/api/admin/dashboard/users/${user.user_id}/sessions`, {
        headers: { 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' },
      })
      const data = await res.json()
      setDetailSessions(data.sessions ?? [])
    } catch {
      setDetailSessions([])
    } finally {
      setDetailLoading(false)
    }
  }

  return (
    <Box sx={{ p: 4, maxWidth: 1200, mx: 'auto' }}>
      {/* Header */}
      <Stack direction="row" alignItems="center" spacing={2} mb={3}>
        <IconButton component={Link} href="/admin">
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h5" fontWeight={700} flex={1}>
          ユーザー別スコアダッシュボード
        </Typography>
        <Button
          variant="outlined"
          startIcon={<FileDownloadIcon />}
          onClick={handleExport}
        >
          CSV エクスポート
        </Button>
      </Stack>

      {error && (
        <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* Filters */}
      <Stack direction="row" spacing={2} mb={2}>
        <TextField
          placeholder="名前・メール・学校名で検索"
          value={query}
          onChange={e => { setQuery(e.target.value); setPage(0) }}
          size="small"
          sx={{ flex: 1 }}
          InputProps={{
            startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>
          }}
        />
        <TextField
          select
          label="並び替え"
          value={sort}
          onChange={e => { setSort(e.target.value); setPage(0) }}
          size="small"
          sx={{ minWidth: 220 }}
        >
          {SORT_OPTIONS.map(opt => (
            <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
          ))}
        </TextField>
      </Stack>

      {/* Table */}
      <Paper elevation={1}>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>名前</TableCell>
                <TableCell>メール</TableCell>
                <TableCell>区分</TableCell>
                <TableCell>登録日</TableCell>
                <TableCell align="right">練習回数</TableCell>
                <TableCell>最終練習日</TableCell>
                <TableCell align="center">平均スコア</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {loading ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 4 }}>
                    <CircularProgress size={24} />
                  </TableCell>
                </TableRow>
              ) : users.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={8} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    ユーザーが見つかりません
                  </TableCell>
                </TableRow>
              ) : users.map(u => (
                <TableRow key={u.user_id} hover>
                  <TableCell>
                    <Typography fontWeight={500}>{u.name || '—'}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">{u.email}</Typography>
                  </TableCell>
                  <TableCell>
                    <Chip label={u.role || '新卒'} size="small" variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">
                      {new Date(u.registered_at).toLocaleDateString('ja-JP')}
                    </Typography>
                  </TableCell>
                  <TableCell align="right">
                    <Typography fontWeight={u.session_count > 0 ? 600 : 400} color={u.session_count > 0 ? 'primary' : 'text.secondary'}>
                      {u.session_count}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">
                      {u.last_session_at
                        ? new Date(u.last_session_at).toLocaleDateString('ja-JP')
                        : '—'}
                    </Typography>
                  </TableCell>
                  <TableCell align="center">
                    <ScoreBadge score={u.avg_score} />
                  </TableCell>
                  <TableCell>
                    <Tooltip title="セッション詳細">
                      <IconButton size="small" onClick={() => handleOpenDetail(u)} disabled={u.session_count === 0}>
                        <ExpandMoreIcon fontSize="small" />
                      </IconButton>
                    </Tooltip>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={e => { setRowsPerPage(Number(e.target.value)); setPage(0) }}
          rowsPerPageOptions={[10, 25, 50]}
          labelRowsPerPage="表示件数:"
          labelDisplayedRows={({ from, to, count }) => `${from}–${to} / ${count}`}
        />
      </Paper>

      {/* Session detail dialog */}
      <Dialog open={!!detailUser} onClose={() => setDetailUser(null)} maxWidth="md" fullWidth>
        <DialogTitle>
          {detailUser?.name || detailUser?.email} — セッション履歴
        </DialogTitle>
        <DialogContent>
          {detailLoading ? (
            <Box textAlign="center" py={4}><CircularProgress /></Box>
          ) : detailSessions.length === 0 ? (
            <Typography color="text.secondary" py={2}>セッションがありません</Typography>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                  <TableCell>終了日時</TableCell>
                  <TableCell align="center">平均スコア</TableCell>
                  {Object.keys(SCORE_LABELS).map(k => (
                    <TableCell key={k} align="center">{SCORE_LABELS[k]}</TableCell>
                  ))}
                </TableRow>
              </TableHead>
              <TableBody>
                {detailSessions.map(s => (
                  <TableRow key={s.session_id} hover>
                    <TableCell>
                      {s.ended_at
                        ? new Date(s.ended_at).toLocaleString('ja-JP', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
                        : '—'}
                    </TableCell>
                    <TableCell align="center"><ScoreBadge score={s.avg_score} /></TableCell>
                    {Object.keys(SCORE_LABELS).map(k => (
                      <TableCell key={k} align="center">
                        <ScoreBadge score={s.scores?.[k] ?? null} />
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </DialogContent>
      </Dialog>
    </Box>
  )
}
