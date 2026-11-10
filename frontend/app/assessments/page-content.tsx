'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Box, Button, Card, CardContent, Paper, Stack, Typography } from '@mui/material'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import {
  STATUS_DISPLAY,
  assessmentApi,
  formatDueAt,
  type AssessmentSession,
  type AssessmentSummary,
} from '@/lib/assessments'

export default function PageContent() {
  const router = useRouter()
  const [items, setItems] = useState<AssessmentSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [startingId, setStartingId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await assessmentApi<{ assessments: AssessmentSummary[] }>('')
      setItems(data.assessments ?? [])
    } catch {
      setError('検査の一覧を読み込めませんでした')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  const start = async (templateId: number) => {
    setStartingId(templateId)
    setError(null)
    try {
      const s = await assessmentApi<AssessmentSession>(`/${templateId}/sessions`, { method: 'POST' })
      router.push(`/assessments/${s.session_id}`)
    } catch {
      setError('検査を始められませんでした。時間をおいてもう一度お試しください')
      setStartingId(null)
    }
  }

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold" mb={1}>
        適性検査
      </Typography>
      <Typography variant="body2" color="text.secondary" mb={3}>
        質問に「あてはまる〜あてはまらない」で答えると、あなたの仕事の向き方の傾向がわかります。途中でやめても、答えた分は保存されます。
      </Typography>

      {error && !loading && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}
        >
          {error}
        </Alert>
      )}

      {loading ? (
        <ListSkeleton rows={2} label="検査を読み込み中" />
      ) : items.length === 0 && !error ? (
        <Paper>
          <EmptyState
            title="いま受けられる検査はありません"
            description="検査が用意されるとここに表示されます。それまではAI相談で自分の傾向を確かめられます。"
            action={<Button variant="contained" onClick={() => router.push('/')}>AI相談へ</Button>}
          />
        </Paper>
      ) : (
        <Stack spacing={2} component="ul" sx={{ listStyle: 'none', p: 0, m: 0 }}>
          {items.map((a) => {
            const st = STATUS_DISPLAY[a.status]
            return (
              <Card key={a.template_id} variant="outlined" component="li">
                <CardContent>
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start" gap={1} flexWrap="wrap">
                    <Typography variant="h6" component="h2" fontWeight="bold">
                      {a.title}
                    </Typography>
                    <Typography variant="body2" fontWeight="bold" data-testid="assessment-status">
                      <span aria-hidden="true">{st.mark} </span>
                      {st.label}
                    </Typography>
                  </Stack>
                  {a.scope === 'school' && (
                    <Typography variant="body2" color="text.secondary" mt={0.5} data-testid="assessment-school-note">
                      学校が用意した検査です。おすすめ企業の計算には使われません。
                    </Typography>
                  )}
                  {a.due_at && (
                    <Typography variant="body2" fontWeight={a.overdue ? 'bold' : undefined} mt={0.5} data-testid="assessment-due">
                      {a.overdue && <span aria-hidden="true">⚠ </span>}
                      {`受検期限 ${formatDueAt(a.due_at)}`}
                      {a.overdue ? '（期限を過ぎています。今からでも受検できます）' : ''}
                    </Typography>
                  )}
                  {a.description && (
                    <Typography variant="body2" color="text.secondary" mt={0.5}>
                      {a.description}
                    </Typography>
                  )}
                  <Typography variant="body2" mt={1}>
                    {a.status === 'in_progress'
                      ? `${a.question_count}問中 ${a.answered_count}問 回答済み`
                      : `全${a.question_count}問（目安 ${Math.max(1, Math.ceil(a.question_count / 4))}分）`}
                  </Typography>
                  <Box mt={2}>
                    {a.status === 'submitted' && a.session_id ? (
                      <Button variant="outlined" onClick={() => router.push(`/assessments/${a.session_id}/result`)}>
                        結果を見る
                      </Button>
                    ) : a.status === 'in_progress' && a.session_id ? (
                      <Button variant="contained" onClick={() => router.push(`/assessments/${a.session_id}`)}>
                        続きから再開する
                      </Button>
                    ) : (
                      <Button
                        variant="contained"
                        disabled={startingId !== null}
                        onClick={() => void start(a.template_id)}
                      >
                        {startingId === a.template_id ? '準備しています…' : '受検を始める'}
                      </Button>
                    )}
                  </Box>
                </CardContent>
              </Card>
            )
          })}
        </Stack>
      )}
      <BottomNavSpacer />
    </Box>
  )
}
