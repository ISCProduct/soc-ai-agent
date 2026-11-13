'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, CircularProgress, Stack, TextField, Typography } from '@mui/material'
import { MAX_CONSULTATION_LENGTH, type ConsultationThread as Thread } from '@/lib/consultation'

type Props = {
  /** 自分の側。自分の発言を右に寄せる */
  me: 'student' | 'teacher'
  load: () => Promise<Thread>
  send: (body: string) => Promise<Thread>
  /** 送信・読み込みのあとに呼ぶ（未読数の取り直しなど） */
  onChanged?: () => void
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/**
 * 相談のスレッド（#2028）。最初の案内（または質問）と、その後のやり取りを古い順に出し、下で返信する。
 * 学生と先生の両方の画面で使う。
 */
export function ConsultationThread({ me, load, send, onChanged }: Props) {
  const [thread, setThread] = useState<Thread | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [draft, setDraft] = useState('')
  const [sending, setSending] = useState(false)
  const [sendError, setSendError] = useState('')
  // 呼び出し側が毎回新しい関数を渡しても、読み込みを繰り返さないよう ref で持つ
  const onChangedRef = useRef(onChanged)
  useEffect(() => {
    onChangedRef.current = onChanged
  }, [onChanged])

  const reload = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      setThread(await load())
      onChangedRef.current?.()
    } catch (e) {
      setError(e instanceof Error ? e.message : '相談を読み込めませんでした')
    } finally {
      setLoading(false)
    }
  }, [load])

  useEffect(() => {
    void reload()
  }, [reload])

  const submit = async () => {
    const body = draft.trim()
    if (!body) return
    setSending(true)
    setSendError('')
    try {
      setThread(await send(body))
      setDraft('')
      onChangedRef.current?.()
    } catch (e) {
      // 失敗しても入力は残す（書き直しをさせない）
      setSendError(e instanceof Error ? e.message : '送れませんでした')
    } finally {
      setSending(false)
    }
  }

  if (loading && !thread) return <Box sx={{ textAlign: 'center', py: 3 }}><CircularProgress size={24} aria-label="相談を読み込み中" /></Box>
  if (error) {
    return <Alert severity="error" action={<Button color="inherit" size="small" onClick={() => void reload()}>再読み込み</Button>}>{error}</Alert>
  }
  if (!thread) return null

  const opener = thread.kind === 'question' ? 'student' : 'teacher'
  // 学生の質問は最初の発言と本文が同じなので、本文を重ねて出さない
  const showOpener = thread.kind !== 'question'
  const tooLong = draft.length > MAX_CONSULTATION_LENGTH

  return (
    <Stack spacing={1.5}>
      <Stack spacing={1} component="ol" sx={{ listStyle: 'none', p: 0, m: 0 }} aria-label="やり取り">
        {showOpener && <Bubble mine={opener === me} who={opener === 'teacher' ? '先生' : 'あなた'} body={thread.message} at={thread.created_at} />}
        {thread.messages.map((m) => (
          <Bubble
            key={m.id}
            mine={m.author_role === me}
            who={m.author_role === me ? (me === 'student' ? 'あなた' : '自分') : m.author_role === 'teacher' ? '先生' : '学生'}
            body={m.body}
            at={m.created_at}
          />
        ))}
      </Stack>
      <TextField
        label={me === 'student' ? '先生への返信' : '学生への返信'}
        multiline
        minRows={2}
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        error={tooLong}
        helperText={tooLong ? `${MAX_CONSULTATION_LENGTH}文字以内にしてください（${draft.length}文字）` : `${draft.length} / ${MAX_CONSULTATION_LENGTH}`}
      />
      {sendError && <Alert severity="error">{sendError}</Alert>}
      <Box sx={{ textAlign: 'right' }}>
        <Button variant="contained" onClick={() => void submit()} disabled={sending || !draft.trim() || tooLong}>
          {sending ? '送信中…' : '送信する'}
        </Button>
      </Box>
    </Stack>
  )
}

function Bubble({ mine, who, body, at }: { mine: boolean; who: string; body: string; at: string }) {
  return (
    <Box component="li" sx={{ display: 'flex', justifyContent: mine ? 'flex-end' : 'flex-start' }}>
      <Box sx={{ maxWidth: '85%', px: 1.5, py: 1, borderRadius: 2, bgcolor: mine ? 'action.selected' : 'action.hover', border: 1, borderColor: mine ? 'primary.light' : 'divider' }}>
        <Typography variant="caption" color="text.secondary">{who}・{formatTime(at)}</Typography>
        <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{body}</Typography>
      </Box>
    </Box>
  )
}
