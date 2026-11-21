'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, Skeleton, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { fetchMyCategoryScores, type MyCategoryScore } from '@/lib/axis-match'

/**
 * あなたの10の傾向（SOCAIAGENT-611）。AI との対話から推定した10カテゴリのスコアを、数値と細い棒で出す。
 * 推薦の順位を説明する材料で、良し悪しの評価ではないので、色分けや順位付けはしない（並びは固定）。
 */
export default function MyCategoryScoresCard({ sessionId }: { sessionId: string }) {
  const [scores, setScores] = useState<MyCategoryScore[] | null>(null)
  const [error, setError] = useState(false)
  const seq = useRef(0)

  const load = useCallback(async () => {
    const mine = ++seq.current
    setError(false)
    try {
      const result = await fetchMyCategoryScores(sessionId)
      if (mine === seq.current) setScores(result)
    } catch {
      if (mine === seq.current) setError(true)
    }
  }, [sessionId])

  useEffect(() => {
    void load()
  }, [load])

  const measured = scores?.filter((s) => s.score !== null).length ?? 0

  return (
    <Box component="section" aria-labelledby="my-category-scores-title" sx={{ mb: 4 }}>
      <Typography id="my-category-scores-title" component="h2" sx={{ fontSize: 17, fontWeight: 700, color: UI.ink, mb: 0.5 }}>
        あなたの10の傾向
      </Typography>
      <Typography sx={{ fontSize: 13, color: UI.inkSoft, mb: 1.5, maxWidth: '64ch' }}>
        AI との対話から推定した参考値です（0〜100）。高い・低いに良し悪しはありません。企業ごとの内訳は、各企業の「軸ごとの内訳を見る」から確認できます。
      </Typography>

      {error && (
        <Alert severity="warning" action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>
          傾向のスコアを読み込めませんでした。
        </Alert>
      )}
      {!error && scores === null && (
        <Stack spacing={1} aria-label="傾向のスコアを読み込み中">
          {[0, 1, 2].map((i) => <Skeleton key={i} variant="rounded" height={18} />)}
        </Stack>
      )}
      {!error && scores !== null && measured === 0 && (
        <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>まだスコアがありません。AI との対話を進めると表示されます。</Typography>
      )}
      {!error && scores !== null && measured > 0 && (
        <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0, display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' }, columnGap: 4, rowGap: 0.75, maxWidth: 880 }}>
          {scores.map((s) => (
            <Box component="li" key={s.label} sx={{ display: 'grid', gridTemplateColumns: '10.5em 1fr 3.5em', alignItems: 'center', gap: 1 }}>
              <Typography sx={{ fontSize: 13.5, color: UI.ink }}>{s.label}</Typography>
              <Box
                role="img"
                aria-label={s.score === null ? `${s.label} 未計測` : `${s.label} ${s.score}`}
                sx={{ height: 6, borderRadius: 3, bgcolor: UI.ruleSoft, overflow: 'hidden' }}
              >
                {s.score !== null && <Box sx={{ height: '100%', width: `${Math.max(0, Math.min(100, s.score))}%`, bgcolor: UI.inkFaint }} />}
              </Box>
              <Typography sx={{ fontSize: 13.5, color: s.score === null ? UI.inkFaint : UI.ink, textAlign: 'right' }}>
                {s.score === null ? '未計測' : s.score}
              </Typography>
            </Box>
          ))}
        </Box>
      )}
    </Box>
  )
}
