'use client'

import { useCallback, useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Alert, Box, Button, LinearProgress, Paper, Stack, Typography } from '@mui/material'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { assessmentApi, type AssessmentResult } from '@/lib/assessments'

export default function PageContent() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const router = useRouter()
  const [result, setResult] = useState<AssessmentResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      setResult(await assessmentApi<AssessmentResult>(`/sessions/${sessionId}/result`))
    } catch {
      setError('結果を読み込めませんでした')
    }
  }, [sessionId])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold" mb={1}>
        {result ? `${result.title}の結果` : '適性検査の結果'}
      </Typography>

      {error ? (
        <Alert
          severity="error"
          action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}
        >
          {error}
        </Alert>
      ) : !result ? (
        <ListSkeleton rows={3} label="結果を読み込み中" />
      ) : (
        <>
          <Typography variant="body2" color="text.secondary" mb={2}>
            数値は0〜100で、あなたの回答から計算した傾向の強さです。高い・低いに良し悪しはなく、他の人との順位でもありません。
          </Typography>
          <Paper variant="outlined" sx={{ p: 2 }}>
            <Stack spacing={1.5} component="ul" sx={{ listStyle: 'none', p: 0, m: 0 }} aria-label="カテゴリ別の傾向">
              {result.scores.map((s) => (
                <Box component="li" key={s.category}>
                  <Stack direction="row" justifyContent="space-between">
                    <Typography variant="body2">{s.category}</Typography>
                    <Typography variant="body2" fontWeight="bold">
                      {Math.round(s.score)}
                    </Typography>
                  </Stack>
                  <LinearProgress
                    variant="determinate"
                    value={Math.min(100, Math.max(0, s.score))}
                    aria-label={`${s.category} ${Math.round(s.score)} / 100`}
                    sx={{ height: 8, borderRadius: 4 }}
                  />
                </Box>
              ))}
            </Stack>
          </Paper>

          <Paper variant="outlined" sx={{ p: 2, mt: 3 }} component="section" aria-labelledby="assessment-usage">
            <Typography id="assessment-usage" variant="h6" component="h2" fontWeight="bold" mb={1}>
              この結果の使われ方
            </Typography>
            <Typography variant="body2" component="ul" sx={{ pl: 2, m: 0, '& li': { mb: 0.5 } }}>
              <li>AI相談（チャット診断）で分かった傾向と合わせて、おすすめ企業のマッチングに使います。</li>
              <li>AI相談をまだ終えていない場合は、終えた時点でマッチングに反映されます。</li>
              <li>この検査は1回ごとの記録です。合否の判定や、他の学生との比較には使いません。</li>
            </Typography>
          </Paper>

          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} mt={3}>
            <Button variant="contained" onClick={() => router.push(getResultsPathOrChat())}>
              マッチング結果を見る
            </Button>
            <Button variant="outlined" onClick={() => router.push('/assessments')}>
              検査の一覧へ戻る
            </Button>
          </Stack>
        </>
      )}
      <BottomNavSpacer />
    </Box>
  )
}
