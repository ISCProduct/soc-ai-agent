'use client'

import { useEffect, useRef, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { formatDateTime } from '@/lib/i18n/format'
import { interviewApi, type AxisComparisonResult, type InterviewTrendPoint } from '@/lib/interview'
import { UI } from '@/lib/design/tokens'

type AxisComparisonProps = {
  points: InterviewTrendPoint[]
}

function sessionLabel(point: InterviewTrendPoint): string {
  const when = formatDateTime(point.created_at, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    timeZone: 'Asia/Tokyo',
  })
  return `#${point.session_id} ${when}`
}

function formatScore(value: number | null): string {
  if (value == null || Number.isNaN(value)) return '—'
  return Number.isInteger(value) ? String(value) : value.toFixed(1)
}

function formatDelta(value: number | null): string {
  if (value == null || Number.isNaN(value)) return ''
  const text = Number.isInteger(value) ? String(Math.abs(value)) : Math.abs(value).toFixed(1)
  if (value > 0) return `+${text}`
  if (value < 0) return `-${text}`
  return '0'
}

export default function AxisComparison({ points }: AxisComparisonProps) {
  const [baseId, setBaseId] = useState<number | ''>('')
  const [targetId, setTargetId] = useState<number | ''>('')
  const [result, setResult] = useState<AxisComparisonResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [retryToken, setRetryToken] = useState(0)
  const requestGen = useRef(0)

  useEffect(() => {
    if (points.length < 2) {
      setBaseId('')
      setTargetId('')
      setResult(null)
      return
    }
    const older = points[points.length - 2].session_id
    const newer = points[points.length - 1].session_id
    setBaseId((current) => (points.some((point) => point.session_id === current) ? current : older))
    setTargetId((current) => (points.some((point) => point.session_id === current) ? current : newer))
  }, [points])

  useEffect(() => {
    if (baseId === '' || targetId === '' || baseId === targetId) {
      setResult(null)
      setLoading(false)
      return
    }
    const gen = requestGen.current + 1
    requestGen.current = gen
    setLoading(true)
    setError(null)
    void interviewApi.compareAxes(baseId, targetId)
      .then((data) => {
        if (requestGen.current !== gen) return
        setResult(data)
      })
      .catch(() => {
        if (requestGen.current !== gen) return
        setResult(null)
        setError('軸ごとの比較を取得できませんでした')
      })
      .finally(() => {
        if (requestGen.current !== gen) return
        setLoading(false)
      })
  }, [baseId, targetId, retryToken])

  return (
    <Paper elevation={0} sx={{ mt: 2, p: { xs: 2, md: 3 }, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field, overflowWrap: 'anywhere' }}>
      <Typography component="h2" sx={{ fontWeight: 700, fontSize: 16, mb: 0.5 }}>回ごとの比較</Typography>
      <Typography sx={{ fontSize: 13, color: UI.inkSoft, mb: 2 }}>
        2回の面接を並べて、どの軸が伸びたか落ちたかを見ます。スコアはAIの評価で、確定した能力ではありません。上の期間を変えると、選べる回も変わります。
      </Typography>
      {points.length < 2 ? (
        <Typography color="text.secondary" fontSize={14} sx={{ py: 2, textAlign: 'center' }}>
          比較するには、完了した面接が2回以上必要です。
        </Typography>
      ) : (
        <Stack spacing={2}>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
            <TextField
              select
              fullWidth
              label="比較する前の回"
              value={baseId}
              onChange={(event) => setBaseId(Number(event.target.value))}
            >
              {points.map((point) => (
                <MenuItem key={point.session_id} value={point.session_id}>{sessionLabel(point)}</MenuItem>
              ))}
            </TextField>
            <TextField
              select
              fullWidth
              label="比較するあとの回"
              value={targetId}
              onChange={(event) => setTargetId(Number(event.target.value))}
            >
              {points.map((point) => (
                <MenuItem key={point.session_id} value={point.session_id}>{sessionLabel(point)}</MenuItem>
              ))}
            </TextField>
          </Stack>
          {baseId !== '' && baseId === targetId ? (
            <Alert severity="info">異なる2回を選んでください。</Alert>
          ) : null}
          {loading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }} aria-busy="true">
              <CircularProgress size={28} sx={{ color: UI.mark }} aria-label="比較を読み込み中" />
            </Box>
          ) : error ? (
            <Stack spacing={1} alignItems="flex-start">
              <Alert severity="error">{error}</Alert>
              <Button size="small" variant="outlined" onClick={() => setRetryToken((current) => current + 1)}>再試行</Button>
            </Stack>
          ) : result ? (
            <Stack spacing={1} component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
              {result.axes.map((axis) => (
                <Box
                  key={axis.axis}
                  component="li"
                  sx={{
                    display: 'flex',
                    flexDirection: { xs: 'column', sm: 'row' },
                    justifyContent: 'space-between',
                    gap: 1,
                    p: 1.5,
                    borderRadius: 1.5,
                    border: `1px solid ${UI.ruleSoft}`,
                  }}
                >
                  <Typography sx={{ fontWeight: 700, minWidth: 160 }}>{axis.label}</Typography>
                  <Typography sx={{ fontSize: 14, color: UI.ink }}>
                    {formatScore(axis.base)} → {formatScore(axis.target)}
                  </Typography>
                  <Typography sx={{ fontWeight: 700, fontSize: 14 }} data-direction={axis.direction}>
                    {axis.change_label}
                    {axis.delta != null ? `（${formatDelta(axis.delta)}）` : ''}
                  </Typography>
                </Box>
              ))}
            </Stack>
          ) : null}
        </Stack>
      )}
    </Paper>
  )
}
