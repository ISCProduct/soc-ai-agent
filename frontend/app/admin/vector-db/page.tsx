'use client'

import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
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
import { authService } from '@/lib/auth'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

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
  const [loading, setLoading] = useState(false)
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

  useEffect(() => {
    void loadStatus()
  }, [loadStatus])

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
    } catch (err) {
      setReembedResult(err instanceof Error ? err.message : '再埋め込みに失敗しました')
    } finally {
      setReembedBusy(false)
    }
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="ベクトルDB管理"
        description="Chroma のコレクション件数確認と、企業単位の削除・再埋め込みを行います（#573 Phase 3）。"
        backHref="/admin"
      />

      <Paper elevation={0} sx={{ p: 3, mb: 3, border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
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

      <Paper elevation={0} sx={{ p: 3, border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
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
    </PageContainer>
  )
}
