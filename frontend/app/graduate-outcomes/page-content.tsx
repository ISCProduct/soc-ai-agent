'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import NextLink from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Container,
  MenuItem,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { fetchGraduateOutcomes, totalLabel, yearLabel, type GraduateOutcomeList } from '@/lib/graduate-outcomes'

/**
 * 卒業生の進路実績の一覧（#2032）。自分の学校の先輩が、どの企業に何人就職したかを見る。
 * 推薦ではなく事実の提示なので、並びは人数の多い順。卒業生の氏名などは出ない。
 * 3人未満の区分は人数を出さず「少数」と出す（学内で誰か特定できるため。並びも Backend が末尾に回す）。
 */
export default function GraduateOutcomesPage() {
  const [year, setYear] = useState<number | ''>('')
  const [industry, setIndustry] = useState('')
  const [data, setData] = useState<GraduateOutcomeList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // 年度・業界を続けて切り替えたとき、前の条件の遅い応答で表示を上書きしない
  const requestSeq = useRef(0)
  const controller = useRef<AbortController | null>(null)

  const load = useCallback(async () => {
    const seq = ++requestSeq.current
    controller.current?.abort()
    const ac = new AbortController()
    controller.current = ac
    setLoading(true)
    setError('')
    try {
      const result = await fetchGraduateOutcomes({ year: year || undefined, industry: industry || undefined }, ac.signal)
      if (seq === requestSeq.current) setData(result)
    } catch {
      if (seq !== requestSeq.current) return
      setError('進路実績を読み込めませんでした。通信状況を確認して、もう一度お試しください')
    } finally {
      if (seq === requestSeq.current) setLoading(false)
    }
  }, [year, industry])

  useEffect(() => {
    void load()
    return () => controller.current?.abort()
  }, [load])

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default' }}>
      <Container maxWidth="md" sx={{ py: 3, pb: { xs: 12, md: 4 } }}>
        <Typography variant="h5" component="h1" fontWeight="bold" gutterBottom>先輩の進路実績</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          あなたの学校の卒業生が就職した企業を、人数の多い順に表示しています。
          卒業生が特定されないよう、年度・職種ごとに3人未満の人数は出さず、合計にも含めません（「少数」「＋少数」と表示します）。
        </Typography>

        {data && data.school_linked && (
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mb: 2 }}>
            <TextField select size="small" label="卒業年度" value={year} onChange={(e) => setYear(e.target.value === '' ? '' : Number(e.target.value))} sx={{ minWidth: 160 }}>
              <MenuItem value="">すべての年度</MenuItem>
              {data.years.map((y) => <MenuItem key={y} value={y}>{yearLabel(y)}</MenuItem>)}
            </TextField>
            <TextField select size="small" label="業界" value={industry} onChange={(e) => setIndustry(e.target.value)} sx={{ minWidth: 200 }}>
              <MenuItem value="">すべての業界</MenuItem>
              {data.industries.map((i) => <MenuItem key={i} value={i}>{i}</MenuItem>)}
            </TextField>
          </Stack>
        )}

        {error && (
          <Alert severity="error" sx={{ mb: 2 }} action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>
            {error}
          </Alert>
        )}
        {loading && !data && (
          <Stack spacing={1.5} aria-label="進路実績を読み込み中">
            {[0, 1, 2].map((i) => <Skeleton key={i} variant="rounded" height={72} />)}
          </Stack>
        )}

        {data && !data.school_linked && (
          <Alert severity="info">
            学校が登録されていないため、進路実績を表示できません。先生に学校の登録を確認してください。
          </Alert>
        )}
        {loading && data && (
          <Typography variant="body2" color="text.secondary" role="status" sx={{ mb: 1 }}>読み込み中…</Typography>
        )}
        {data && data.school_linked && data.companies.length === 0 && (
          <Typography color="text.secondary">
            {year || industry ? '条件に合う進路実績はありません。年度や業界を変えてみてください。' : 'まだ進路実績が登録されていません。'}
          </Typography>
        )}

        {data && data.school_linked && data.companies.length > 0 && (
          <>
            <Typography variant="body2" sx={{ mb: 1 }}>合計 {totalLabel(data.total, data.total_few)}・{data.companies.length}社</Typography>
            <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', p: 0, m: 0 }}>
              {data.companies.map((c) => (
                <Box component="li" key={c.company_id}>
                  <Card variant="outlined">
                    <CardActionArea component={NextLink} href={`/company/${c.company_id}`}>
                      <CardContent>
                        <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'baseline', gap: 1, flexWrap: 'wrap' }}>
                          <Typography fontWeight={700}>{c.company_name}</Typography>
                          <Typography variant="body2"><strong>{totalLabel(c.total, c.total_few)}</strong></Typography>
                        </Stack>
                        {c.industry && <Typography variant="caption" color="text.secondary">{c.industry}</Typography>}
                        <Stack direction="row" sx={{ flexWrap: 'wrap', gap: 0.5, mt: 1 }}>
                          {c.by_year.map((y) => <Chip key={y.year} size="small" label={`${yearLabel(y.year)} ${y.count}人`} />)}
                          {c.by_year_few && c.total != null && <Chip size="small" variant="outlined" label="＋少数" />}
                        </Stack>
                      </CardContent>
                    </CardActionArea>
                  </Card>
                </Box>
              ))}
            </Stack>
          </>
        )}
      </Container>
    </Box>
  )
}
