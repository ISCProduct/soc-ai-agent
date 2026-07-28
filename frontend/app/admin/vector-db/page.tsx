'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
  Grid,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import RefreshIcon from '@mui/icons-material/Refresh'
import { authService } from '@/lib/auth'

type VectorStats = {
  total_collections: number
  total_documents: number
  cache_hit_count: number
  cache_miss_count: number
  cache_hit_rate: number
  estimated_savings_usd: number
  last_updated: string
}

type CollectionStatus = {
  name: string
  exists: boolean
  count: number
  company_count?: number | null
  sources?: string[]
  doc_types?: string[]
  latest_fetched_at?: string | null
  error?: string
}

type VectorStatus = {
  backend: string
  host?: string | null
  port?: number | null
  company?: string | null
  collections: CollectionStatus[]
  total_documents: number
  message?: string
  error?: string
}

const DOC_TYPES = [
  { value: '', label: 'すべて' },
  { value: 'company_research', label: 'company_research' },
  { value: 'resume_review', label: 'resume_review' },
  { value: 'interview_hints', label: 'interview_hints' },
  { value: 'es_review', label: 'es_review' },
]

export default function AdminVectorPage() {
  const [company, setCompany] = useState('')
  const [status, setStatus] = useState<VectorStatus | null>(null)
  const [stats, setStats] = useState<VectorStats | null>(null)
  const [loading, setLoading] = useState(false)
  const [statsLoading, setStatsLoading] = useState(false)
  const [error, setError] = useState('')
  const [reembedCompany, setReembedCompany] = useState('')
  const [docType, setDocType] = useState('')
  const [refresh, setRefresh] = useState(true)
  const [reembedBusy, setReembedBusy] = useState(false)
  const [reembedResult, setReembedResult] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const loadStatus = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const headers = authService.getAdminFetchHeaders()
      const qs = company.trim() ? `?company=${encodeURIComponent(company.trim())}` : ''
      const res = await fetch(`/api/admin/vector/status${qs}`, { headers, cache: 'no-store' })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data?.message || data?.error || `status ${res.status}`)
      }
      setStatus(data as VectorStatus)
    } catch (err) {
      setStatus(null)
      setError(err instanceof Error ? err.message : '取得に失敗しました')
    } finally {
      setLoading(false)
    }
  }, [company])

  const loadStats = useCallback(async () => {
    setStatsLoading(true)
    try {
      const headers = authService.getAdminFetchHeaders()
      const res = await fetch('/api/admin/vector/stats', { headers, cache: 'no-store' })
      const data = await res.json()
      if (!res.ok) {
        console.warn('Stats API not available:', data?.message || data?.error)
        setStats(null)
        return
      }
      setStats(data as VectorStats)
    } catch (err) {
      console.warn('Failed to load stats:', err instanceof Error ? err.message : '不明なエラー')
      setStats(null)
    } finally {
      setStatsLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadStatus()
    void loadStats()
  }, [loadStatus, loadStats])

  const handleReembed = async () => {
    const name = reembedCompany.trim()
    if (!name) {
      setReembedResult('企業名を入力してください')
      return
    }
    setReembedBusy(true)
    setReembedResult('')
    try {
      const headers = {
        ...authService.getAdminFetchHeaders(),
        'Content-Type': 'application/json',
      }
      const res = await fetch('/api/admin/vector/reembed', {
        method: 'POST',
        headers,
        body: JSON.stringify({
          company_name: name,
          doc_type: docType || null,
          refresh,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        throw new Error(data?.message || data?.error || `reembed ${res.status}`)
      }
      setReembedResult(
        `削除 ${data.deleted ?? 0} 件 / refreshed=${Boolean(data.refreshed)} / sources=${(data.sources || []).join(',') || '-'}`,
      )
      await loadStatus()
      await loadStats()
    } catch (err) {
      setReembedResult(err instanceof Error ? err.message : '再埋め込みに失敗しました')
    } finally {
      setReembedBusy(false)
    }
  }

  return (
    <Box sx={{ p: 4, maxWidth: 1200, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin">
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h4" fontWeight="bold">
          AI/RAG 運用管理
        </Typography>
        <Box sx={{ ml: 'auto' }}>
          <IconButton
            aria-label="統計とステータスを更新"
            onClick={() => { void loadStatus(); void loadStats() }}
            disabled={loading || statsLoading}
          >
            <RefreshIcon />
          </IconButton>
        </Box>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        RAG ベクトルDB、キャッシュヒット率、コスト削減効果を一元管理します。
      </Typography>

      {/* RAG 利用統計セクション */}
      {stats && (
        <Box sx={{ mb: 3 }}>
          <Typography variant="h6" gutterBottom sx={{ mb: 2 }}>
            RAG 利用統計
          </Typography>
          <Grid container spacing={2} sx={{ mb: 3 }}>
            <Grid size={{ xs: 12, md: 6 }}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom>
                    コレクション数
                  </Typography>
                  <Typography variant="h4">
                    {stats.total_collections}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, md: 6 }}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom>
                    総ドキュメント数
                  </Typography>
                  <Typography variant="h4">
                    {stats.total_documents.toLocaleString()}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, md: 6 }}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom>
                    キャッシュヒット率
                  </Typography>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Typography variant="h4">
                      {(stats.cache_hit_rate * 100).toFixed(1)}%
                    </Typography>
                    <Chip
                      label={`${stats.cache_hit_count} hits`}
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  </Stack>
                  <Typography variant="caption" color="textSecondary" sx={{ mt: 1, display: 'block' }}>
                    キャッシュミス: {stats.cache_miss_count}
                  </Typography>
                </CardContent>
              </Card>
            </Grid>
            <Grid size={{ xs: 12, md: 6 }}>
              <Card>
                <CardContent>
                  <Typography color="textSecondary" gutterBottom>
                    推定月次節約額
                  </Typography>
                  <Stack direction="row" alignItems="baseline" spacing={1}>
                    <Typography variant="h4">
                      ${stats.estimated_savings_usd.toFixed(2)}
                    </Typography>
                    <Typography variant="caption" color="success.main">
                      RAG による削減
                    </Typography>
                  </Stack>
                </CardContent>
              </Card>
            </Grid>
          </Grid>
        </Box>
      )}

      {statsLoading && (
        <Box sx={{ display: 'flex', justifyContent: 'center', p: 2, mb: 3 }}>
          <CircularProgress size={24} />
        </Box>
      )}

      <Paper sx={{ p: 3, mb: 3 }}>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }}>
          <TextField
            label="企業名フィルタ（任意）"
            value={company}
            onChange={(e) => setCompany(e.target.value)}
            fullWidth
          />
          <Button variant="contained" onClick={() => void loadStatus()} disabled={loading} sx={{ minWidth: 120 }}>
            更新
          </Button>
        </Stack>
        {loading && <CircularProgress size={24} sx={{ mt: 2 }} />}
        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
        {status && (
          <Box sx={{ mt: 2 }}>
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mb: 1 }}>
              <Chip label={status.backend} size="small" />
              <Chip label={`総ドキュメント ${status.total_documents}`} size="small" color="primary" />
              {status.company ? <Chip label={`filter: ${status.company}`} size="small" variant="outlined" /> : null}
            </Stack>
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>コレクション</TableCell>
                    <TableCell align="right">件数</TableCell>
                    <TableCell align="right">企業件数</TableCell>
                    <TableCell>sources</TableCell>
                    <TableCell>最新 fetched_at</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {(status.collections || []).map((c) => (
                    <TableRow key={c.name}>
                      <TableCell>
                        {c.name}
                        {!c.exists ? '（未作成）' : ''}
                      </TableCell>
                      <TableCell align="right">{c.count}</TableCell>
                      <TableCell align="right">
                        {c.company_count === undefined || c.company_count === null ? '-' : c.company_count}
                      </TableCell>
                      <TableCell>{(c.sources || []).join(', ') || '-'}</TableCell>
                      <TableCell>{c.latest_fetched_at || '-'}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
          </Box>
        )}
      </Paper>

      <Paper sx={{ p: 3 }}>
        <Typography variant="h6" gutterBottom>
          再埋め込み
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          指定企業のベクトルを削除し、任意で WebSearch により再取得・再埋め込みします。
        </Typography>
        <Divider sx={{ mb: 2 }} />
        <Stack spacing={2}>
          <TextField
            label="企業名"
            value={reembedCompany}
            onChange={(e) => setReembedCompany(e.target.value)}
            fullWidth
            required
          />
          <TextField
            select
            label="doc_type"
            value={docType}
            onChange={(e) => setDocType(e.target.value)}
            fullWidth
          >
            {DOC_TYPES.map((d) => (
              <MenuItem key={d.value || 'all'} value={d.value}>
                {d.label}
              </MenuItem>
            ))}
          </TextField>
          <FormControlLabel
            control={<Switch checked={refresh} onChange={(e) => setRefresh(e.target.checked)} />}
            label="削除後に WebSearch で再取得する"
          />
          <Button variant="contained" color="warning" onClick={() => void handleReembed()} disabled={reembedBusy}>
            {reembedBusy ? '実行中…' : '削除 / 再埋め込みを実行'}
          </Button>
          {reembedResult && <Alert severity="info">{reembedResult}</Alert>}
        </Stack>
      </Paper>
    </Box>
  )
}
