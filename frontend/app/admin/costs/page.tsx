'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  MenuItem,
  Paper,
  Stack,
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
import { authService } from '@/lib/auth'

type DailyRow = {
  date: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type MonthlyRow = {
  month: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type ModelRow = {
  model: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type Summary = {
  current_month_cost_usd: number
  model_breakdown: ModelRow[]
  realtime?: {
    current_month_cost_usd: number
    active_connections: number
    user_breakdown: RealtimeUserRow[]
  }
}

type RealtimeDailyRow = {
  date: string
  total_cost_usd: number
  total_duration_seconds: number
  session_count: number
  user_count: number
}

type RealtimeUserRow = {
  user_id: number
  total_cost_usd: number
  total_duration_seconds: number
  session_count: number
}

function CostBar({ value, max }: { value: number; max: number }) {
  const pct = max > 0 ? Math.min(100, (value / max) * 100) : 0
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
      <Box sx={{
        flex: 1, height: 8, bgcolor: '#e0e0e0', borderRadius: 4, overflow: 'hidden',
      }}>
        <Box sx={{
          width: `${pct}%`, height: '100%',
          bgcolor: pct > 80 ? '#d32f2f' : pct > 50 ? '#f57c00' : '#1976d2',
          borderRadius: 4, transition: 'width 0.3s',
        }} />
      </Box>
      <Typography variant="body2" sx={{ minWidth: 60, textAlign: 'right' }}>
        ${value.toFixed(4)}
      </Typography>
    </Box>
  )
}

function MiniChart({ data, valueKey, labelKey }: {
  data: any[]
  valueKey: string
  labelKey: string
}) {
  if (data.length === 0) return (
    <Typography color="text.secondary" textAlign="center" py={2} fontSize="0.85rem">
      データなし
    </Typography>
  )
  const max = Math.max(...data.map(d => d[valueKey] as number), 0.0001)
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-end', gap: '2px', height: 80, mt: 1 }}>
      {data.map((row, i) => {
        const h = Math.max(4, ((row[valueKey] as number) / max) * 76)
        return (
          <Box key={i} sx={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <Box
              title={`${row[labelKey]}: $${(row[valueKey] as number).toFixed(4)}`}
              sx={{
                width: '100%', height: h,
                bgcolor: '#1976d2', borderRadius: '2px 2px 0 0',
                cursor: 'default', '&:hover': { bgcolor: '#1565c0' },
              }}
            />
          </Box>
        )
      })}
    </Box>
  )
}

export default function AdminCostsPage() {
  const [adminEmail, setAdminEmail] = useState('')
  const [summary, setSummary] = useState<Summary | null>(null)
  const [daily, setDaily] = useState<DailyRow[]>([])
  const [monthly, setMonthly] = useState<MonthlyRow[]>([])
  const [realtimeDaily, setRealtimeDaily] = useState<RealtimeDailyRow[]>([])
  const [dailyDays, setDailyDays] = useState(30)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
    setAdminEmail(user.email)
  }, [])

  const fetchAll = useCallback(async () => {
    if (!adminEmail) return
    setLoading(true)
    setError('')
    const h = { 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' }
    try {
      const [sumRes, dailyRes, monthlyRes] = await Promise.all([
        fetch('/api/admin/costs', { headers: h }),
        fetch(`/api/admin/costs/daily?days=${dailyDays}`, { headers: h }),
        fetch('/api/admin/costs/monthly?months=12', { headers: h }),
      ])
      const [sumData, dailyData, monthlyData] = await Promise.all([
        sumRes.json(), dailyRes.json(), monthlyRes.json(),
      ])
      setSummary(sumData)
      setDaily(dailyData.daily ?? [])
      setMonthly(monthlyData.monthly ?? [])
      setRealtimeDaily(dailyData.realtime_daily ?? [])
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [adminEmail, dailyDays])

  useEffect(() => { fetchAll() }, [fetchAll])

  const totalDailyCost = daily.reduce((s, r) => s + r.total_cost_usd, 0)
  const maxDailyModel = summary?.model_breakdown?.[0]?.total_cost_usd ?? 0.0001
  const realtimeDailyCost = realtimeDaily.reduce((s, r) => s + r.total_cost_usd, 0)

  return (
    <Box sx={{ p: 4, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={2} mb={3}>
        <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
        <Typography variant="h5" fontWeight={700} flex={1}>
          APIコストモニタリング
        </Typography>
        {loading && <CircularProgress size={20} />}
      </Stack>

      {error && (
        <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* KPI Cards */}
      <Stack direction="row" spacing={2} mb={3} flexWrap="wrap">
        <Card sx={{ flex: 1, minWidth: 200 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">今月の合計コスト</Typography>
            <Typography variant="h4" fontWeight={700} color={
              (summary?.current_month_cost_usd ?? 0) > 50 ? 'error.main' :
              (summary?.current_month_cost_usd ?? 0) > 20 ? 'warning.main' : 'success.main'
            }>
              ${(summary?.current_month_cost_usd ?? 0).toFixed(4)}
            </Typography>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 200 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              過去{dailyDays}日合計コスト
            </Typography>
            <Typography variant="h4" fontWeight={700}>
              ${totalDailyCost.toFixed(4)}
            </Typography>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 200 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">
              過去{dailyDays}日 APIコール数
            </Typography>
            <Typography variant="h4" fontWeight={700}>
              {daily.reduce((s, r) => s + r.call_count, 0).toLocaleString()}
            </Typography>
          </CardContent>
        </Card>

        <Card sx={{ flex: 1, minWidth: 200 }}>
          <CardContent>
            <Typography variant="body2" color="text.secondary">Realtime 今月コスト</Typography>
            <Typography variant="h4" fontWeight={700}>
              ${(summary?.realtime?.current_month_cost_usd ?? 0).toFixed(4)}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              active: {summary?.realtime?.active_connections ?? 0}
            </Typography>
          </CardContent>
        </Card>
      </Stack>

      {/* Daily cost chart */}
      <Paper elevation={1} sx={{ p: 3, mb: 3, borderRadius: 2 }}>
        <Stack direction="row" alignItems="center" justifyContent="space-between" mb={2}>
          <Typography variant="h6" fontWeight={600}>日次コスト推移</Typography>
          <TextField
            select size="small" value={dailyDays}
            onChange={e => setDailyDays(Number(e.target.value))}
            sx={{ width: 120 }}
          >
            <MenuItem value={7}>直近 7 日</MenuItem>
            <MenuItem value={30}>直近 30 日</MenuItem>
            <MenuItem value={90}>直近 90 日</MenuItem>
          </TextField>
        </Stack>
        <MiniChart data={daily} valueKey="total_cost_usd" labelKey="date" />
        <Divider sx={{ my: 2 }} />
        <TableContainer sx={{ maxHeight: 300 }}>
          <Table size="small" stickyHeader>
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>日付</TableCell>
                <TableCell align="right">コスト (USD)</TableCell>
                <TableCell align="right">トークン数</TableCell>
                <TableCell align="right">コール数</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {[...daily].reverse().map(row => (
                <TableRow key={row.date} hover>
                  <TableCell>{row.date}</TableCell>
                  <TableCell align="right">${row.total_cost_usd.toFixed(6)}</TableCell>
                  <TableCell align="right">{row.total_tokens.toLocaleString()}</TableCell>
                  <TableCell align="right">{row.call_count.toLocaleString()}</TableCell>
                </TableRow>
              ))}
              {daily.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ color: 'text.secondary', py: 3 }}>
                    データなし
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Monthly chart */}
      <Paper elevation={1} sx={{ p: 3, mb: 3, borderRadius: 2 }}>
        <Typography variant="h6" fontWeight={600} mb={2}>月次コスト推移（過去12ヶ月）</Typography>
        <MiniChart data={monthly} valueKey="total_cost_usd" labelKey="month" />
        <Divider sx={{ my: 2 }} />
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>月</TableCell>
                <TableCell align="right">コスト (USD)</TableCell>
                <TableCell align="right">トークン数</TableCell>
                <TableCell align="right">コール数</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {[...monthly].reverse().map(row => (
                <TableRow key={row.month} hover>
                  <TableCell>{row.month}</TableCell>
                  <TableCell align="right">${row.total_cost_usd.toFixed(4)}</TableCell>
                  <TableCell align="right">{row.total_tokens.toLocaleString()}</TableCell>
                  <TableCell align="right">{row.call_count.toLocaleString()}</TableCell>
                </TableRow>
              ))}
              {monthly.length === 0 && (
                <TableRow>
                  <TableCell colSpan={4} align="center" sx={{ color: 'text.secondary', py: 3 }}>
                    データなし
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Realtime usage */}
      <Paper elevation={1} sx={{ p: 3, mb: 3, borderRadius: 2 }}>
        <Typography variant="h6" fontWeight={600} mb={1}>Realtime 利用状況</Typography>
        <Typography variant="body2" color="text.secondary" mb={2}>
          過去{dailyDays}日合計: ${realtimeDailyCost.toFixed(4)}
        </Typography>
        <MiniChart data={realtimeDaily} valueKey="total_cost_usd" labelKey="date" />
        <Divider sx={{ my: 2 }} />
        <TableContainer sx={{ maxHeight: 280 }}>
          <Table size="small" stickyHeader>
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>日付</TableCell>
                <TableCell align="right">コスト (USD)</TableCell>
                <TableCell align="right">時間 (分)</TableCell>
                <TableCell align="right">セッション数</TableCell>
                <TableCell align="right">利用ユーザー数</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {[...realtimeDaily].reverse().map(row => (
                <TableRow key={row.date} hover>
                  <TableCell>{row.date}</TableCell>
                  <TableCell align="right">${row.total_cost_usd.toFixed(4)}</TableCell>
                  <TableCell align="right">{(row.total_duration_seconds / 60).toFixed(1)}</TableCell>
                  <TableCell align="right">{row.session_count.toLocaleString()}</TableCell>
                  <TableCell align="right">{row.user_count.toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Realtime users */}
      <Paper elevation={1} sx={{ p: 3, mb: 3, borderRadius: 2 }}>
        <Typography variant="h6" fontWeight={600} mb={2}>Realtime ユーザー別利用（過去30日）</Typography>
        <TableContainer>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f5f5f5' }}>
                <TableCell>User ID</TableCell>
                <TableCell align="right">コスト (USD)</TableCell>
                <TableCell align="right">時間 (分)</TableCell>
                <TableCell align="right">セッション数</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {(summary?.realtime?.user_breakdown ?? []).map(row => (
                <TableRow key={row.user_id} hover>
                  <TableCell>{row.user_id}</TableCell>
                  <TableCell align="right">${row.total_cost_usd.toFixed(4)}</TableCell>
                  <TableCell align="right">{(row.total_duration_seconds / 60).toFixed(1)}</TableCell>
                  <TableCell align="right">{row.session_count.toLocaleString()}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      </Paper>

      {/* Model breakdown */}
      <Paper elevation={1} sx={{ p: 3, borderRadius: 2 }}>
        <Typography variant="h6" fontWeight={600} mb={2}>モデル別コスト内訳（過去30日）</Typography>
        {(summary?.model_breakdown ?? []).length === 0 ? (
          <Typography color="text.secondary" textAlign="center" py={3}>データなし</Typography>
        ) : (
          <Stack spacing={1.5}>
            {(summary?.model_breakdown ?? []).map(row => (
              <Box key={row.model}>
                <Stack direction="row" justifyContent="space-between" mb={0.5}>
                  <Stack direction="row" alignItems="center" spacing={1}>
                    <Chip label={row.model} size="small" variant="outlined" />
                    <Typography variant="caption" color="text.secondary">
                      {row.call_count.toLocaleString()} calls / {row.total_tokens.toLocaleString()} tokens
                    </Typography>
                  </Stack>
                </Stack>
                <CostBar value={row.total_cost_usd} max={maxDailyModel} />
              </Box>
            ))}
          </Stack>
        )}
      </Paper>
    </Box>
  )
}
