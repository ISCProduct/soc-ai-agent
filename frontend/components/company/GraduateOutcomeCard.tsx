'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import NextLink from 'next/link'
import { Alert, Box, Button, Card, CardContent, Chip, Link, Skeleton, Stack, Typography } from '@mui/material'
import SchoolIcon from '@mui/icons-material/School'
import { fetchCompanyGraduateOutcome, totalLabel, yearLabel, type GraduateOutcomeCompany } from '@/lib/graduate-outcomes'

/**
 * 企業詳細の「先輩の進路実績」（#2032）。自分の学校の卒業生がこの企業に何人就職したかを、年度・職種ごとの人数で出す。
 * 推薦ではなく事実の提示。実績が無い（または学校が未登録・企業が見つからない）ときはカードごと出さない。
 * 3人未満は人数を出さず「少数」と出す（学内で誰か特定できるため）。
 */
export function GraduateOutcomeCard({ companyId }: { companyId: string }) {
  const [data, setData] = useState<GraduateOutcomeCompany | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  // 企業を続けて移動したとき、前の企業の遅い応答で表示を上書きしない
  const requestSeq = useRef(0)

  const load = useCallback(async () => {
    const seq = ++requestSeq.current
    setLoading(true)
    setFailed(false)
    setData(null)
    try {
      const result = await fetchCompanyGraduateOutcome(companyId)
      if (seq === requestSeq.current) setData(result)
    } catch {
      if (seq === requestSeq.current) setFailed(true)
    } finally {
      if (seq === requestSeq.current) setLoading(false)
    }
  }, [companyId])

  useEffect(() => {
    void load()
  }, [load])

  if (loading) return <Skeleton variant="rounded" height={96} sx={{ mb: 3 }} aria-label="先輩の進路実績を読み込み中" />
  if (failed) {
    return (
      <Alert severity="warning" sx={{ mb: 3 }} action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>
        先輩の進路実績を読み込めませんでした
      </Alert>
    )
  }
  if (!data || !data.school_linked || data.total === 0) return null
  const few = data.total == null

  return (
    <Card sx={{ mb: 3, borderRadius: 2 }} component="section" aria-label="先輩の進路実績">
      <CardContent>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1 }}>
          <SchoolIcon color="primary" fontSize="small" />
          <Typography fontWeight={700}>先輩の進路実績</Typography>
        </Stack>
        <Typography variant="body2" sx={{ mb: 1.5 }}>
          {few ? (
            <>あなたの学校から <strong>少数</strong> の先輩がこの企業に就職しています。</>
          ) : (
            <>あなたの学校から <strong>{totalLabel(data.total, data.total_few)}</strong> の先輩がこの企業に就職しています。</>
          )}
        </Typography>
        {/* 見せる区分が無い（少数だけの）企業は内訳を出さない */}
        {data.by_year.length > 0 && (
          <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 1 }}>
            {data.by_year.map((y) => (
              <Chip key={y.year} size="small" label={`${yearLabel(y.year)} ${y.count}人`} />
            ))}
            {data.by_year_few && <Chip size="small" variant="outlined" label="＋少数" />}
          </Stack>
        )}
        {data.by_position.length > 0 && (
          <Box sx={{ mb: 1 }}>
            <Typography variant="caption" color="text.secondary">職種</Typography>
            <Typography variant="body2">
              {[...data.by_position.map((p) => `${p.title} ${p.count}人`), ...(data.by_position_few ? ['＋少数'] : [])].join('、')}
            </Typography>
          </Box>
        )}
        {few || data.by_year_few || data.by_position_few ? (
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
            卒業生が特定されないよう、年度・職種ごとに3人未満の人数は出さず、合計にも含めていません。
          </Typography>
        ) : null}
        <Link component={NextLink} href="/graduate-outcomes" variant="body2">ほかの企業の進路実績を見る</Link>
      </CardContent>
    </Card>
  )
}
