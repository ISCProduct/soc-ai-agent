'use client'

import { useCallback, useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { Alert, Box, Button, LinearProgress, Paper, Stack, Typography } from '@mui/material'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { AbilityApiError, SECTION_LABEL, abilityApi, type AbilityResult } from '@/lib/ability'

export default function PageContent() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const router = useRouter()
  const [result, setResult] = useState<AbilityResult | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setError(null)
    try {
      setResult(await abilityApi<AbilityResult>(`/sessions/${sessionId}/result`))
    } catch (e) {
      setError(
        e instanceof AbilityApiError && e.status === 404
          ? 'この結果は見つかりませんでした。まだ提出していない可能性があります'
          : '結果を読み込めませんでした',
      )
    }
  }, [sessionId])

  useEffect(() => {
    void load()
  }, [load])

  if (error) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <Alert
          severity="error"
          action={<Button color="inherit" size="small" onClick={() => router.push('/ability-assessments')}>一覧へ戻る</Button>}
        >
          {error}
        </Alert>
      </Box>
    )
  }
  if (!result) {
    return (
      <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
        <ListSkeleton rows={2} label="結果を読み込み中" />
      </Box>
    )
  }

  return (
    <Box sx={{ maxWidth: 720, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold">
        {result.title} の結果
      </Typography>
      <Typography variant="body2" color="text.secondary" mt={0.5} mb={3}>
        分野ごとの正答数です。
      </Typography>

      {/*
        レーダーチャートは使わない。5項目以上のレーダーは面積が形で誤読される。
        偏差値・順位・他の学生との比較は出さない（この時点の数字が自己認識を固定する）。
      */}
      <Paper variant="outlined" sx={{ p: 2 }}>
        <Stack spacing={2}>
          {result.order.map((name) => {
            const s = result.sections[name]
            if (!s) return null
            const label = SECTION_LABEL[name] ?? name
            const pct = s.total > 0 ? (s.correct / s.total) * 100 : 0
            return (
              <Box key={name} data-testid={`ability-section-${name}`}>
                <Stack direction="row" justifyContent="space-between" alignItems="baseline">
                  <Typography variant="body2">{label}</Typography>
                  <Typography variant="body2" fontWeight="bold">
                    {s.correct} / {s.total} 問
                  </Typography>
                </Stack>
                <LinearProgress
                  variant="determinate"
                  value={Math.min(100, Math.max(0, pct))}
                  aria-label={`${label} ${s.correct} / ${s.total} 問 正解`}
                  sx={{ height: 8, borderRadius: 4 }}
                />
              </Box>
            )
          })}
        </Stack>
      </Paper>

      <Paper variant="outlined" sx={{ p: 2, mt: 3 }} component="section" aria-labelledby="ability-usage">
        <Typography id="ability-usage" variant="h6" component="h2" fontWeight="bold" mb={1}>
          この結果の使われ方
        </Typography>
        <Typography variant="body2" component="ul" sx={{ pl: 2, m: 0, '& li': { mb: 0.5 } }}>
          <li><strong>おすすめ企業の計算には使いません。</strong>言語・非言語の正答数は、性格検査の10項目とは別のものとして扱います。</li>
          <li>答えなかった問題は誤答として数えています。時間内に解けた分だけの記録です。</li>
          <li>この検査は1回ごとの記録です。合否の判定や、他の学生との比較には使いません。</li>
          <li>企業へ個別に開示することはありません。</li>
        </Typography>
      </Paper>

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} mt={3}>
        <Button variant="contained" onClick={() => router.push('/ability-assessments')}>
          検査の一覧へ戻る
        </Button>
        <Button variant="outlined" onClick={() => router.push('/assessments')}>
          性格検査を見る
        </Button>
      </Stack>
      <BottomNavSpacer />
    </Box>
  )
}
