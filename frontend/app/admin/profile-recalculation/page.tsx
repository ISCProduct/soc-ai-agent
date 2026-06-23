'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Alert,
  Box,
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
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'

type RecalcResult = {
  company_id: number
  company_name: string
  updated: boolean
  sample_count: number
  message?: string
}

type HistoryEntry = {
  id: number
  company_id: number
  trigger: string
  sample_count: number
  created_at: string
}

export default function AdminProfileRecalculationPage() {
  const [loading, setLoading] = useState(false)
  const [results, setResults] = useState<RecalcResult[]>([])
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [minSamples, setMinSamples] = useState('3')

  const [individualCompanyId, setIndividualCompanyId] = useState('')
  const [individualLoading, setIndividualLoading] = useState(false)
  const [individualResult, setIndividualResult] = useState<RecalcResult | null>(null)

  const [historyCompanyId, setHistoryCompanyId] = useState('')
  const [histories, setHistories] = useState<HistoryEntry[]>([])
  const [historyLoading, setHistoryLoading] = useState(false)

  const [rollbackCompanyId, setRollbackCompanyId] = useState<number | null>(null)
  const [rollbackLoading, setRollbackLoading] = useState(false)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const headers = authService.getAdminFetchHeaders()

  const handleBulkRecalculate = async () => {
    setLoading(true)
    setError('')
    setSuccess('')
    setResults([])
    try {
      const res = await fetch('/api/admin/profile-recalculation', {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ min_samples: parseInt(minSamples) || 3 }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || '一括再計算に失敗しました')
        return
      }
      setResults(data.results || [])
      setSuccess(`完了: ${data.updated_count} 件更新, ${data.skipped_count} 件スキップ (合計 ${data.total} 件)`)
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setLoading(false)
    }
  }

  const handleIndividualRecalculate = async () => {
    if (!individualCompanyId) return
    setIndividualLoading(true)
    setError('')
    setIndividualResult(null)
    try {
      const res = await fetch(`/api/admin/profile-recalculation/${individualCompanyId}`, {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ min_samples: parseInt(minSamples) || 3 }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || '再計算に失敗しました')
        return
      }
      setIndividualResult(data)
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setIndividualLoading(false)
    }
  }

  const handleLoadHistory = async () => {
    if (!historyCompanyId) return
    setHistoryLoading(true)
    setHistories([])
    try {
      const res = await fetch(`/api/admin/profile-recalculation/${historyCompanyId}`, {
        headers,
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || '履歴取得に失敗しました')
        return
      }
      setHistories(data.histories || [])
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setHistoryLoading(false)
    }
  }

  const handleRollback = async () => {
    if (rollbackCompanyId === null) return
    setRollbackLoading(true)
    try {
      const res = await fetch(`/api/admin/profile-recalculation/${rollbackCompanyId}/rollback`, {
        method: 'POST',
        headers,
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data.error || 'ロールバックに失敗しました')
      } else {
        setSuccess(data.message || 'ロールバックしました')
      }
    } catch {
      setError('通信エラーが発生しました')
    } finally {
      setRollbackLoading(false)
      setRollbackCompanyId(null)
    }
  }

  return (
    <Box sx={{ p: 4, maxWidth: 960, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
        <Typography variant="h4" fontWeight="bold">
          プロファイル再計算
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        企業マッチングプロファイルを再計算します。サンプル数が少ない企業はスキップされます。
      </Typography>

      {error && <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>{error}</Alert>}
      {success && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>{success}</Alert>}

      <Stack spacing={3}>
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>一括再計算</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              全企業のプロファイルを一括で再計算します。最小サンプル数以上の企業のみ更新されます。
            </Typography>
            <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
              <TextField
                label="最小サンプル数"
                type="number"
                size="small"
                value={minSamples}
                onChange={(e) => setMinSamples(e.target.value)}
                sx={{ width: 160 }}
              />
              <Button
                variant="contained"
                onClick={handleBulkRecalculate}
                disabled={loading}
                startIcon={loading ? <CircularProgress size={16} /> : undefined}
              >
                一括再計算を実行
              </Button>
            </Stack>
            {results.length > 0 && (
              <>
                <Divider sx={{ mb: 2 }} />
                <Table size="small">
                  <TableHead>
                    <TableRow>
                      <TableCell>企業ID</TableCell>
                      <TableCell>企業名</TableCell>
                      <TableCell>サンプル数</TableCell>
                      <TableCell>状態</TableCell>
                    </TableRow>
                  </TableHead>
                  <TableBody>
                    {results.map((r) => (
                      <TableRow key={r.company_id}>
                        <TableCell>{r.company_id}</TableCell>
                        <TableCell>{r.company_name}</TableCell>
                        <TableCell>{r.sample_count}</TableCell>
                        <TableCell>
                          <Chip
                            label={r.updated ? '更新済み' : 'スキップ'}
                            color={r.updated ? 'success' : 'default'}
                            size="small"
                          />
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>個別再計算</Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              特定企業のプロファイルを再計算します。
            </Typography>
            <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
              <TextField
                label="企業ID"
                type="number"
                size="small"
                value={individualCompanyId}
                onChange={(e) => setIndividualCompanyId(e.target.value)}
                sx={{ width: 160 }}
              />
              <Button
                variant="outlined"
                onClick={handleIndividualRecalculate}
                disabled={individualLoading || !individualCompanyId}
                startIcon={individualLoading ? <CircularProgress size={16} /> : undefined}
              >
                再計算
              </Button>
              <Button
                variant="outlined"
                color="warning"
                onClick={() => setRollbackCompanyId(parseInt(individualCompanyId))}
                disabled={!individualCompanyId}
              >
                ロールバック
              </Button>
            </Stack>
            {individualResult && (
              <Alert severity={individualResult.updated ? 'success' : 'info'}>
                企業ID {individualResult.company_id} ({individualResult.company_name}):
                {individualResult.updated ? ' 更新完了' : ' スキップ'} (サンプル数: {individualResult.sample_count})
              </Alert>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>再計算履歴</Typography>
            <Stack direction="row" spacing={2} alignItems="center" sx={{ mb: 2 }}>
              <TextField
                label="企業ID"
                type="number"
                size="small"
                value={historyCompanyId}
                onChange={(e) => setHistoryCompanyId(e.target.value)}
                sx={{ width: 160 }}
              />
              <Button
                variant="outlined"
                onClick={handleLoadHistory}
                disabled={historyLoading || !historyCompanyId}
                startIcon={historyLoading ? <CircularProgress size={16} /> : undefined}
              >
                履歴を表示
              </Button>
            </Stack>
            {histories.length > 0 && (
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>ID</TableCell>
                    <TableCell>企業ID</TableCell>
                    <TableCell>トリガー</TableCell>
                    <TableCell>サンプル数</TableCell>
                    <TableCell>実行日時</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {histories.map((h) => (
                    <TableRow key={h.id}>
                      <TableCell>{h.id}</TableCell>
                      <TableCell>{h.company_id}</TableCell>
                      <TableCell>{h.trigger}</TableCell>
                      <TableCell>{h.sample_count}</TableCell>
                      <TableCell>{new Date(h.created_at).toLocaleString('ja-JP')}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
            {histories.length === 0 && historyCompanyId && !historyLoading && (
              <Typography variant="body2" color="text.secondary">履歴がありません</Typography>
            )}
          </CardContent>
        </Card>
      </Stack>

      <Dialog open={rollbackCompanyId !== null} onClose={() => setRollbackCompanyId(null)}>
        <DialogTitle>ロールバックの確認</DialogTitle>
        <DialogContent>
          <Typography>
            企業ID {rollbackCompanyId} のプロファイルを直前のバージョンにロールバックします。よろしいですか？
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setRollbackCompanyId(null)}>キャンセル</Button>
          <Button
            color="warning"
            variant="contained"
            onClick={handleRollback}
            disabled={rollbackLoading}
            startIcon={rollbackLoading ? <CircularProgress size={16} /> : undefined}
          >
            ロールバック
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
