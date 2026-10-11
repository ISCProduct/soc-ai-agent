'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  FormControl,
  FormControlLabel,
  FormLabel,
  LinearProgress,
  Paper,
  Radio,
  RadioGroup,
  Stack,
  Typography,
} from '@mui/material'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import {
  AssessmentApiError,
  LIKERT_OPTIONS,
  QUESTIONS_PER_PAGE,
  assessmentApi,
  firstUnansweredPage,
  type AssessmentSession,
} from '@/lib/assessments'

type SaveState = 'saving' | 'saved' | 'error'

export default function PageContent() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const router = useRouter()
  const [session, setSession] = useState<AssessmentSession | null>(null)
  const [answers, setAnswers] = useState<Record<string, number>>({})
  const [saveState, setSaveState] = useState<Record<string, SaveState>>({})
  const [page, setPage] = useState(0)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // 同じ問題への保存は直列に送る。後から選んだ値が先に届いて古い値で上書きされるのを防ぐ。
  const queues = useRef<Record<string, Promise<void>>>({})
  const headingRef = useRef<HTMLHeadingElement>(null)

  const load = useCallback(async () => {
    setLoadError(null)
    try {
      const s = await assessmentApi<AssessmentSession>(`/sessions/${sessionId}`)
      if (s.status === 'submitted') {
        router.replace(`/assessments/${sessionId}/result`)
        return
      }
      setSession(s)
      setAnswers(s.answers ?? {})
      setPage(firstUnansweredPage(s))
    } catch (e) {
      setLoadError(
        e instanceof AssessmentApiError && e.status === 404
          ? 'この検査は見つかりませんでした。一覧から選び直してください'
          : '検査を読み込めませんでした',
      )
    }
  }, [sessionId, router])

  useEffect(() => {
    void load()
  }, [load])

  const answer = (questionId: number, value: number) => {
    const key = String(questionId)
    setAnswers((prev) => ({ ...prev, [key]: value }))
    setSaveState((prev) => ({ ...prev, [key]: 'saving' }))
    const run = (queues.current[key] ?? Promise.resolve()).then(async () => {
      try {
        await assessmentApi(`/sessions/${sessionId}/answers/${questionId}`, { method: 'PUT', body: { value } })
        // 後続の保存が待っているなら、その結果を待つ（途中の「保存しました」で誤魔化さない）
        if (queues.current[key] === run) setSaveState((prev) => ({ ...prev, [key]: 'saved' }))
      } catch {
        if (queues.current[key] === run) setSaveState((prev) => ({ ...prev, [key]: 'error' }))
      }
    })
    queues.current[key] = run
  }

  const goTo = (next: number) => {
    setPage(next)
    headingRef.current?.focus()
  }

  const submit = async () => {
    setSubmitting(true)
    setSubmitError(null)
    try {
      await Promise.all(Object.values(queues.current))
      await assessmentApi(`/sessions/${sessionId}/submit`, { method: 'POST' })
      router.push(`/assessments/${sessionId}/result`)
    } catch (e) {
      if (e instanceof AssessmentApiError && e.status === 409) {
        router.push(`/assessments/${sessionId}/result`)
        return
      }
      setSubmitError('提出できませんでした。答えた内容は保存されています。時間をおいてもう一度「提出する」を押してください')
      setSubmitting(false)
    }
  }

  if (loadError) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <Alert
          severity="error"
          action={<Button color="inherit" size="small" onClick={() => router.push('/assessments')}>一覧へ戻る</Button>}
        >
          {loadError}
        </Alert>
      </Box>
    )
  }
  if (!session) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <ListSkeleton rows={2} label="検査を読み込み中" />
      </Box>
    )
  }

  const total = session.questions.length
  const pages = Math.max(1, Math.ceil(total / QUESTIONS_PER_PAGE))
  const visible = session.questions.slice(page * QUESTIONS_PER_PAGE, (page + 1) * QUESTIONS_PER_PAGE)
  const answeredCount = session.questions.filter((q) => answers[String(q.id)] !== undefined).length
  const pageDone = visible.every((q) => answers[String(q.id)] !== undefined)
  const hasSaveError = Object.values(saveState).includes('error')
  const isLast = page === pages - 1

  return (
    <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold" tabIndex={-1} ref={headingRef} sx={{ outline: 'none' }}>
        {session.title}
      </Typography>
      <Typography variant="body2" color="text.secondary" mt={0.5}>
        {page + 1} / {pages} ページ ・ {answeredCount} / {total} 問 回答済み
      </Typography>
      <LinearProgress
        variant="determinate"
        value={total ? (answeredCount / total) * 100 : 0}
        aria-label={`回答済み ${answeredCount} / ${total} 問`}
        sx={{ my: 2, height: 8, borderRadius: 4 }}
      />
      <Typography variant="body2" color="text.secondary" mb={2}>
        正解・不正解はありません。いまの自分に近いものを選んでください。選ぶと自動で保存されます。
      </Typography>

      <Stack spacing={2}>
        {visible.map((q) => {
          const key = String(q.id)
          const state = saveState[key]
          return (
            <Paper key={q.id} variant="outlined" sx={{ p: 2 }}>
              <FormControl component="fieldset" fullWidth>
                <FormLabel component="legend" sx={{ color: 'text.primary', fontWeight: 'bold', mb: 1 }}>
                  問{q.position}. {q.body}
                </FormLabel>
                <RadioGroup
                  value={answers[key] ?? ''}
                  onChange={(e) => answer(q.id, Number(e.target.value))}
                >
                  {LIKERT_OPTIONS.map((o) => (
                    <FormControlLabel key={o.value} value={o.value} control={<Radio />} label={o.label} />
                  ))}
                </RadioGroup>
                <Typography variant="caption" role="status" color={state === 'error' ? 'error' : 'text.secondary'}>
                  {state === 'saving' && '保存しています…'}
                  {state === 'saved' && '✓ 保存しました'}
                  {state === 'error' && '× 保存できませんでした。もう一度選んでください（それまでに答えた内容は保存されています）'}
                </Typography>
              </FormControl>
            </Paper>
          )
        })}
      </Stack>

      {submitError && <Alert severity="error" sx={{ mt: 2 }}>{submitError}</Alert>}

      <Stack direction="row" justifyContent="space-between" mt={3} gap={1}>
        <Button variant="outlined" disabled={page === 0} onClick={() => goTo(page - 1)}>
          前へ
        </Button>
        {isLast ? (
          <Button
            variant="contained"
            disabled={answeredCount < total || hasSaveError || submitting}
            onClick={() => void submit()}
          >
            {submitting ? '提出しています…' : '提出する'}
          </Button>
        ) : (
          <Button variant="contained" disabled={!pageDone} onClick={() => goTo(page + 1)}>
            次へ
          </Button>
        )}
      </Stack>
      {!pageDone && (
        <Typography variant="caption" color="text.secondary" display="block" textAlign="right" mt={1}>
          このページの問題にすべて答えると進めます
        </Typography>
      )}
      <Box mt={3}>
        <Button onClick={() => router.push('/assessments')}>中断して一覧へ戻る（回答は保存済み）</Button>
      </Box>
      <BottomNavSpacer />
    </Box>
  )
}
