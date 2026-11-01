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
  AbilityApiError,
  QUESTIONS_PER_PAGE,
  SECTION_LABEL,
  abilityApi,
  firstUnansweredPage,
  formatRemaining,
  type AbilitySessionView,
} from '@/lib/ability'

type SaveState = 'saving' | 'saved' | 'error'

export default function PageContent() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const router = useRouter()
  const [session, setSession] = useState<AbilitySessionView | null>(null)
  const [answers, setAnswers] = useState<Record<string, number>>({})
  const [saveState, setSaveState] = useState<Record<string, SaveState>>({})
  const [page, setPage] = useState(0)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [submitError, setSubmitError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  // 締め切りを過ぎたことに気づいた状態。以降は回答させず、提出だけを促す。
  const [timeUp, setTimeUp] = useState(false)
  // 中断の確認中。保存キューを待っている間に二重で押させない。
  const [leaving, setLeaving] = useState(false)
  const [remaining, setRemaining] = useState<number | null>(null)
  // 同じ問題への保存は直列に送る。後から選んだ値が先に届いて古い値で上書きされるのを防ぐ。
  const queues = useRef<Record<string, Promise<void>>>({})
  // 最後の保存が失敗している問題。提出直前に見る（state は submit のクロージャでは古いため ref で持つ）。
  const failed = useRef(new Set<string>())
  // timeUp の最新値。非同期の待機後に読むので、クロージャに閉じ込めた値は使わない。
  const timeUpRef = useRef(false)
  const headingRef = useRef<HTMLHeadingElement>(null)

  const load = useCallback(async () => {
    setLoadError(null)
    try {
      const s = await abilityApi<AbilitySessionView>(`/sessions/${sessionId}`)
      if (s.status === 'submitted') {
        router.replace(`/ability-assessments/${sessionId}/result`)
        return
      }
      setSession(s)
      setAnswers(s.answers ?? {})
      setPage(firstUnansweredPage(s))
      setRemaining(s.remaining_seconds ?? null)
    } catch (e) {
      if (e instanceof AbilityApiError && e.status === 410) {
        // 締め切り済み。問題は読めないが、答えた分で提出はできる。
        timeUpRef.current = true
        setTimeUp(true)
        setLoadError(null)
        setSession(null)
        return
      }
      setLoadError(
        e instanceof AbilityApiError && e.status === 404
          ? 'この検査は見つかりませんでした。一覧から選び直してください'
          : '検査を読み込めませんでした',
      )
    }
  }, [sessionId, router])

  useEffect(() => {
    void load()
  }, [load])

  // 残り時間の表示。サーバが凍結した締め切りから数える。
  // 0 になったら回答を止めるが、判定はサーバが行う（この時計は表示とUIの切り替え用）。
  useEffect(() => {
    if (remaining === null) return
    if (remaining <= 0) {
      timeUpRef.current = true
      setTimeUp(true)
      return
    }
    const id = setInterval(() => {
      setRemaining((prev) => (prev === null ? null : Math.max(0, prev - 1)))
    }, 1000)
    return () => clearInterval(id)
  }, [remaining])

  const answer = (questionId: number, choice: number) => {
    const key = String(questionId)
    setAnswers((prev) => ({ ...prev, [key]: choice }))
    setSaveState((prev) => ({ ...prev, [key]: 'saving' }))
    const run = (queues.current[key] ?? Promise.resolve()).then(async () => {
      try {
        await abilityApi(`/sessions/${sessionId}/answers/${questionId}`, { method: 'PUT', body: { choice } })
        // 後続の保存が待っているなら、その結果を待つ（途中の「保存しました」で誤魔化さない）
        if (queues.current[key] === run) {
          failed.current.delete(key)
          setSaveState((prev) => ({ ...prev, [key]: 'saved' }))
        }
      } catch (e) {
        if (queues.current[key] !== run) return
        // 410 は時間切れ。保存の失敗として見せると「選び直せ」と誤解させる。
        if (e instanceof AbilityApiError && e.status === 410) {
          timeUpRef.current = true
          setTimeUp(true)
          setRemaining(0)
          failed.current.delete(key)
          setSaveState((prev) => ({ ...prev, [key]: 'error' }))
          return
        }
        failed.current.add(key)
        setSaveState((prev) => ({ ...prev, [key]: 'error' }))
      }
    })
    queues.current[key] = run
  }

  const goTo = (next: number) => {
    setPage(next)
    headingRef.current?.focus()
  }

  /**
   * 中断して一覧へ戻る。保存の結果を確かめてから移動する。
   * 待たずに遷移すると画面が消え、保存に失敗しても誰も気づけない。
   * 学生は保存済みと思って離れ、再開時にサーバーの古い回答が出る。
   */
  const leave = async () => {
    setLeaving(true)
    setSubmitError(null)
    await drainQueues()
    // 待つ前に読んだ値では判断しない（待っている間に失敗が増えうる）
    if (!timeUpRef.current && failed.current.size > 0) {
      setSubmitError('保存できていない回答があります。「保存できませんでした」と出ている問題を選び直してから中断してください')
      setLeaving(false)
      return
    }
    router.push('/ability-assessments')
  }

  /** 保存キューが空になるまで待つ。待っている間に追加された保存も待つ。 */
  const drainQueues = async () => {
    for (let i = 0; i < 20; i++) {
      const pending = Object.values(queues.current)
      if (pending.length === 0) break
      await Promise.all(pending)
      // 待っている間に新しい保存が積まれていないか確かめる
      if (Object.values(queues.current).every((p) => pending.includes(p))) break
    }
  }

  const submit = async () => {
    setSubmitting(true)
    setSubmitError(null)
    try {
      await drainQueues()
      // 画面で選び直した回答が保存できていないまま提出すると、サーバーに残る古い回答で採点が確定する。
      // 時間切れのときは選び直せないので、この確認は行わない（提出を妨げない）。
      if (!timeUpRef.current && failed.current.size > 0) {
        setSubmitError('保存できていない回答があります。「保存できませんでした」と出ている問題を選び直してから提出してください')
        setSubmitting(false)
        return
      }
      await abilityApi(`/sessions/${sessionId}/submit`, { method: 'POST' })
      router.push(`/ability-assessments/${sessionId}/result`)
    } catch (e) {
      if (e instanceof AbilityApiError && e.status === 409) {
        router.push(`/ability-assessments/${sessionId}/result`)
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
          action={<Button color="inherit" size="small" onClick={() => router.push('/ability-assessments')}>一覧へ戻る</Button>}
        >
          {loadError}
        </Alert>
      </Box>
    )
  }

  // 問題を読めない時間切れ（再開しようとした場合）。提出だけを促す。
  if (timeUp && !session) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <Alert severity="warning" data-testid="ability-time-up">
          制限時間を過ぎました。これまでに答えた内容は保存されています。「提出する」を押すと、その回答で結果が出ます。
        </Alert>
        {submitError && <Alert severity="error" sx={{ mt: 2 }}>{submitError}</Alert>}
        <Stack direction="row" gap={1} mt={2}>
          <Button variant="contained" disabled={submitting} onClick={() => void submit()}>
            {submitting ? '提出しています…' : '提出する'}
          </Button>
          <Button onClick={() => router.push('/ability-assessments')}>一覧へ戻る</Button>
        </Stack>
        <BottomNavSpacer />
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
  const hasSaveError = !timeUp && Object.values(saveState).includes('error')
  const isLast = page === pages - 1

  return (
    <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold" tabIndex={-1} ref={headingRef} sx={{ outline: 'none' }}>
        {session.title}
      </Typography>
      <Stack direction="row" justifyContent="space-between" alignItems="baseline" flexWrap="wrap" gap={1} mt={0.5}>
        <Typography variant="body2" color="text.secondary">
          {page + 1} / {pages} 問目 ・ {answeredCount} / {total} 問 回答済み
        </Typography>
        {remaining !== null && (
          <Typography
            variant="body2"
            role="status"
            fontWeight="bold"
            color={remaining <= 60 ? 'error' : 'text.primary'}
            data-testid="ability-remaining"
          >
            残り時間 {formatRemaining(remaining)}
            {remaining <= 60 && <span> （まもなく締め切られます）</span>}
          </Typography>
        )}
      </Stack>
      <LinearProgress
        variant="determinate"
        value={total ? (answeredCount / total) * 100 : 0}
        aria-label={`回答済み ${answeredCount} / ${total} 問`}
        sx={{ my: 2, height: 8, borderRadius: 4 }}
      />

      {timeUp ? (
        <Alert severity="warning" sx={{ mb: 2 }} data-testid="ability-time-up">
          制限時間を過ぎました。これ以上は回答できません。「提出する」を押すと、ここまでの回答で結果が出ます。
        </Alert>
      ) : (
        <Typography variant="body2" color="text.secondary" mb={2}>
          選択肢から1つ選んでください。選ぶと自動で保存されます。制限時間になると自動的に締め切られます。
        </Typography>
      )}

      <Stack spacing={2}>
        {visible.map((q) => {
          const key = String(q.id)
          const state = saveState[key]
          return (
            <Paper key={q.id} variant="outlined" sx={{ p: 2 }}>
              {/* 提出中・中断確認中は選び直させない。提出のPOSTが新しいPUTを追い越すと、
                  画面に出ている選択と採点された回答が食い違う。 */}
              <FormControl component="fieldset" fullWidth disabled={timeUp || submitting || leaving}>
                <FormLabel component="legend" sx={{ color: 'text.primary', fontWeight: 'bold', mb: 1 }}>
                  <Typography component="span" variant="caption" color="text.secondary" display="block">
                    {SECTION_LABEL[q.section] ?? q.section}
                  </Typography>
                  問{q.position}. {q.body}
                </FormLabel>
                <RadioGroup
                  value={answers[key] ?? ''}
                  onChange={(e) => answer(q.id, Number(e.target.value))}
                >
                  {q.choices.map((label, i) => (
                    <FormControlLabel key={i} value={i} control={<Radio />} label={label} />
                  ))}
                </RadioGroup>
                <Typography variant="caption" role="status" color={state === 'error' ? 'error' : 'text.secondary'}>
                  {state === 'saving' && '保存しています…'}
                  {state === 'saved' && '✓ 保存しました'}
                  {state === 'error' && !timeUp && '× 保存できませんでした。それまでに答えた内容は保存されています'}
                  {state === 'error' && timeUp && '× 制限時間を過ぎたため、この回答は記録されていません'}
                </Typography>
                {/* MUI の RadioGroup は同じ値を選び直しても onChange が出ない。
                    「もう一度選んでください」だけだと、同じ選択肢に戻したい学生は
                    別の選択肢を経由するか再読み込みするまで提出できなくなる。 */}
                {state === 'error' && !timeUp && answers[key] !== undefined && (
                  <Box mt={0.5}>
                    <Button size="small" variant="outlined" disabled={submitting || leaving} onClick={() => answer(q.id, answers[key])}>
                      この回答をもう一度保存する
                    </Button>
                  </Box>
                )}
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
        {isLast || timeUp ? (
          <Button
            variant="contained"
            disabled={hasSaveError || submitting}
            onClick={() => void submit()}
          >
            {submitting ? '提出しています…' : '提出する'}
          </Button>
        ) : (
          <Button variant="contained" onClick={() => goTo(page + 1)}>
            次へ
          </Button>
        )}
      </Stack>
      {!timeUp && !pageDone && !isLast && (
        <Typography variant="caption" color="text.secondary" display="block" textAlign="right" mt={1}>
          わからない問題は飛ばして進めます（未回答は誤答として数えます）
        </Typography>
      )}
      <Box mt={3}>
        <Button disabled={leaving || submitting} onClick={() => void leave()}>
          {leaving ? '保存を確認しています…' : '中断して一覧へ戻る'}
        </Button>
      </Box>
      <BottomNavSpacer />
    </Box>
  )
}
