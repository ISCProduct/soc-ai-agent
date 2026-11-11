'use client'

import { useCallback, useEffect, useState } from 'react'
import { Alert, Box, Button, Paper, Skeleton, Stack, TextField, Typography } from '@mui/material'
import { authService } from '@/lib/auth'

/**
 * 機能横断のコンバージョンファネル（診断 → 面接 → ES → 応募 → 内定）（#2039）。
 *
 * 教員がどの段階で学生が止まっているかを見て、指導の手を打てるようにする。
 * 数値は棒の長さだけに頼らず、人数と割合を文字でも出す（色やグラフが読めなくても分かるように）。
 */

export type FunnelStage = {
  stage: 'diagnosis' | 'interview' | 'es' | 'applied' | 'offered'
  reached: number
  rate_of_students: number
  continued_from_previous: number
  continuation_rate: number
}

export type FunnelResponse = {
  funnel: { students: number; stages: FunnelStage[] }
}

export const FUNNEL_STAGE_LABELS: Record<FunnelStage['stage'], string> = {
  diagnosis: '診断',
  interview: '面接練習',
  es: 'ES添削',
  applied: '応募',
  offered: '内定',
}

const STAGE_HELP: Record<FunnelStage['stage'], string> = {
  diagnosis: 'AI診断で回答した、または学校の診断を提出した',
  interview: 'AI面接を最後まで終えた',
  es: 'ESの添削を受けた',
  applied: '企業に応募した',
  offered: '内定・内定承諾',
}

export function formatRate(rate: number): string {
  return `${Math.round(rate * 1000) / 10}%`
}

type Props = {
  adminEmail: string
  schoolId: number | undefined
}

export function ConversionFunnelCard({ adminEmail, schoolId }: Props) {
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [data, setData] = useState<FunnelResponse['funnel'] | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    if (!adminEmail) return
    if (from && to && from > to) {
      setError('開始日は終了日以前の日付にしてください')
      return
    }
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({
        ...(schoolId !== undefined ? { school_id: String(schoolId) } : {}),
        ...(from ? { from } : {}),
        ...(to ? { to } : {}),
      })
      const res = await fetch(`/api/admin/dashboard/funnel?${params}`, {
        headers: { 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' },
      })
      const body = await res.json().catch(() => ({}))
      if (!res.ok) {
        // 担当校を持つ先生は、学校の選択が決まるまで school_id 無しの 400 が返る。エラーとして見せない
        if (res.status === 400 && schoolId === undefined && /school_id/.test(String(body?.message ?? ''))) {
          setData(null)
          return
        }
        throw new Error(res.status === 403 ? 'この学校のデータを見る権限がありません' : 'ファネルを読み込めませんでした')
      }
      setData((body as FunnelResponse).funnel)
    } catch (e: unknown) {
      setData(null)
      setError(e instanceof Error ? e.message : 'ファネルを読み込めませんでした')
    } finally {
      setLoading(false)
    }
  }, [adminEmail, schoolId, from, to])

  useEffect(() => {
    void load()
  }, [load])

  return (
    <Paper variant="outlined" sx={{ p: { xs: 2, md: 3 }, mb: 3 }}>
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={2}
        sx={{ alignItems: { xs: 'stretch', md: 'center' }, justifyContent: 'space-between', mb: 2 }}
      >
        <Box>
          <Typography variant="h6" component="h2">進路のファネル</Typography>
          <Typography variant="body2" color="text.secondary">
            診断から内定まで、どの段階で学生が止まっているかを確認できます
          </Typography>
        </Box>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
          <TextField
            label="開始日"
            type="date"
            size="small"
            value={from}
            onChange={(e) => setFrom(e.target.value)}
            InputLabelProps={{ shrink: true }}
          />
          <TextField
            label="終了日"
            type="date"
            size="small"
            value={to}
            onChange={(e) => setTo(e.target.value)}
            InputLabelProps={{ shrink: true }}
          />
          {(from || to) && (
            <Button size="small" onClick={() => { setFrom(''); setTo('') }}>期間を解除</Button>
          )}
        </Stack>
      </Stack>

      {error && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}
        >
          {error}
        </Alert>
      )}

      {loading && !data && (
        <Stack spacing={1} aria-label="ファネルを読み込み中">
          {Object.keys(FUNNEL_STAGE_LABELS).map((k) => <Skeleton key={k} variant="rounded" height={36} />)}
        </Stack>
      )}

      {data && data.students === 0 && (
        <Typography color="text.secondary">対象の学生がいません。学校の選択を確認してください。</Typography>
      )}

      {data && data.students > 0 && (
        <Box>
          <Typography variant="body2" color="text.secondary" mb={1.5}>
            対象の学生 {data.students} 人{from || to ? `（期間: ${from || '指定なし'} 〜 ${to || '指定なし'}）` : ''}
          </Typography>
          <Stack component="ol" spacing={1.5} sx={{ listStyle: 'none', p: 0, m: 0 }}>
            {data.stages.map((s, i) => (
              <Box component="li" key={s.stage}>
                <Stack direction="row" sx={{ justifyContent: 'space-between', alignItems: 'baseline', gap: 1, flexWrap: 'wrap' }}>
                  <Typography fontWeight={600}>
                    {i + 1}. {FUNNEL_STAGE_LABELS[s.stage]}
                    <Typography component="span" variant="caption" color="text.secondary" ml={1}>
                      {STAGE_HELP[s.stage]}
                    </Typography>
                  </Typography>
                  <Typography variant="body2">
                    <strong>{s.reached}人</strong>（{formatRate(s.rate_of_students)}）
                    {i > 0 && (
                      <Typography component="span" variant="caption" color="text.secondary" ml={1}>
                        前の段階から {formatRate(s.continuation_rate)} が継続
                      </Typography>
                    )}
                  </Typography>
                </Stack>
                <Box
                  role="img"
                  aria-label={`${FUNNEL_STAGE_LABELS[s.stage]} ${s.reached}人、対象の学生の${formatRate(s.rate_of_students)}`}
                  sx={{ mt: 0.5, height: 10, borderRadius: 5, bgcolor: 'action.hover', overflow: 'hidden' }}
                >
                  <Box sx={{ height: '100%', width: `${Math.min(100, s.rate_of_students * 100)}%`, bgcolor: 'primary.main' }} />
                </Box>
              </Box>
            ))}
          </Stack>
          <Typography variant="caption" color="text.secondary" display="block" mt={2}>
            「前の段階から継続」は、前の段階に到達した学生のうち、この段階にも到達した割合です。
            内定のあと辞退した学生は、選考の記録が辞退に変わるため内定に数えられません。
          </Typography>
        </Box>
      )}
    </Paper>
  )
}
