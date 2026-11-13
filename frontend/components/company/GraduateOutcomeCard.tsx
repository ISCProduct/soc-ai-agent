'use client'

import { useCallback, useEffect, useState } from 'react'
import NextLink from 'next/link'
import { Alert, Box, Button, Card, CardContent, Chip, Link, Skeleton, Stack, Typography } from '@mui/material'
import SchoolIcon from '@mui/icons-material/School'
import { fetchCompanyGraduateOutcome, type GraduateOutcomeCompany } from '@/lib/graduate-outcomes'

/**
 * 企業詳細の「先輩の進路実績」（#2032）。自分の学校の卒業生がこの企業に何人就職したかを、年度・職種ごとの人数で出す。
 * 推薦ではなく事実の提示。実績が無い（または学校が未登録の）ときはカードごと出さない。
 */
export function GraduateOutcomeCard({ companyId }: { companyId: string }) {
  const [data, setData] = useState<GraduateOutcomeCompany | null>(null)
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    setFailed(false)
    try {
      setData(await fetchCompanyGraduateOutcome(companyId))
    } catch {
      setFailed(true)
    } finally {
      setLoading(false)
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

  return (
    <Card sx={{ mb: 3, borderRadius: 2 }} component="section" aria-label="先輩の進路実績">
      <CardContent>
        <Stack direction="row" spacing={1} sx={{ alignItems: 'center', mb: 1 }}>
          <SchoolIcon color="primary" fontSize="small" />
          <Typography fontWeight={700}>先輩の進路実績</Typography>
        </Stack>
        <Typography variant="body2" sx={{ mb: 1.5 }}>
          あなたの学校から <strong>{data.total}人</strong> の先輩がこの企業に就職しています。
        </Typography>
        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 1, mb: 1 }}>
          {data.by_year.map((y) => (
            <Chip key={y.year} size="small" label={`${y.year}年卒 ${y.count}人`} />
          ))}
        </Stack>
        {data.by_position.length > 0 && (
          <Box sx={{ mb: 1 }}>
            <Typography variant="caption" color="text.secondary">職種</Typography>
            <Typography variant="body2">{data.by_position.map((p) => `${p.title} ${p.count}人`).join('、')}</Typography>
          </Box>
        )}
        <Link component={NextLink} href="/graduate-outcomes" variant="body2">ほかの企業の進路実績を見る</Link>
      </CardContent>
    </Card>
  )
}
