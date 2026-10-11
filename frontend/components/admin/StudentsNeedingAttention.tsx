'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import { Alert, Box, Button, Card, CardContent, Divider, Skeleton, Stack, Typography } from '@mui/material'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'
import { INACTIVE_DAYS, type StudentTendencyResponse } from '@/lib/student-insights'
import { LOW_MATCH_THRESHOLD } from '@/lib/low-match'

type Row = {
  key: string
  /** 事実だけを書く。人物評価の言葉を使わない（#1573）。 */
  label: string
  param: string
  total: number
  names: string[]
}

type State = { kind: 'loading' } | { kind: 'error' } | { kind: 'ready'; rows: Row[] }

const CONDITIONS = [
  { key: 'inactive', label: `最近${INACTIVE_DAYS}日間、活動がありません`, param: `inactive_days=${INACTIVE_DAYS}` },
  { key: 'low_match', label: `マッチ度${LOW_MATCH_THRESHOLD}未満の企業への応募が進行中です`, param: 'low_match_only=true' },
  { key: 'resume', label: '履歴書が未提出か、見直しが必要です', param: 'resume_needs_attention_only=true' },
] as const

const PREVIEW = 3

async function loadRows(): Promise<Row[]> {
  const access = await getAdminSchoolAccess()
  // ponytail: 担当校が複数ある教員は先頭校だけを数える。全校合算が要るなら学校ごとに並べる。
  const school = access.restricted ? access.schools[0]?.id : undefined
  const headers = authService.getAdminFetchHeaders()
  return Promise.all(
    CONDITIONS.map(async (c) => {
      const qs = `${c.param}&limit=${PREVIEW}${school !== undefined ? `&school_id=${school}` : ''}`
      const res = await fetch(`/api/admin/teacher/students/tendency-analysis?${qs}`, { headers, cache: 'no-store' })
      if (!res.ok) throw new Error('students needing attention fetch failed')
      const data: StudentTendencyResponse = await res.json()
      return { ...c, total: data.total ?? 0, names: (data.students ?? []).map((s) => s.name || s.email) }
    }),
  )
}

/**
 * 教員の入口に出す「対応が必要な学生」（#1573）。
 * 条件ごとに人数と先頭数名を出し、傾向分析の一覧をその条件で開く。
 */
export function StudentsNeedingAttention() {
  const [state, setState] = useState<State>({ kind: 'loading' })

  const load = useCallback((isStale: () => boolean = () => false) => {
    setState({ kind: 'loading' })
    loadRows()
      .then((rows) => { if (!isStale()) setState({ kind: 'ready', rows }) })
      .catch(() => { if (!isStale()) setState({ kind: 'error' }) })
  }, [])

  useEffect(() => {
    let stale = false
    load(() => stale)
    return () => { stale = true }
  }, [load])

  const rows = state.kind === 'ready' ? state.rows.filter((r) => r.total > 0) : []

  return (
    <Box component="section" aria-labelledby="needs-attention-heading" sx={{ mb: 3 }}>
      <Typography id="needs-attention-heading" variant="h6" sx={{ mt: 1, mb: 1.5 }}>
        対応が必要な学生
      </Typography>
      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
        <CardContent>
          {state.kind === 'loading' ? (
            <Stack spacing={1} role="status" aria-label="対応が必要な学生を読み込み中">
              {CONDITIONS.map((c) => <Skeleton key={c.key} variant="text" height={32} />)}
            </Stack>
          ) : state.kind === 'error' ? (
            <Alert
              severity="error"
              action={<Button color="inherit" size="small" onClick={() => load()}>再読み込み</Button>}
            >
              対応が必要な学生を読み込めませんでした。時間をおいて再読み込みしてください。
            </Alert>
          ) : rows.length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              {`いま当てはまる学生はいません（最近${INACTIVE_DAYS}日間の未活動・低マッチの応募・履歴書の見直し）。生徒全体は傾向分析で確認できます。`}
            </Typography>
          ) : (
            <Stack divider={<Divider flexItem />} spacing={1.5}>
              {rows.map((r) => (
                <Stack
                  key={r.key}
                  direction={{ xs: 'column', sm: 'row' }}
                  spacing={1}
                  sx={{ alignItems: { sm: 'center' }, justifyContent: 'space-between' }}
                >
                  <Box sx={{ minWidth: 0 }}>
                    <Typography variant="body1" fontWeight={700}>{`${r.total}人: ${r.label}`}</Typography>
                    <Typography variant="body2" color="text.secondary">
                      {r.names.join('、')}{r.total > r.names.length ? ` ほか${r.total - r.names.length}人` : ''}
                    </Typography>
                  </Box>
                  <Button
                    variant="outlined"
                    component={Link}
                    href={`/admin/student-insights?${r.param}`}
                    sx={{ flexShrink: 0 }}
                  >
                    該当する学生を一覧で見る
                  </Button>
                </Stack>
              ))}
            </Stack>
          )}
          <Typography variant="caption" color="text.secondary" component="p" sx={{ mt: 2 }}>
            記録上の事実だけを示しています。声をかけるきっかけとしてご活用ください。
          </Typography>
        </CardContent>
      </Card>
    </Box>
  )
}
