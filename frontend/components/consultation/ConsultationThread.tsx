'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, CircularProgress, Stack, TextField, Typography } from '@mui/material'
import { MAX_CONSULTATION_LENGTH, countConsultationChars, type ConsultationThread as Thread } from '@/lib/consultation'

type Props = {
  /** 自分の側。自分の発言を右に寄せる */
  me: 'student' | 'teacher'
  load: () => Promise<Thread>
  /** seenIds は、送信時に画面に出ていた相手の未読の発言の ID（先生の返信で、そのうちのものだけを既読にする） */
  send: (body: string, seenIds: number[]) => Promise<Thread>
  /** 送信・読み込みのあとに呼ぶ（未読数の取り直しなど） */
  onChanged?: () => void
}

function formatTime(iso: string): string {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleString('ja-JP', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
}

/** 保存できた発言を、表示中のスレッドに ID で足す（重複は足さない・ID の順に並べる）。 */
export function mergeSavedMessages(thread: Thread, saved: Thread['messages']): Thread {
  const known = new Set(thread.messages.map((m) => m.id))
  const added = saved.filter((m) => m.id > 0 && !known.has(m.id))
  if (added.length === 0) return thread
  return { ...thread, messages: [...thread.messages, ...added].sort((a, b) => a.id - b.id) }
}

/**
 * 相談のスレッド（#2028）。最初の案内（または質問）と、その後のやり取りを古い順に出し、下で返信する。
 * 学生と先生の両方の画面で使う。別のスレッドを開くときは、呼び出し側で key を変えてインスタンスを分ける
 * （前のスレッドへの返信の完了で、開き直したスレッドの表示を上書きしないように）。
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
  // 別のスレッドに切り替わったあとに、前のスレッドの遅い応答で表示を上書きしない
  const requestSeq = useRef(0)

  const reload = useCallback(async () => {
    const seq = ++requestSeq.current
    setLoading(true)
    setError('')
    try {
      const result = await load()
      if (seq !== requestSeq.current) return
      setThread(result)
      onChangedRef.current?.()
    } catch (e) {
      if (seq === requestSeq.current) setError(e instanceof Error ? e.message : '相談を読み込めませんでした')
    } finally {
      if (seq === requestSeq.current) setLoading(false)
    }
  }, [load])

  useEffect(() => {
    void reload()
  }, [reload])

  const submit = async () => {
    const body = draft.trim()
    // 読み直しの途中に送ると、あとから返った古い取得結果で新しい発言が上書きされるので送らない
    if (!body || !thread || loading) return
    setSending(true)
    setSendError('')
    try {
      const seenIds = thread.messages.filter((m) => m.author_role !== me && !m.read_at).map((m) => m.id)
      const result = await send(body, seenIds)
      // 送信中は入力欄を止めているので、ここで消すのは送った文章だけ
      setDraft('')
      if (result.stale) {
        // 保存はできたが、スレッドの読み直しに失敗した。送信は成功なので、保存した発言を表示中のスレッドに
        // 足してから読み直す（再送させない。読み直しにも失敗したら、スレッドは残して警告を出す）
        setThread((cur) => (cur ? mergeSavedMessages(cur, result.messages) : cur))
        onChangedRef.current?.()
        void reload()
      } else {
        setThread(result)
        onChangedRef.current?.()
      }
    } catch (e) {
      // 失敗しても入力は残す（書き直しをさせない）
      setSendError(e instanceof Error ? e.message : '送れませんでした')
    } finally {
      setSending(false)
    }
  }

  if (loading && !thread) return <Box sx={{ textAlign: 'center', py: 3 }}><CircularProgress size={24} aria-label="相談を読み込み中" /></Box>
  const retry = <Button color="inherit" size="small" onClick={() => void reload()}>再読み込み</Button>
  // 一度も読めていなければエラーだけを出す。表示中のスレッドがあれば残し、最新でないことを知らせる
  if (error && !thread) return <Alert severity="error" action={retry}>{error}</Alert>
  if (!thread) return null

  const opener = thread.kind === 'question' ? 'student' : 'teacher'
  // 学生の質問は最初の発言と本文が同じなので、本文を重ねて出さない
  const showOpener = thread.kind !== 'question'
  const length = countConsultationChars(draft)
  const tooLong = length > MAX_CONSULTATION_LENGTH

  return (
    <Stack spacing={1.5}>
      {error && (
        <Alert severity="warning" action={retry}>
          やり取りを最新にできませんでした。表示が古い可能性があります（{error}）
        </Alert>
      )}
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
        disabled={sending}
        error={tooLong}
        helperText={tooLong ? `${MAX_CONSULTATION_LENGTH}文字以内にしてください（${length}文字）` : `${length} / ${MAX_CONSULTATION_LENGTH}`}
      />
      {sendError && <Alert severity="error">{sendError}</Alert>}
      <Box sx={{ textAlign: 'right' }}>
        <Button variant="contained" onClick={() => void submit()} disabled={sending || loading || !draft.trim() || tooLong}>
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
