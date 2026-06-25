'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Collapse,
  Divider,
  IconButton,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import Grid from '@mui/material/GridLegacy'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import ScheduleIcon from '@mui/icons-material/Schedule'
import { authService } from '@/lib/auth'

type CrawlSource = {
  id: number
  name: string
  target_type: string
  source_type?: string
  source_url?: string
  schedule_type: string
  schedule_day: number
  schedule_time: string
  is_active: boolean
  last_run_at?: string
  next_run_at?: string
}

type CrawlRun = {
  id: number
  source_id: number
  status: string
  message?: string
  started_at: string
  ended_at?: string
}

type AiTargetType = 'fetch_info_all' | 'fetch_jobs_all' | 'fetch_persona_all'

const AI_TARGETS: { type: AiTargetType; label: string; description: string; color: string }[] = [
  {
    type: 'fetch_info_all',
    label: '全企業 基本情報取得',
    description:
      'OpenAI WebSearchで登録済み全企業の基本情報（概要・業種・所在地・公式URL等）を自動取得します。すでに取得済みの企業はスキップされます。',
    color: '#1976d2',
  },
  {
    type: 'fetch_jobs_all',
    label: '全企業 求人情報取得',
    description:
      '採用ページ・WantedlyのWebSearchで登録済み全企業の求人情報（職種・給与・勤務地・必要スキル等）を自動取得します。すでに取得済みの企業はスキップされます。',
    color: '#388e3c',
  },
  {
    type: 'fetch_persona_all',
    label: '全企業 人物像分析',
    description:
      '登録済み全企業の求める人物像をAIで分析し、CompanyWeightProfileに保存します。すでに分析済みの企業はスキップされます。',
    color: '#7b1fa2',
  },
]

const WEEKDAY_OPTIONS = [
  { value: 0, label: '日' },
  { value: 1, label: '月' },
  { value: 2, label: '火' },
  { value: 3, label: '水' },
  { value: 4, label: '木' },
  { value: 5, label: '金' },
  { value: 6, label: '土' },
]

export default function AdminCrawlingPage() {
  const [sources, setSources] = useState<CrawlSource[]>([])
  const [runs, setRuns] = useState<CrawlRun[]>([])
  const [error, setError] = useState('')
  const [runningType, setRunningType] = useState<AiTargetType | null>(null)
  const [runningAll, setRunningAll] = useState(false)
  const [scheduleOpen, setScheduleOpen] = useState<AiTargetType | null>(null)
  const [scheduleSaving, setScheduleSaving] = useState(false)

  const [scheduleType, setScheduleType] = useState<'weekly' | 'monthly'>('weekly')
  const [scheduleDay, setScheduleDay] = useState(1)
  const [scheduleTime, setScheduleTime] = useState('09:00')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const loadSources = async () => {
    try {
      const res = await fetch('/api/admin/crawl-sources', {
        headers: authService.getAdminFetchHeaders(),
      })
      const text = await res.text()
      if (!text) return
      const data = JSON.parse(text)
      if (res.ok) setSources(data?.sources || [])
    } catch {
      // バックエンド未起動時は無視
    }
  }

  const loadRuns = async () => {
    try {
      const res = await fetch('/api/admin/crawl-runs', {
        headers: authService.getAdminFetchHeaders(),
      })
      const text = await res.text()
      if (!text) return
      const data = JSON.parse(text)
      if (res.ok) setRuns(data?.runs || [])
    } catch {
      // バックエンド未起動時は無視
    }
  }

  useEffect(() => {
    loadSources()
    loadRuns()
  }, [])

  const getSourceByType = (type: AiTargetType) => sources.find((s) => s.target_type === type)

  const handleRunNow = async (type: AiTargetType) => {
    setError('')
    setRunningType(type)
    try {
      let source = getSourceByType(type)

      if (!source) {
        const target = AI_TARGETS.find((t) => t.type === type)!
        const createRes = await fetch('/api/admin/crawl-sources', {
          method: 'POST',
          headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: target.label,
            target_type: type,
            source_type: 'ai',
            source_url: '',
            schedule_type: 'weekly',
            schedule_day: 1,
            schedule_time: '09:00',
          }),
        })
        if (!createRes.ok) {
          const d = await createRes.json()
          setError(d?.error || '実行準備に失敗しました')
          return
        }
        const d = await createRes.json()
        source = d as CrawlSource
        await loadSources()
      }

      if (!source) return

      const runRes = await fetch(`/api/admin/crawl-sources/${source.id}/run`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!runRes.ok) {
        const d = await runRes.json()
        setError(d?.error || '実行に失敗しました')
        return
      }
      await loadRuns()
    } finally {
      setRunningType(null)
    }
  }

  const handleRunAll = async () => {
    setError('')
    setRunningAll(true)
    for (const target of AI_TARGETS) {
      await handleRunNow(target.type)
    }
    setRunningAll(false)
  }

  const handleOpenSchedule = (type: AiTargetType) => {
    if (scheduleOpen === type) {
      setScheduleOpen(null)
      return
    }
    const existing = getSourceByType(type)
    if (existing) {
      setScheduleType(existing.schedule_type as 'weekly' | 'monthly')
      setScheduleDay(existing.schedule_day)
      setScheduleTime(existing.schedule_time)
    } else {
      setScheduleType('weekly')
      setScheduleDay(1)
      setScheduleTime('09:00')
    }
    setScheduleOpen(type)
  }

  const handleSaveSchedule = async (type: AiTargetType) => {
    setError('')
    setScheduleSaving(true)
    try {
      const target = AI_TARGETS.find((t) => t.type === type)!
      const existing = getSourceByType(type)

      if (existing) {
        await fetch(`/api/admin/crawl-sources/${existing.id}`, {
          method: 'PUT',
          headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            schedule_type: scheduleType,
            schedule_day: scheduleDay,
            schedule_time: scheduleTime,
            is_active: true,
          }),
        })
      } else {
        await fetch('/api/admin/crawl-sources', {
          method: 'POST',
          headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: target.label,
            target_type: type,
            source_type: 'ai',
            source_url: '',
            schedule_type: scheduleType,
            schedule_day: scheduleDay,
            schedule_time: scheduleTime,
          }),
        })
      }
      await loadSources()
      setScheduleOpen(null)
    } finally {
      setScheduleSaving(false)
    }
  }

  const handleToggleActive = async (source: CrawlSource) => {
    await fetch(`/api/admin/crawl-sources/${source.id}`, {
      method: 'PUT',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_active: !source.is_active }),
    })
    await loadSources()
  }

  return (
    <Box sx={{ p: 4, maxWidth: 1100, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1 }}>
        <IconButton component={Link} href="/admin">
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h4" fontWeight="bold">
          AI情報取得管理
        </Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
        OpenAI WebSearchを使って登録済み企業の基本情報・求人情報・人物像を自動取得します。
        スケジュール設定で定期実行が可能です。
      </Typography>
      <Button
        variant="contained"
        size="large"
        onClick={handleRunAll}
        disabled={runningAll || runningType !== null}
        sx={{ mb: 3, bgcolor: '#1a1a2e', '&:hover': { bgcolor: '#16213e' } }}
      >
        {runningAll
          ? `全て実行中... (${AI_TARGETS.findIndex(t => t.type === runningType) + 1}/${AI_TARGETS.length})`
          : '全て今すぐ実行'}
      </Button>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError('')}>
          {error}
        </Alert>
      )}

      <Grid container spacing={2} sx={{ mb: 3 }}>
        {AI_TARGETS.map((target) => {
          const source = getSourceByType(target.type)
          const isRunning = runningType === target.type
          const isScheduleOpen = scheduleOpen === target.type

          return (
            <Grid item xs={12} md={4} key={target.type}>
              <Card sx={{ height: '100%', borderTop: `4px solid ${target.color}` }}>
                <CardContent>
                  <Typography variant="h6" fontWeight="bold" gutterBottom>
                    {target.label}
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2, minHeight: 72 }}>
                    {target.description}
                  </Typography>

                  {source && (
                    <Box sx={{ mb: 2, p: 1.5, bgcolor: '#f9f9f9', borderRadius: 1 }}>
                      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                        <Chip
                          label={source.is_active ? '自動実行ON' : '自動実行OFF'}
                          size="small"
                          color={source.is_active ? 'success' : 'default'}
                        />
                        <Typography variant="caption" color="text.secondary">
                          {source.schedule_type === 'weekly'
                            ? `毎週${WEEKDAY_OPTIONS.find((d) => d.value === source.schedule_day)?.label} ${source.schedule_time}`
                            : `毎月${source.schedule_day}日 ${source.schedule_time}`}
                        </Typography>
                      </Stack>
                      <Typography variant="caption" color="text.secondary" component="div">
                        次回:{' '}
                        {source.next_run_at
                          ? new Date(source.next_run_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' })
                          : '未設定'}
                      </Typography>
                      <Typography variant="caption" color="text.secondary" component="div">
                        前回:{' '}
                        {source.last_run_at
                          ? new Date(source.last_run_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' })
                          : '未実行'}
                      </Typography>
                    </Box>
                  )}

                  <Stack spacing={1}>
                    <Button
                      variant="contained"
                      startIcon={<PlayArrowIcon />}
                      onClick={() => handleRunNow(target.type)}
                      disabled={isRunning}
                      sx={{
                        bgcolor: target.color,
                        '&:hover': { bgcolor: target.color, filter: 'brightness(0.88)' },
                      }}
                    >
                      {isRunning ? '実行中...' : '今すぐ実行'}
                    </Button>
                    <Button
                      variant="outlined"
                      startIcon={<ScheduleIcon />}
                      onClick={() => handleOpenSchedule(target.type)}
                      size="small"
                    >
                      {source ? 'スケジュール変更' : 'スケジュール設定'}
                    </Button>
                    {source && (
                      <Button
                        size="small"
                        variant="text"
                        color={source.is_active ? 'error' : 'success'}
                        onClick={() => handleToggleActive(source)}
                      >
                        {source.is_active ? '自動実行を停止' : '自動実行を再開'}
                      </Button>
                    )}
                  </Stack>

                  <Collapse in={isScheduleOpen}>
                    <Box sx={{ mt: 2, pt: 2, borderTop: '1px solid #eee' }}>
                      <Stack spacing={1.5}>
                        <TextField
                          select
                          label="頻度"
                          value={scheduleType}
                          onChange={(e) => setScheduleType(e.target.value as 'weekly' | 'monthly')}
                          size="small"
                        >
                          <MenuItem value="weekly">毎週</MenuItem>
                          <MenuItem value="monthly">毎月</MenuItem>
                        </TextField>
                        {scheduleType === 'weekly' ? (
                          <TextField
                            select
                            label="曜日"
                            value={scheduleDay}
                            onChange={(e) => setScheduleDay(Number(e.target.value))}
                            size="small"
                          >
                            {WEEKDAY_OPTIONS.map((opt) => (
                              <MenuItem key={opt.value} value={opt.value}>
                                {opt.label}
                              </MenuItem>
                            ))}
                          </TextField>
                        ) : (
                          <TextField
                            type="number"
                            label="日付"
                            value={scheduleDay}
                            onChange={(e) => setScheduleDay(Number(e.target.value))}
                            size="small"
                            inputProps={{ min: 1, max: 31 }}
                          />
                        )}
                        <TextField
                          type="time"
                          label="実行時刻"
                          value={scheduleTime}
                          onChange={(e) => setScheduleTime(e.target.value)}
                          size="small"
                          InputLabelProps={{ shrink: true }}
                        />
                        <Stack direction="row" spacing={1}>
                          <Button
                            variant="contained"
                            size="small"
                            onClick={() => handleSaveSchedule(target.type)}
                            disabled={scheduleSaving}
                          >
                            {scheduleSaving ? '保存中...' : '保存'}
                          </Button>
                          <Button
                            variant="text"
                            size="small"
                            onClick={() => setScheduleOpen(null)}
                          >
                            キャンセル
                          </Button>
                        </Stack>
                      </Stack>
                    </Box>
                  </Collapse>
                </CardContent>
              </Card>
            </Grid>
          )
        })}
      </Grid>

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            実行履歴
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={1}>
            {runs.length === 0 && (
              <Typography variant="body2" color="text.secondary">
                まだ実行履歴がありません。
              </Typography>
            )}
            {runs.map((run) => {
              const src = sources.find((s) => s.id === run.source_id)
              const target = AI_TARGETS.find((t) => t.type === src?.target_type)
              return (
                <Box
                  key={run.id}
                  sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}
                >
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Chip
                      label={target?.label ?? `source #${run.source_id}`}
                      size="small"
                      variant="outlined"
                    />
                    <Chip
                      label={run.status}
                      size="small"
                      color={
                        run.status === 'success'
                          ? 'success'
                          : run.status === 'running'
                          ? 'info'
                          : 'default'
                      }
                    />
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    {new Date(run.started_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' })}
                  </Typography>
                </Box>
              )
            })}
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
