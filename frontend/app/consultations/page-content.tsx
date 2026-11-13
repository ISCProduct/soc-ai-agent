'use client'

import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Badge,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Container,
  Dialog,
  DialogContent,
  DialogTitle,
  IconButton,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import { fetchActiveGuidances, type StudentGuidance } from '@/lib/teacher-guidance'
import {
  MAX_CONSULTATION_LENGTH,
  askTeacher,
  fetchStudentThread,
  fetchStudentUnread,
  replyAsStudent,
} from '@/lib/consultation'
import { ConsultationThread } from '@/components/consultation/ConsultationThread'

/**
 * 先生への相談（#2028）。先生からの案内に返信したり、自分から質問したりできる。
 * 担当校の先生だけが読める。
 */
export default function ConsultationsPage() {
  const [threads, setThreads] = useState<StudentGuidance[]>([])
  const [unread, setUnread] = useState<Record<string, number>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [openId, setOpenId] = useState<number | null>(null)
  const [question, setQuestion] = useState('')
  const [asking, setAsking] = useState(false)
  const [askError, setAskError] = useState('')

  const refreshUnread = useCallback(() => {
    fetchStudentUnread().then((u) => setUnread(u.by_guidance)).catch(() => {
      // 未読の印は補助なので、取れなくても一覧は出す
    })
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setThreads(await fetchActiveGuidances())
      refreshUnread()
    } catch {
      setError('相談を読み込めませんでした。通信状況を確認して、もう一度お試しください')
    } finally {
      setLoading(false)
    }
  }, [refreshUnread])

  useEffect(() => {
    void load()
    // ホームの案内の「返信する」から ?thread=ID で開く
    const id = Number(new URLSearchParams(window.location.search).get('thread'))
    if (Number.isSafeInteger(id) && id > 0) setOpenId(id)
  }, [load])

  const ask = async () => {
    const body = question.trim()
    if (!body) return
    setAsking(true)
    setAskError('')
    try {
      const thread = await askTeacher(body)
      setQuestion('')
      await load()
      setOpenId(thread.guidance_id)
    } catch (e) {
      setAskError(e instanceof Error ? e.message : '質問を送れませんでした')
    } finally {
      setAsking(false)
    }
  }

  const loadOpen = useCallback(() => fetchStudentThread(openId ?? 0), [openId])
  const sendOpen = useCallback((body: string) => replyAsStudent(openId ?? 0, body), [openId])
  const tooLong = question.length > MAX_CONSULTATION_LENGTH

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <Container maxWidth="md" sx={{ py: 3, pb: { xs: 12, md: 4 } }}>
        <Typography variant="h5" component="h1" fontWeight="bold" gutterBottom>先生への相談</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          進路や就職活動のことを、学校の先生に相談できます。内容はあなたの学校の先生だけが読めます。
        </Typography>

        <Card variant="outlined" sx={{ mb: 3 }}>
          <CardContent>
            <Typography variant="subtitle1" component="h2" fontWeight="bold" gutterBottom>先生に質問する</Typography>
            <TextField
              label="質問の内容"
              multiline
              minRows={3}
              fullWidth
              value={question}
              onChange={(e) => setQuestion(e.target.value)}
              error={tooLong}
              helperText={tooLong ? `${MAX_CONSULTATION_LENGTH}文字以内にしてください（${question.length}文字）` : `${question.length} / ${MAX_CONSULTATION_LENGTH}`}
            />
            {askError && <Alert severity="error" sx={{ mt: 1 }}>{askError}</Alert>}
            <Box sx={{ textAlign: 'right', mt: 1 }}>
              <Button variant="contained" onClick={() => void ask()} disabled={asking || !question.trim() || tooLong}>
                {asking ? '送信中…' : '質問を送る'}
              </Button>
            </Box>
          </CardContent>
        </Card>

        <Typography variant="subtitle1" component="h2" fontWeight="bold" gutterBottom>やり取り</Typography>
        {error && (
          <Alert severity="error" action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>{error}</Alert>
        )}
        {loading && threads.length === 0 && (
          <Stack spacing={1} aria-label="相談を読み込み中">
            {[0, 1].map((i) => <Skeleton key={i} variant="rounded" height={64} />)}
          </Stack>
        )}
        {!loading && !error && threads.length === 0 && (
          <Typography color="text.secondary">まだやり取りはありません。先生からの案内や、あなたの質問がここに表示されます。</Typography>
        )}
        <Stack component="ul" spacing={1} sx={{ listStyle: 'none', p: 0, m: 0 }}>
          {threads.map((t) => {
            const n = unread[String(t.id)] ?? 0
            return (
              <Box component="li" key={t.id}>
                <Card variant="outlined">
                  <CardActionArea onClick={() => setOpenId(t.id)} aria-label={`${t.kind === 'question' ? 'あなたの質問' : '先生からの案内'}を開く${n > 0 ? `（未読 ${n} 件）` : ''}`}>
                    <CardContent sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                      <Badge color="error" badgeContent={n} invisible={n === 0}>
                        <Typography variant="caption" color="primary" sx={{ minWidth: 72 }}>
                          {t.kind === 'question' ? 'あなたの質問' : '先生から'}
                        </Typography>
                      </Badge>
                      <Typography variant="body2" noWrap sx={{ flex: 1, minWidth: 0 }}>{t.message}</Typography>
                    </CardContent>
                  </CardActionArea>
                </Card>
              </Box>
            )
          })}
        </Stack>
      </Container>

      <Dialog open={openId !== null} onClose={() => setOpenId(null)} fullWidth maxWidth="sm">
        <DialogTitle sx={{ pr: 6 }}>
          相談のやり取り
          <IconButton aria-label="閉じる" onClick={() => setOpenId(null)} sx={{ position: 'absolute', right: 8, top: 8 }}>
            <CloseIcon />
          </IconButton>
        </DialogTitle>
        <DialogContent dividers>
          {openId !== null && <ConsultationThread me="student" load={loadOpen} send={sendOpen} onChanged={refreshUnread} />}
        </DialogContent>
      </Dialog>
    </Box>
  )
}
