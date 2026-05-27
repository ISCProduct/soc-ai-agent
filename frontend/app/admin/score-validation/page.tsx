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
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  Paper,
  Stack,
  Tab,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import AddIcon from '@mui/icons-material/Add'
import { authService } from '@/lib/auth'
import { correlationLabel, correlationColor, formatPercent } from '@/lib/score-validation-utils'

// ── 型定義 ──────────────────────────────────────────────────────────────────

type CorrelationReport = {
  generated_at: string
  total_candidates: number
  categories: Array<{
    category: string
    correlation: number
    p_value: number
    sample_count: number
    avg_score: number
    pass_rate: number
  }>
}

type PhaseMetrics = {
  generated_at: string
  phases: Array<{
    phase: string
    precision: number
    recall: number
    f1_score: number
    sample_count: number
  }>
}

type CalibrationResult = {
  id: number
  triggered_by: string
  sample_count: number
  weights: Record<string, number>
  is_active: boolean
  created_at: string
}

type Variant = {
  id: number
  experiment_name: string
  variant_name: string
  description: string
  traffic_ratio: number
  is_active: boolean
  created_at: string
}

type VariantResult = {
  variant_name: string
  sample_count: number
  avg_score: number
  pass_rate: number
}

// ── サブコンポーネント ────────────────────────────────────────────────────

function CorrelationTab({ headers }: { headers: Record<string, string> }) {
  const [data, setData] = useState<CorrelationReport | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    fetch('/api/admin/score-validation/correlation', { headers })
      .then(r => r.json())
      .then(d => setData(d))
      .catch(() => setError('相関データの取得に失敗しました'))
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <Box textAlign="center" py={6}><CircularProgress /></Box>
  if (error) return <Alert severity="error">{error}</Alert>
  if (!data) return null

  return (
    <Box>
      <Stack direction="row" justifyContent="space-between" alignItems="center" mb={2}>
        <Typography variant="subtitle2" color="text.secondary">
          対象候補者: {data.total_candidates}名 / 生成日時: {new Date(data.generated_at).toLocaleString('ja-JP')}
        </Typography>
      </Stack>
      <TableContainer component={Paper} elevation={0} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow sx={{ bgcolor: '#f5f5f5' }}>
              <TableCell>カテゴリ</TableCell>
              <TableCell align="right">相関係数</TableCell>
              <TableCell>強度</TableCell>
              <TableCell align="right">p値</TableCell>
              <TableCell align="right">サンプル数</TableCell>
              <TableCell align="right">平均スコア</TableCell>
              <TableCell align="right">通過率</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.categories.map(cat => (
              <TableRow key={cat.category} hover>
                <TableCell><Typography fontWeight={500}>{cat.category}</Typography></TableCell>
                <TableCell align="right">
                  <Typography fontWeight={700} color={correlationColor(cat.correlation)}>
                    {cat.correlation.toFixed(3)}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Chip
                    label={correlationLabel(cat.correlation)}
                    size="small"
                    sx={{ bgcolor: correlationColor(cat.correlation) + '20', color: correlationColor(cat.correlation) }}
                  />
                </TableCell>
                <TableCell align="right">
                  <Typography variant="body2" color={cat.p_value < 0.05 ? 'success.main' : 'text.secondary'}>
                    {cat.p_value.toFixed(4)}
                  </Typography>
                </TableCell>
                <TableCell align="right">{cat.sample_count}</TableCell>
                <TableCell align="right">{cat.avg_score.toFixed(2)}</TableCell>
                <TableCell align="right">{formatPercent(cat.pass_rate)}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

function PhaseMetricsTab({ headers }: { headers: Record<string, string> }) {
  const [data, setData] = useState<PhaseMetrics | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    setLoading(true)
    fetch('/api/admin/score-validation/phase-metrics', { headers })
      .then(r => r.json())
      .then(d => setData(d))
      .catch(() => setError('フェーズ別メトリクスの取得に失敗しました'))
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  if (loading) return <Box textAlign="center" py={6}><CircularProgress /></Box>
  if (error) return <Alert severity="error">{error}</Alert>
  if (!data) return null

  return (
    <Box>
      <Typography variant="subtitle2" color="text.secondary" mb={2}>
        生成日時: {new Date(data.generated_at).toLocaleString('ja-JP')}
      </Typography>
      <TableContainer component={Paper} elevation={0} variant="outlined">
        <Table size="small">
          <TableHead>
            <TableRow sx={{ bgcolor: '#f5f5f5' }}>
              <TableCell>フェーズ</TableCell>
              <TableCell align="right">適合率 (Precision)</TableCell>
              <TableCell align="right">再現率 (Recall)</TableCell>
              <TableCell align="right">F1スコア</TableCell>
              <TableCell align="right">サンプル数</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {data.phases.map(ph => (
              <TableRow key={ph.phase} hover>
                <TableCell><Typography fontWeight={500}>{ph.phase}</Typography></TableCell>
                <TableCell align="right">{formatPercent(ph.precision)}</TableCell>
                <TableCell align="right">{formatPercent(ph.recall)}</TableCell>
                <TableCell align="right">
                  <Typography fontWeight={700} color={ph.f1_score >= 0.7 ? 'success.main' : ph.f1_score >= 0.5 ? 'warning.main' : 'error.main'}>
                    {ph.f1_score.toFixed(3)}
                  </Typography>
                </TableCell>
                <TableCell align="right">{ph.sample_count}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    </Box>
  )
}

function ABTestTab({ headers }: { headers: Record<string, string> }) {
  const [variants, setVariants] = useState<Variant[]>([])
  const [results, setResults] = useState<VariantResult[]>([])
  const [selectedExp, setSelectedExp] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [createOpen, setCreateOpen] = useState(false)
  const [form, setForm] = useState({ experiment_name: '', variant_name: '', description: '', traffic_ratio: '0.5' })
  const [submitting, setSubmitting] = useState(false)

  const fetchVariants = useCallback(() => {
    setLoading(true)
    fetch('/api/admin/score-validation/variants', { headers })
      .then(r => r.json())
      .then(d => setVariants(d.experiments ?? []))
      .catch(() => setError('バリアント一覧の取得に失敗しました'))
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { fetchVariants() }, [fetchVariants])

  const fetchResults = useCallback((exp: string) => {
    if (!exp) return
    fetch(`/api/admin/score-validation/variants/results?experiment=${encodeURIComponent(exp)}`, { headers })
      .then(r => r.json())
      .then(d => setResults(d.results ?? []))
      .catch(() => setResults([]))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { if (selectedExp) fetchResults(selectedExp) }, [selectedExp, fetchResults])

  const handleCreate = async () => {
    setSubmitting(true)
    try {
      const res = await fetch('/api/admin/score-validation/variants', {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...form, traffic_ratio: parseFloat(form.traffic_ratio) }),
      })
      if (!res.ok) throw new Error('作成に失敗しました')
      setCreateOpen(false)
      setForm({ experiment_name: '', variant_name: '', description: '', traffic_ratio: '0.5' })
      fetchVariants()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '作成に失敗しました')
    } finally {
      setSubmitting(false)
    }
  }

  const experiments = [...new Set(variants.map(v => v.experiment_name))]

  return (
    <Box>
      {error && <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>{error}</Alert>}
      <Stack direction="row" justifyContent="flex-end" mb={2}>
        <Button variant="contained" startIcon={<AddIcon />} onClick={() => setCreateOpen(true)}>
          バリアント作成
        </Button>
      </Stack>

      {loading ? (
        <Box textAlign="center" py={6}><CircularProgress /></Box>
      ) : (
        <TableContainer component={Paper} elevation={0} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>実験名</TableCell>
                <TableCell>バリアント名</TableCell>
                <TableCell>説明</TableCell>
                <TableCell align="right">トラフィック比率</TableCell>
                <TableCell>ステータス</TableCell>
                <TableCell>作成日</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {variants.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={7} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    バリアントがありません
                  </TableCell>
                </TableRow>
              ) : variants.map(v => (
                <TableRow key={v.id} hover>
                  <TableCell><Typography fontWeight={500}>{v.experiment_name}</Typography></TableCell>
                  <TableCell>{v.variant_name}</TableCell>
                  <TableCell><Typography variant="body2" color="text.secondary">{v.description || '—'}</Typography></TableCell>
                  <TableCell align="right">{formatPercent(v.traffic_ratio)}</TableCell>
                  <TableCell>
                    <Chip label={v.is_active ? '有効' : '無効'} size="small" color={v.is_active ? 'success' : 'default'} />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2">{new Date(v.created_at).toLocaleDateString('ja-JP')}</Typography>
                  </TableCell>
                  <TableCell>
                    <Button size="small" onClick={() => setSelectedExp(v.experiment_name)}>
                      結果確認
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {selectedExp && (
        <Box mt={4}>
          <Typography variant="h6" gutterBottom>
            実験結果: {selectedExp}
          </Typography>
          <TableContainer component={Paper} elevation={0} variant="outlined">
            <Table size="small">
              <TableHead>
                <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                  <TableCell>バリアント</TableCell>
                  <TableCell align="right">サンプル数</TableCell>
                  <TableCell align="right">平均スコア</TableCell>
                  <TableCell align="right">通過率</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {results.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={4} align="center" sx={{ py: 3, color: 'text.secondary' }}>
                      結果データがありません
                    </TableCell>
                  </TableRow>
                ) : results.map(r => (
                  <TableRow key={r.variant_name} hover>
                    <TableCell><Typography fontWeight={500}>{r.variant_name}</Typography></TableCell>
                    <TableCell align="right">{r.sample_count}</TableCell>
                    <TableCell align="right">{r.avg_score.toFixed(2)}</TableCell>
                    <TableCell align="right">{formatPercent(r.pass_rate)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        </Box>
      )}

      <Dialog open={createOpen} onClose={() => setCreateOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>バリアント作成</DialogTitle>
        <DialogContent>
          <Stack spacing={2} mt={1}>
            <TextField
              label="実験名"
              value={form.experiment_name}
              onChange={e => setForm(f => ({ ...f, experiment_name: e.target.value }))}
              fullWidth
              required
            />
            <TextField
              label="バリアント名"
              value={form.variant_name}
              onChange={e => setForm(f => ({ ...f, variant_name: e.target.value }))}
              fullWidth
              required
            />
            <TextField
              label="説明"
              value={form.description}
              onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
              fullWidth
              multiline
              rows={2}
            />
            <TextField
              label="トラフィック比率 (0〜1)"
              type="number"
              value={form.traffic_ratio}
              onChange={e => setForm(f => ({ ...f, traffic_ratio: e.target.value }))}
              inputProps={{ min: 0.01, max: 1, step: 0.05 }}
              fullWidth
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setCreateOpen(false)}>キャンセル</Button>
          <Button
            variant="contained"
            onClick={handleCreate}
            disabled={submitting || !form.experiment_name || !form.variant_name}
          >
            {submitting ? <CircularProgress size={20} /> : '作成'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

function CalibrationTab({ headers }: { headers: Record<string, string> }) {
  const [history, setHistory] = useState<CalibrationResult[]>([])
  const [loading, setLoading] = useState(false)
  const [running, setRunning] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const fetchHistory = useCallback(() => {
    setLoading(true)
    fetch('/api/admin/score-validation/calibration/history', { headers })
      .then(r => r.json())
      .then(d => setHistory(d.history ?? []))
      .catch(() => setError('キャリブレーション履歴の取得に失敗しました'))
      .finally(() => setLoading(false))
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => { fetchHistory() }, [fetchHistory])

  const handleRun = async () => {
    setRunning(true)
    setError('')
    setSuccess('')
    try {
      const res = await fetch('/api/admin/score-validation/calibration/run', {
        method: 'POST',
        headers,
      })
      if (!res.ok) {
        const d = await res.json()
        throw new Error(d?.message ?? 'キャリブレーションの実行に失敗しました')
      }
      setSuccess('キャリブレーションを実行しました')
      fetchHistory()
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'キャリブレーションの実行に失敗しました')
    } finally {
      setRunning(false)
    }
  }

  return (
    <Box>
      {error && <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>{error}</Alert>}
      {success && <Alert severity="success" onClose={() => setSuccess('')} sx={{ mb: 2 }}>{success}</Alert>}

      <Stack direction="row" justifyContent="flex-end" mb={2}>
        <Button
          variant="contained"
          color="warning"
          startIcon={running ? <CircularProgress size={16} color="inherit" /> : <PlayArrowIcon />}
          onClick={handleRun}
          disabled={running}
        >
          キャリブレーション実行
        </Button>
      </Stack>

      {loading ? (
        <Box textAlign="center" py={6}><CircularProgress /></Box>
      ) : (
        <TableContainer component={Paper} elevation={0} variant="outlined">
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>実行日時</TableCell>
                <TableCell>実行者</TableCell>
                <TableCell align="right">サンプル数</TableCell>
                <TableCell>ステータス</TableCell>
                <TableCell>重みの概要</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {history.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                    実行履歴がありません
                  </TableCell>
                </TableRow>
              ) : history.map(h => (
                <TableRow key={h.id} hover>
                  <TableCell>
                    <Typography variant="body2">{new Date(h.created_at).toLocaleString('ja-JP')}</Typography>
                  </TableCell>
                  <TableCell>{h.triggered_by}</TableCell>
                  <TableCell align="right">{h.sample_count}</TableCell>
                  <TableCell>
                    <Chip label={h.is_active ? '適用中' : '旧バージョン'} size="small" color={h.is_active ? 'primary' : 'default'} />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary" sx={{ fontFamily: 'monospace', fontSize: '0.78rem' }}>
                      {Object.entries(h.weights).slice(0, 3).map(([k, v]) => `${k}: ${v.toFixed(2)}`).join(', ')}
                      {Object.keys(h.weights).length > 3 && ' …'}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Box>
  )
}

// ── メインページ ──────────────────────────────────────────────────────────

export default function ScoreValidationPage() {
  const [tab, setTab] = useState(0)
  const [headers, setHeaders] = useState<Record<string, string>>({})

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
    setHeaders({
      'X-Admin-Email': user.email,
      'X-Admin-Token': authService.getStoredToken() || '',
    })
  }, [])

  return (
    <Box sx={{ p: 4, maxWidth: 1200, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={2} mb={3}>
        <IconButton component={Link} href="/admin">
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h5" fontWeight={700} flex={1}>
          スコア精度検証
        </Typography>
      </Stack>

      <Paper elevation={1}>
        <Tabs value={tab} onChange={(_, v) => setTab(v)} sx={{ borderBottom: 1, borderColor: 'divider', px: 2 }}>
          <Tab label="相関分析" />
          <Tab label="フェーズ別メトリクス" />
          <Tab label="A/Bテスト管理" />
          <Tab label="キャリブレーション" />
        </Tabs>
        <Box p={3}>
          {tab === 0 && <CorrelationTab headers={headers} />}
          {tab === 1 && <PhaseMetricsTab headers={headers} />}
          {tab === 2 && <ABTestTab headers={headers} />}
          {tab === 3 && <CalibrationTab headers={headers} />}
        </Box>
      </Paper>
    </Box>
  )
}
