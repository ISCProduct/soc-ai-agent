'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, Skeleton, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { fetchAxisBreakdown, type AxisBreakdown, type AxisRow } from '@/lib/axis-match'

/**
 * 推薦1件の、軸ごとの内訳（SOCAIAGENT-611）。10軸それぞれの「あなた・企業の重視度・一致度」を数値で出し、
 * 合っている点（強み）と、補うと伝わりやすくなる点を短い文で添える。
 *
 * 推薦の順位を変えるものではない。誇張しないよう、文は決まった規則で作ったものだけを出し、
 * 棒は2本の細い線（あなた・企業）にとどめる。
 */
export default function AxisBreakdownPanel({ matchId }: { matchId: number }) {
  const [data, setData] = useState<AxisBreakdown | null>(null)
  const [error, setError] = useState('')
  const seq = useRef(0)

  const load = useCallback(async () => {
    const mine = ++seq.current
    setError('')
    setData(null)
    try {
      const result = await fetchAxisBreakdown(matchId)
      if (mine === seq.current) setData(result)
    } catch (e) {
      if (mine === seq.current) setError(e instanceof Error && e.message.startsWith('この企業') ? e.message : '内訳を読み込めませんでした。')
    }
  }, [matchId])

  useEffect(() => {
    void load()
  }, [load])

  if (error) {
    return (
      <Alert severity="warning" sx={{ mt: 1.5 }} action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>
        {error}
      </Alert>
    )
  }
  if (!data) {
    return (
      <Stack spacing={0.75} sx={{ mt: 1.5 }} aria-label="内訳を読み込み中">
        {[0, 1, 2, 3].map((i) => <Skeleton key={i} variant="rounded" height={16} />)}
      </Stack>
    )
  }

  return (
    <Box sx={{ mt: 1.5, pt: 1.5, borderTop: `1px solid ${UI.ruleSoft}` }}>
      {data.strengths.length > 0 && (
        <Notes title="合っている点" items={data.strengths.map((n) => n.text)} />
      )}
      {data.gaps.length > 0 && (
        <Notes title="補うと伝わりやすくなる点" items={data.gaps.map((n) => n.text)} />
      )}
      {data.strengths.length === 0 && data.gaps.length === 0 && (
        <Typography sx={{ fontSize: 13.5, color: UI.inkSoft, mb: 1.5 }}>
          目立って合っている点・離れている点はありません。下の軸ごとの数値を参考にしてください。
        </Typography>
      )}
      {data.unmeasured.length > 0 && (
        <Typography sx={{ fontSize: 13, color: UI.inkSoft, mb: 1.5 }}>
          この企業が重視している「{data.unmeasured.join('」「')}」は、まだ診断で測れていません。AI との対話を進めると分かります。
        </Typography>
      )}

      <Box
        component="table"
        sx={{
          width: '100%',
          maxWidth: 720,
          borderCollapse: 'collapse',
          '& th, & td': { fontSize: 13, py: 0.5, px: 0.75, textAlign: 'left', verticalAlign: 'middle' },
          '& th': { color: UI.inkSoft, fontWeight: 600, borderBottom: `1px solid ${UI.ruleSoft}` },
          '& td.num': { textAlign: 'right', whiteSpace: 'nowrap' },
          '& th.num': { textAlign: 'right' },
          // 狭い画面では線の列を省く（数値の列で足りる）
          '& .bar': { display: { xs: 'none', sm: 'table-cell' } },
        }}
      >
        <caption style={{ captionSide: 'top', textAlign: 'left', fontSize: 13, color: 'inherit', paddingBottom: 4 }}>
          軸ごとの内訳（0〜100。一致度は「100 − あなたと企業の差」）
        </caption>
        <thead>
          <tr>
            <th scope="col">軸</th>
            <th scope="col" className="num">あなた</th>
            <th scope="col" className="num">企業の重視度</th>
            <th scope="col" className="num">一致度</th>
            <th scope="col" className="bar" style={{ width: '30%' }}>
              <Box component="span" sx={{ fontWeight: 400 }}>線: 上があなた・下が企業</Box>
            </th>
          </tr>
        </thead>
        <tbody>
          {data.axes.map((a) => <AxisTableRow key={a.key} row={a} />)}
        </tbody>
      </Box>
      <Typography sx={{ fontSize: 12, color: UI.inkFaint, mt: 1 }}>
        あなたのスコアは AI との対話から推定した参考値、企業の重視度は企業の情報をもとにした目安です。推薦の順位を決めるものではありません。
      </Typography>
    </Box>
  )
}

function Notes({ title, items }: { title: string; items: string[] }) {
  return (
    <Box sx={{ mb: 1.5 }}>
      <Typography component="h4" sx={{ fontSize: 13.5, fontWeight: 700, color: UI.ink, mb: 0.5 }}>{title}</Typography>
      <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
        {items.map((t) => (
          <li key={t}><Typography component="span" sx={{ fontSize: 13.5, color: UI.inkSoft, lineHeight: 1.8 }}>{t}</Typography></li>
        ))}
      </Box>
    </Box>
  )
}

function AxisTableRow({ row }: { row: AxisRow }) {
  const measured = row.user_score !== null
  return (
    <tr>
      <th scope="row" style={{ fontWeight: 400 }}>{row.label}</th>
      <td className="num">{measured ? row.user_score : '未計測'}</td>
      <td className="num">{row.company_weight}</td>
      <td className="num">{row.match !== null ? Math.round(row.match) : '—'}</td>
      <td className="bar" aria-hidden="true">
        {/* 細い線2本（上: あなた、下: 企業）。数値は左の列にあるので、線は位置の目安だけ */}
        <Bar value={row.user_score} color={UI.mark} />
        <Bar value={row.company_weight} color={UI.inkFaint} />
      </td>
    </tr>
  )
}

function Bar({ value, color }: { value: number | null; color: string }) {
  return (
    <Box sx={{ height: 4, my: '2px', borderRadius: 2, bgcolor: UI.ruleSoft, overflow: 'hidden' }}>
      {value !== null && <Box sx={{ height: '100%', width: `${Math.max(0, Math.min(100, value))}%`, bgcolor: color }} />}
    </Box>
  )
}
