'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Box,
  Button,
  Chip,
  CircularProgress,
  IconButton,
  Paper,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import PsychologyIcon from '@mui/icons-material/Psychology'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts'
import { authService, User } from '@/lib/auth'
import { interviewApi, InterviewDetail, InterviewSession, InterviewTrendPoint, TeacherReport } from '@/lib/interview'
import InterviewSummary from '../components/InterviewSummary'
import { parseJsonSafe } from '@/lib/interview-utils'

const PRIMARY = '#ec5b13'

const statusLabel = (s: string) => {
  if (s === 'finished') return <Chip label="完了" color="success" size="small" />
  if (s === 'in_progress') return <Chip label="進行中" color="warning" size="small" />
  return <Chip label="未開始" size="small" />
}

export default function InterviewHistoryPage() {
  const router = useRouter()
  const [user, setUser] = useState<User | null>(null)
  const [sessions, setSessions] = useState<InterviewSession[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [selectedDetail, setSelectedDetail] = useState<InterviewDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [trendPoints, setTrendPoints] = useState<InterviewTrendPoint[]>([])
  const [trendLimit, setTrendLimit] = useState<number>(10)
  const [trendLoading, setTrendLoading] = useState(false)

  const limit = 10

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) { router.replace('/login'); return }
    setUser(storedUser)
  }, [router])

  useEffect(() => {
    if (!user) return
    setLoading(true)
    interviewApi.listSessions(user.user_id, page, limit)
      .then(data => { setSessions(data.sessions); setTotal(data.total) })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [user, page])

  useEffect(() => {
    if (!user) return
    setTrendLoading(true)
    interviewApi.getTrend(user.user_id, trendLimit)
      .then(points => setTrendPoints(points))
      .catch(() => setTrendPoints([]))
      .finally(() => setTrendLoading(false))
  }, [user, trendLimit])

  const isTeacher = user?.role === 'teacher'

  const handleSelectSession = async (session: InterviewSession) => {
    if (!user) return
    setDetailLoading(true)
    setSelectedDetail(null)
    try {
      const detail = await interviewApi.getDetail(session.id, user.user_id, user.role)
      setSelectedDetail(detail)
    } catch { /* ignore */ }
    finally { setDetailLoading(false) }
  }

  const totalPages = Math.ceil(total / limit)

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f8f6f6' }}>
      {/* Header */}
      <Box component="header" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', px: { xs: 2, sm: 3, lg: 8 }, py: 2, bgcolor: '#fff', borderBottom: '1px solid #e2e8f0' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ color: PRIMARY }}><PsychologyIcon sx={{ fontSize: 32 }} /></Box>
          <Typography sx={{ fontWeight: 700, fontSize: { xs: 16, sm: 20 }, color: '#0f172a' }}>面接履歴</Typography>
          {user?.role === 'teacher' && (
            <Chip label="教員モード" size="small" sx={{ bgcolor: '#3b82f6', color: '#fff', fontWeight: 700, ml: 1 }} />
          )}
        </Box>
        <IconButton onClick={() => router.push('/interview')} sx={{ bgcolor: '#f1f5f9', color: '#475569' }}>
          <ArrowBackIcon />
        </IconButton>
      </Box>

      {/* Trend chart */}
      <Box sx={{ maxWidth: 1100, mx: 'auto', px: { xs: 2, md: 4 }, pt: { xs: 2, md: 4 }, pb: 0 }}>
        <Paper elevation={0} sx={{ p: { xs: 2, md: 3 }, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }} flexWrap="wrap" gap={1}>
            <Typography sx={{ fontWeight: 700, fontSize: 16 }}>パフォーマンストレンド</Typography>
            <ToggleButtonGroup
              value={trendLimit}
              exclusive
              onChange={(_, v) => { if (v !== null) setTrendLimit(v) }}
              size="small"
            >
              <ToggleButton value={5} sx={{ fontSize: 12, px: 1.5 }}>直近5回</ToggleButton>
              <ToggleButton value={10} sx={{ fontSize: 12, px: 1.5 }}>直近10回</ToggleButton>
              <ToggleButton value={0} sx={{ fontSize: 12, px: 1.5 }}>全期間</ToggleButton>
            </ToggleButtonGroup>
          </Stack>
          {trendLoading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <CircularProgress size={28} sx={{ color: PRIMARY }} />
            </Box>
          ) : trendPoints.length === 0 ? (
            <Typography color="text.secondary" fontSize={14} sx={{ py: 3, textAlign: 'center' }}>
              完了した面接がないためトレンドを表示できません。
            </Typography>
          ) : (
            <Box sx={{ height: { xs: 180, md: 240 } }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={trendPoints.map((p, i) => ({
                ...p,
                label: `#${p.session_id}`,
                index: i + 1,
              }))}>
                <CartesianGrid strokeDasharray="3 3" stroke="#f1f5f9" />
                <XAxis dataKey="label" tick={{ fontSize: 12, fill: '#64748b' }} />
                <YAxis domain={[0, 10]} tick={{ fontSize: 12, fill: '#64748b' }} width={28} />
                <Tooltip formatter={(v: number) => v?.toFixed(1)} />
                <Legend wrapperStyle={{ fontSize: 12 }} />
                <Line type="monotone" dataKey="logic" name="論理性" stroke="#ec5b13" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
                <Line type="monotone" dataKey="specificity" name="具体性" stroke="#3b82f6" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
                <Line type="monotone" dataKey="ownership" name="主体性" stroke="#10b981" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
                <Line type="monotone" dataKey="communication" name="コミュニケーション" stroke="#8b5cf6" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
                <Line type="monotone" dataKey="enthusiasm" name="熱意" stroke="#f59e0b" dot={{ r: 3 }} activeDot={{ r: 5 }} connectNulls />
              </LineChart>
            </ResponsiveContainer>
            </Box>
          )}
        </Paper>
      </Box>

      <Box sx={{ display: 'flex', maxWidth: 1100, mx: 'auto', p: { xs: 2, md: 4 }, gap: 3, flexDirection: { xs: 'column', md: 'row' }, alignItems: 'flex-start' }}>
        {/* Session list */}
        <Box sx={{ flex: 1, minWidth: 0 }}>
          <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>セッション一覧</Typography>
          {loading ? (
            <CircularProgress size={32} sx={{ color: PRIMARY }} />
          ) : sessions.length === 0 ? (
            <Typography color="text.secondary">面接履歴がありません。</Typography>
          ) : (
            <Stack spacing={1.5}>
              {sessions.map(s => (
                <Paper
                  key={s.id}
                  elevation={0}
                  onClick={() => handleSelectSession(s)}
                  sx={{
                    p: 2.5, borderRadius: 2, border: '1px solid #e2e8f0', cursor: 'pointer',
                    borderColor: selectedDetail?.session.id === s.id ? PRIMARY : '#e2e8f0',
                    bgcolor: selectedDetail?.session.id === s.id ? `${PRIMARY}05` : '#fff',
                    '&:hover': { borderColor: PRIMARY, bgcolor: `${PRIMARY}05` },
                    transition: 'all 0.15s',
                  }}
                >
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                    <Box>
                      <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                        <Typography sx={{ fontWeight: 600, fontSize: 15 }}>
                          面接セッション #{s.id}
                        </Typography>
                        {statusLabel(s.status)}
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        {s.started_at
                          ? new Date(s.started_at).toLocaleString('ja-JP', { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Tokyo' })
                          : new Date(s.created_at).toLocaleString('ja-JP', { year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Tokyo' })}
                      </Typography>
                    </Box>
                    <Box sx={{ textAlign: 'right' }}>
                      <Typography sx={{ fontSize: 13, fontWeight: 600, color: '#475569' }}>
                        ${s.estimated_cost_usd.toFixed(3)}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">推定コスト</Typography>
                    </Box>
                  </Stack>
                </Paper>
              ))}
            </Stack>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <Stack direction="row" spacing={1} justifyContent="center" sx={{ mt: 3 }}>
              <Button size="small" disabled={page === 1} onClick={() => setPage(p => p - 1)}>前へ</Button>
              <Typography sx={{ lineHeight: '32px', fontSize: 13, color: '#64748b' }}>{page} / {totalPages}</Typography>
              <Button size="small" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>次へ</Button>
            </Stack>
          )}
        </Box>

        {/* Detail panel */}
        <Box sx={{ width: { xs: '100%', md: 420 }, flexShrink: 0 }}>
          <Typography variant="h6" sx={{ fontWeight: 700, mb: 2 }}>詳細・レポート</Typography>
          {detailLoading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <CircularProgress size={32} sx={{ color: PRIMARY }} />
            </Box>
          ) : !selectedDetail ? (
            <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
              <Typography color="text.secondary" fontSize={14}>左のセッションを選択すると詳細が表示されます。</Typography>
            </Paper>
          ) : (
            <Stack spacing={2}>
              {/* Report */}
              {selectedDetail.report ? (
                <>
                  <InterviewSummary report={selectedDetail.report} userId={user?.user_id} theme="light" />

                  {/* 教員用レポートパネル */}
                  {isTeacher && (() => {
                    const tr: TeacherReport | null = parseJsonSafe(selectedDetail.report?.teacher_report_json) as TeacherReport | null
                    if (!tr) return null
                    return (
                      <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: '2px solid #3b82f6', bgcolor: '#eff6ff' }}>
                        <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
                          <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: '#3b82f6' }} />
                          <Typography sx={{ fontWeight: 700, color: '#1d4ed8', fontSize: 14 }}>教員用詳細レポート</Typography>
                        </Stack>
                        {tr.overall_comment && (
                          <Box sx={{ mb: 1.5 }}>
                            <Typography variant="caption" sx={{ fontWeight: 700, color: '#1d4ed8', display: 'block', mb: 0.5 }}>総評</Typography>
                            <Typography variant="body2" sx={{ color: '#1e3a8a', lineHeight: 1.7 }}>{tr.overall_comment}</Typography>
                          </Box>
                        )}
                        {tr.coaching_points?.length > 0 && (
                          <Box sx={{ mb: 1.5 }}>
                            <Typography variant="caption" sx={{ fontWeight: 700, color: '#1d4ed8', display: 'block', mb: 0.5 }}>指導ポイント</Typography>
                            <Stack spacing={0.5}>
                              {tr.coaching_points.map((p, i) => (
                                <Typography key={i} variant="body2" sx={{ color: '#1e3a8a', pl: 1 }}>・{p}</Typography>
                              ))}
                            </Stack>
                          </Box>
                        )}
                        {tr.next_steps?.length > 0 && (
                          <Box sx={{ mb: 1.5 }}>
                            <Typography variant="caption" sx={{ fontWeight: 700, color: '#1d4ed8', display: 'block', mb: 0.5 }}>次回に向けた課題</Typography>
                            <Stack spacing={0.5}>
                              {tr.next_steps.map((s, i) => (
                                <Typography key={i} variant="body2" sx={{ color: '#1e3a8a', pl: 1 }}>・{s}</Typography>
                              ))}
                            </Stack>
                          </Box>
                        )}
                        {tr.detailed_evidence && Object.keys(tr.detailed_evidence).length > 0 && (
                          <Box>
                            <Typography variant="caption" sx={{ fontWeight: 700, color: '#1d4ed8', display: 'block', mb: 0.5 }}>評価根拠（詳細）</Typography>
                            <Stack spacing={0.5}>
                              {Object.entries(tr.detailed_evidence).map(([k, v]) => (
                                <Box key={k}>
                                  <Typography variant="caption" sx={{ fontWeight: 700, color: '#2563eb' }}>{k}: </Typography>
                                  <Typography variant="caption" sx={{ color: '#1e3a8a' }}>{v}</Typography>
                                </Box>
                              ))}
                            </Stack>
                          </Box>
                        )}
                      </Paper>
                    )
                  })()}
                </>
              ) : (
                <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                  <Typography variant="body2" color="text.secondary">
                    {selectedDetail.session.status === 'finished' ? 'レポートを生成中です...' : 'レポートは面接終了後に生成されます。'}
                  </Typography>
                </Paper>
              )}

              {/* Utterances */}
              {selectedDetail.utterances.length > 0 && (
                <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                  <Typography sx={{ fontWeight: 700, mb: 1.5 }}>発話ログ</Typography>
                  <Stack spacing={1} sx={{ maxHeight: 320, overflowY: 'auto' }}>
                    {selectedDetail.utterances.map((u, i) => (
                      <Box key={i} sx={{ display: 'flex', flexDirection: 'column', alignItems: u.role === 'ai' ? 'flex-start' : 'flex-end' }}>
                        <Typography sx={{ fontSize: 10, fontWeight: 700, color: u.role === 'ai' ? '#64748b' : PRIMARY, mb: 0.3 }}>
                          {u.role === 'ai' ? '面接官AI' : 'あなた'}
                        </Typography>
                        <Box sx={{ bgcolor: u.role === 'ai' ? '#f1f5f9' : `${PRIMARY}15`, px: 1.5, py: 0.8, borderRadius: 1.5, maxWidth: '90%' }}>
                          <Typography variant="body2" sx={{ color: '#0f172a', lineHeight: 1.6 }}>{u.text}</Typography>
                        </Box>
                      </Box>
                    ))}
                  </Stack>
                </Paper>
              )}
            </Stack>
          )}
        </Box>
      </Box>
    </Box>
  )
}
