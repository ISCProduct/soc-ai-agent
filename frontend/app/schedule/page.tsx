'use client'

import { useEffect, useState, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import ArrowBackIosIcon from '@mui/icons-material/ArrowBackIos'
import ArrowForwardIosIcon from '@mui/icons-material/ArrowForwardIos'
import AddIcon from '@mui/icons-material/Add'
import EditIcon from '@mui/icons-material/Edit'
import DeleteIcon from '@mui/icons-material/Delete'
import FileDownloadIcon from '@mui/icons-material/FileDownload'
import { authService } from '@/lib/auth'

const STAGE_OPTIONS = [
  { value: '書類選考', label: '書類選考', color: '#9c27b0' },
  { value: '1次面接', label: '1次面接', color: '#1976d2' },
  { value: '2次面接', label: '2次面接', color: '#0288d1' },
  { value: '最終面接', label: '最終面接', color: '#e65100' },
  { value: '内定',    label: '内定',    color: '#388e3c' },
  { value: 'その他',  label: 'その他',  color: '#757575' },
]

type ScheduleEvent = {
  id: number
  user_id: number
  company_name: string
  stage: string
  title: string
  scheduled_at: string
  notes: string
  created_at: string
  updated_at: string
}

type EventFormState = {
  company_name: string
  stage: string
  title: string
  scheduled_at: string
  notes: string
}

const EMPTY_FORM: EventFormState = {
  company_name: '',
  stage: '書類選考',
  title: '',
  scheduled_at: '',
  notes: '',
}

function stageColor(stage: string): string {
  return STAGE_OPTIONS.find(s => s.value === stage)?.color ?? '#757575'
}

function isoToDatetimeLocal(iso: string): string {
  if (!iso) return ''
  return iso.slice(0, 16)
}

function datetimeLocalToISO(local: string): string {
  if (!local) return ''
  return new Date(local).toISOString()
}

function getDaysInMonth(year: number, month: number): Date[] {
  const days: Date[] = []
  const date = new Date(year, month, 1)
  while (date.getMonth() === month) {
    days.push(new Date(date))
    date.setDate(date.getDate() + 1)
  }
  return days
}

function getCalendarGrid(year: number, month: number): (Date | null)[][] {
  const days = getDaysInMonth(year, month)
  const firstDay = days[0].getDay()
  const cells: (Date | null)[] = Array(firstDay).fill(null).concat(days)
  while (cells.length % 7 !== 0) cells.push(null)
  const weeks: (Date | null)[][] = []
  for (let i = 0; i < cells.length; i += 7) weeks.push(cells.slice(i, i + 7))
  return weeks
}

const WEEKDAYS = ['日', '月', '火', '水', '木', '金', '土']

export default function SchedulePage() {
  const router = useRouter()
  const [userId, setUserId] = useState<number | null>(null)
  const [events, setEvents] = useState<ScheduleEvent[]>([])
  const [error, setError] = useState<string | null>(null)
  const [viewYear, setViewYear] = useState(new Date().getFullYear())
  const [viewMonth, setViewMonth] = useState(new Date().getMonth())
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<EventFormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    const auth = authService.getStoredUser()
    if (!auth?.user_id) {
      router.push('/login')
      return
    }
    setUserId(auth.user_id)
  }, [router])

  const fetchEvents = useCallback(async () => {
    if (!userId) return
    try {
      const res = await fetch(`/api/schedule?user_id=${userId}`)
      if (!res.ok) throw new Error('スケジュールの取得に失敗しました')
      const data = await res.json()
      setEvents(Array.isArray(data) ? data : [])
    } catch (e: any) {
      setError(e.message)
    }
  }, [userId])

  useEffect(() => {
    fetchEvents()
  }, [fetchEvents])

  const eventsOnDay = (date: Date) => {
    return events.filter(ev => {
      const d = new Date(ev.scheduled_at)
      return d.getFullYear() === date.getFullYear() &&
        d.getMonth() === date.getMonth() &&
        d.getDate() === date.getDate()
    })
  }

  const handlePrevMonth = () => {
    if (viewMonth === 0) { setViewMonth(11); setViewYear(y => y - 1) }
    else setViewMonth(m => m - 1)
  }

  const handleNextMonth = () => {
    if (viewMonth === 11) { setViewMonth(0); setViewYear(y => y + 1) }
    else setViewMonth(m => m + 1)
  }

  const openCreateDialog = (date?: Date) => {
    setEditingId(null)
    setForm({
      ...EMPTY_FORM,
      scheduled_at: date ? `${date.toISOString().slice(0, 10)}T10:00` : '',
    })
    setDialogOpen(true)
  }

  const openEditDialog = (ev: ScheduleEvent) => {
    setEditingId(ev.id)
    setForm({
      company_name: ev.company_name,
      stage: ev.stage,
      title: ev.title,
      scheduled_at: isoToDatetimeLocal(ev.scheduled_at),
      notes: ev.notes,
    })
    setDialogOpen(true)
  }

  const handleSave = async () => {
    if (!userId) return
    setSaving(true)
    setError(null)
    try {
      const payload = {
        ...form,
        scheduled_at: datetimeLocalToISO(form.scheduled_at),
      }
      const url = editingId
        ? `/api/schedule/${editingId}?user_id=${userId}`
        : `/api/schedule?user_id=${userId}`
      const method = editingId ? 'PUT' : 'POST'
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      if (!res.ok) {
        const data = await res.text()
        throw new Error(data || '保存に失敗しました')
      }
      setDialogOpen(false)
      await fetchEvents()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: number) => {
    if (!userId) return
    if (!confirm('このイベントを削除しますか？')) return
    try {
      const res = await fetch(`/api/schedule/${id}?user_id=${userId}`, { method: 'DELETE' })
      if (!res.ok && res.status !== 204) throw new Error('削除に失敗しました')
      await fetchEvents()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleExport = () => {
    if (!userId) return
    window.location.href = `/api/schedule/export?user_id=${userId}`
  }

  const weeks = getCalendarGrid(viewYear, viewMonth)
  const today = new Date()

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f5f5', p: 3 }}>
      <Box sx={{ maxWidth: 900, mx: 'auto' }}>
        {/* Header */}
        <Stack direction="row" alignItems="center" spacing={2} mb={3}>
          <IconButton onClick={() => router.back()}>
            <ArrowBackIcon />
          </IconButton>
          <Typography variant="h5" fontWeight={700} flex={1}>
            選考スケジュール
          </Typography>
          <Button
            variant="outlined"
            startIcon={<FileDownloadIcon />}
            onClick={handleExport}
            size="small"
          >
            .ics エクスポート
          </Button>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={() => openCreateDialog()}
          >
            イベント追加
          </Button>
        </Stack>

        {error && (
          <Alert severity="error" onClose={() => setError(null)} sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        {/* Calendar */}
        <Paper elevation={1} sx={{ borderRadius: 2, overflow: 'hidden', mb: 3 }}>
          {/* Month navigation */}
          <Stack direction="row" alignItems="center" justifyContent="center" sx={{ p: 2, bgcolor: '#fff' }}>
            <IconButton onClick={handlePrevMonth}><ArrowBackIosIcon fontSize="small" /></IconButton>
            <Typography variant="h6" fontWeight={600} sx={{ minWidth: { xs: 120, sm: 160 }, textAlign: 'center' }}>
              {viewYear}年 {viewMonth + 1}月
            </Typography>
            <IconButton onClick={handleNextMonth}><ArrowForwardIosIcon fontSize="small" /></IconButton>
          </Stack>

          <Divider />

          {/* Weekday headers */}
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', bgcolor: '#f9f9f9' }}>
            {WEEKDAYS.map((d, i) => (
              <Box key={d} sx={{
                p: 1, textAlign: 'center',
                color: i === 0 ? '#d32f2f' : i === 6 ? '#1565c0' : 'text.secondary',
                fontWeight: 600, fontSize: '0.85rem',
              }}>
                {d}
              </Box>
            ))}
          </Box>

          <Divider />

          {/* Calendar grid */}
          <Box>
            {weeks.map((week, wi) => (
              <Box key={wi} sx={{ display: 'grid', gridTemplateColumns: 'repeat(7, 1fr)', borderBottom: wi < weeks.length - 1 ? '1px solid #eee' : 'none' }}>
                {week.map((day, di) => {
                  const isToday = day &&
                    day.getFullYear() === today.getFullYear() &&
                    day.getMonth() === today.getMonth() &&
                    day.getDate() === today.getDate()
                  const dayEvents = day ? eventsOnDay(day) : []
                  return (
                    <Box
                      key={di}
                      onClick={() => day && openCreateDialog(day)}
                      sx={{
                        minHeight: 80,
                        p: 0.5,
                        borderRight: di < 6 ? '1px solid #eee' : 'none',
                        bgcolor: day ? '#fff' : '#f9f9f9',
                        cursor: day ? 'pointer' : 'default',
                        '&:hover': day ? { bgcolor: '#f0f4ff' } : {},
                        transition: 'background-color 0.15s',
                      }}
                    >
                      {day && (
                        <>
                          <Box
                            sx={{
                              width: 26, height: 26, borderRadius: '50%', display: 'flex',
                              alignItems: 'center', justifyContent: 'center',
                              bgcolor: isToday ? '#1976d2' : 'transparent',
                              color: isToday ? '#fff' : di === 0 ? '#d32f2f' : di === 6 ? '#1565c0' : 'text.primary',
                              fontSize: '0.85rem', fontWeight: isToday ? 700 : 400,
                              mb: 0.5,
                            }}
                          >
                            {day.getDate()}
                          </Box>
                          <Stack spacing={0.3}>
                            {dayEvents.map(ev => (
                              <Tooltip key={ev.id} title={`${ev.company_name} - ${ev.stage}${ev.title ? ` (${ev.title})` : ''}`}>
                                <Box
                                  onClick={e => { e.stopPropagation(); openEditDialog(ev) }}
                                  sx={{
                                    bgcolor: stageColor(ev.stage),
                                    color: '#fff',
                                    borderRadius: 0.5,
                                    px: 0.5,
                                    fontSize: '0.7rem',
                                    whiteSpace: 'nowrap',
                                    overflow: 'hidden',
                                    textOverflow: 'ellipsis',
                                    cursor: 'pointer',
                                  }}
                                >
                                  {ev.company_name}
                                </Box>
                              </Tooltip>
                            ))}
                          </Stack>
                        </>
                      )}
                    </Box>
                  )
                })}
              </Box>
            ))}
          </Box>
        </Paper>

        {/* Event list */}
        <Typography variant="h6" fontWeight={600} mb={2}>
          {viewYear}年 {viewMonth + 1}月 のイベント一覧
        </Typography>
        {events
          .filter(ev => {
            const d = new Date(ev.scheduled_at)
            return d.getFullYear() === viewYear && d.getMonth() === viewMonth
          })
          .sort((a, b) => new Date(a.scheduled_at).getTime() - new Date(b.scheduled_at).getTime())
          .map(ev => (
            <Paper key={ev.id} elevation={1} sx={{ p: 2, mb: 1.5, borderRadius: 2, borderLeft: `4px solid ${stageColor(ev.stage)}` }}>
              <Stack direction="row" alignItems="flex-start" spacing={2}>
                <Box flex={1}>
                  <Stack direction="row" alignItems="center" spacing={1} mb={0.5}>
                    <Typography fontWeight={600}>{ev.company_name}</Typography>
                    <Chip label={ev.stage} size="small" sx={{ bgcolor: stageColor(ev.stage), color: '#fff', height: 20, fontSize: '0.7rem' }} />
                    {ev.title && <Typography variant="body2" color="text.secondary">{ev.title}</Typography>}
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    {new Date(ev.scheduled_at).toLocaleString('ja-JP', { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                  </Typography>
                  {ev.notes && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                      {ev.notes}
                    </Typography>
                  )}
                </Box>
                <Stack direction="row">
                  <IconButton size="small" onClick={() => openEditDialog(ev)}>
                    <EditIcon fontSize="small" />
                  </IconButton>
                  <IconButton size="small" onClick={() => handleDelete(ev.id)}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </Stack>
              </Stack>
            </Paper>
          ))}

        {events.filter(ev => {
          const d = new Date(ev.scheduled_at)
          return d.getFullYear() === viewYear && d.getMonth() === viewMonth
        }).length === 0 && (
          <Typography color="text.secondary" textAlign="center" py={3}>
            この月のイベントはありません
          </Typography>
        )}
      </Box>

      {/* Create / Edit dialog */}
      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editingId ? 'イベントを編集' : 'イベントを追加'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} mt={1}>
            <TextField
              label="企業名"
              required
              value={form.company_name}
              onChange={e => setForm(f => ({ ...f, company_name: e.target.value }))}
              fullWidth
            />
            <TextField
              select
              label="選考ステージ"
              value={form.stage}
              onChange={e => setForm(f => ({ ...f, stage: e.target.value }))}
              fullWidth
            >
              {STAGE_OPTIONS.map(opt => (
                <MenuItem key={opt.value} value={opt.value}>{opt.label}</MenuItem>
              ))}
            </TextField>
            <TextField
              label="タイトル（任意）"
              value={form.title}
              onChange={e => setForm(f => ({ ...f, title: e.target.value }))}
              fullWidth
            />
            <TextField
              label="日時"
              type="datetime-local"
              required
              value={form.scheduled_at}
              onChange={e => setForm(f => ({ ...f, scheduled_at: e.target.value }))}
              fullWidth
              InputLabelProps={{ shrink: true }}
            />
            <TextField
              label="メモ"
              value={form.notes}
              onChange={e => setForm(f => ({ ...f, notes: e.target.value }))}
              fullWidth
              multiline
              rows={3}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          {editingId && (
            <Button
              color="error"
              onClick={async () => {
                setDialogOpen(false)
                await handleDelete(editingId)
              }}
            >
              削除
            </Button>
          )}
          <Box flex={1} />
          <Button onClick={() => setDialogOpen(false)}>キャンセル</Button>
          <Button variant="contained" onClick={handleSave} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}
