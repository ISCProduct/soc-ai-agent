'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Box, Button, Card, CardContent, Paper, Stack, Typography } from '@mui/material'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import {
  STATUS_DISPLAY,
  abilityApi,
  formatTimeLimit,
  type AbilitySessionView,
  type AbilitySummary,
} from '@/lib/ability'

export default function PageContent() {
  const router = useRouter()
  const [items, setItems] = useState<AbilitySummary[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [startingId, setStartingId] = useState<number | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await abilityApi<{ assessments: AbilitySummary[] }>('')
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
      const s = await abilityApi<AbilitySessionView>(`/${templateId}/sessions`, { method: 'POST' })
      router.push(`/ability-assessments/${s.session_id}`)
    } catch {
      setError('検査を始められませんでした。時間をおいてもう一度お試しください')
      setStartingId(null)
    }
  }

  return (
    <Box sx={{ maxWidth: 800, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold" mb={1}>
        能力検査
      </Typography>
      <Typography variant="body2" color="text.secondary" mb={3}>
        言語・非言語の問題に選択式で答えます。<strong>制限時間があり、時間になると自動的に締め切られます</strong>。
        答えた分は自動で保存され、時間切れでもそこまでの回答で提出できます。
        この結果はおすすめ企業の計算には使いません。
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
            title="いま受けられる能力検査はありません"
            description="検査が用意されるとここに表示されます。それまでは性格検査やAI相談で自分の傾向を確かめられます。"
            action={<Button variant="contained" onClick={() => router.push('/assessments')}>性格検査へ</Button>}
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
                    <Typography variant="body2" fontWeight="bold" data-testid="ability-status">
                      <span aria-hidden="true">{st.mark} </span>
                      {st.label}
                    </Typography>
                  </Stack>
                  {a.description && (
                    <Typography variant="body2" color="text.secondary" mt={0.5}>
                      {a.description}
                    </Typography>
                  )}
                  <Typography variant="body2" mt={1} data-testid="ability-meta">
                    {a.status === 'in_progress'
                      ? `${a.question_count}問中 ${a.answered_count}問 回答済み`
                      : `全${a.question_count}問`}
                    {a.time_limit_seconds ? `／制限時間 ${formatTimeLimit(a.time_limit_seconds)}` : '／制限時間なし'}
                  </Typography>
                  {a.status === 'expired' && (
                    <Alert severity="warning" sx={{ mt: 1.5 }} data-testid="ability-expired-note">
                      制限時間を過ぎたため、この検査は再開できません。答えた分で提出すると結果が出ます。
                    </Alert>
                  )}
                  <Box mt={2}>
                    {a.status === 'submitted' && a.session_id ? (
                      <Button variant="outlined" onClick={() => router.push(`/ability-assessments/${a.session_id}/result`)}>
                        結果を見る
                      </Button>
                    ) : (a.status === 'in_progress' || a.status === 'expired') && a.session_id ? (
                      <Button variant="contained" onClick={() => router.push(`/ability-assessments/${a.session_id}`)}>
                        {a.status === 'expired' ? '提出する' : '続きから再開する'}
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
