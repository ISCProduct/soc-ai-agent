'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import NextLink from 'next/link'
import { Box, Button, Skeleton, Typography } from '@mui/material'
import EventIcon from '@mui/icons-material/Event'
import { authService } from '@/lib/auth'
import { fetchWithTimeout, LIST_FETCH_TIMEOUT_MS } from '@/lib/fetch-timeout'
import { pickNextActions, type GrowthStatus, type NextAction } from '@/lib/next-actions'
import { fetchPortfolio } from '@/lib/portfolio'

type State =
  | { kind: 'loading' }
  | { kind: 'error' }
  | { kind: 'ready'; actions: NextAction[] }

/**
 * 診断と面接の状況（SOCAIAGENT-614）。既存の /api/portfolio から取る。
 * 取れなくても予定・応募は出したいので、失敗（タイムアウトを含む）は null にする（改善の提案を出さないだけ）。
 * 帯だけが使う。右パネルの「これからの予定」には改善の提案を出さない。
 */
async function loadGrowthStatus(): Promise<GrowthStatus | null> {
  try {
    const p = await fetchPortfolio(['diagnosis', 'interview'], LIST_FETCH_TIMEOUT_MS)
    return { diagnosis: p.diagnosis?.scores ?? [], interviewCount: p.interviews?.length ?? 0 }
  } catch {
    return null
  }
}

/**
 * 直近の予定と選考状況から「次にやること」を組み立てる（右パネルの「これからの予定」でも使う）。
 * growth を渡したときだけ、改善の提案も混ぜる（帯だけが渡す）。
 */
export async function loadNextActions(growth?: Promise<GrowthStatus | null>): Promise<NextAction[]> {
  const headers = authService.getUserFetchHeaders()
  const [schedule, applications, growthStatus] = await Promise.all([
    fetchWithTimeout('/api/schedule', { headers, cache: 'no-store' }),
    fetchWithTimeout('/api/applications', { headers, cache: 'no-store' }),
    growth ?? Promise.resolve(null),
  ])
  if (!schedule.ok || !applications.ok) throw new Error('next actions fetch failed')
  const events = await schedule.json()
  const apps = await applications.json()
  return pickNextActions(Array.isArray(events) ? events : [], apps?.applications ?? [], new Date(), growthStatus)
}

/**
 * ホーム上部の「次にやること」（#1576）。チャットの入力欄を押し下げないよう常に1行に収め、
 * 2件目以降は件数だけ出してスケジュール・選考状況へ送る。
 */
export function NextActionsBar() {
  const [state, setState] = useState<State>({ kind: 'loading' })

  // 再読み込みを続けて押したとき、先の遅い応答で新しい結果を上書きしない
  const seq = useRef(0)
  const load = useCallback(() => {
    const mine = ++seq.current
    setState({ kind: 'loading' })
    loadNextActions(loadGrowthStatus())
      .then((actions) => { if (mine === seq.current) setState({ kind: 'ready', actions }) })
      .catch(() => { if (mine === seq.current) setState({ kind: 'error' }) })
  }, [])

  useEffect(() => { load() }, [load])

  return (
    <Box
      component="section"
      aria-label="次にやること"
      sx={{
        display: 'flex', alignItems: 'center', gap: 1.5, px: 2, py: 1,
        minHeight: 52, bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider',
      }}
    >
      <EventIcon color="primary" aria-hidden />
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Typography variant="caption" color="text.secondary" component="p">次にやること</Typography>
        {state.kind === 'loading' ? (
          <Skeleton variant="text" width="60%" aria-label="次にやることを読み込み中" />
        ) : state.kind === 'error' ? (
          <Typography variant="body2" role="alert" noWrap>
            予定と選考状況を読み込めませんでした
          </Typography>
        ) : state.actions.length === 0 ? (
          // 折り返すと帯が2〜3行になり、375pxでは会話ログが下限まで潰れる（#1763）。
          <Typography
            variant="body2"
            noWrap
            title="直近2週間の予定はありません。面接や締切を登録すると、ここに出ます"
          >
            直近2週間の予定はありません。面接や締切を登録すると、ここに出ます
          </Typography>
        ) : (
          <Typography variant="body2" fontWeight={700} noWrap title={state.actions[0].text}>
            {state.actions[0].text}
            {state.actions.length > 1 && (
              <Typography component="span" variant="body2" color="text.secondary" fontWeight={400}>
                {`（ほか${state.actions.length - 1}件）`}
              </Typography>
            )}
          </Typography>
        )}
      </Box>
      {state.kind === 'error' ? (
        <Button size="small" variant="outlined" onClick={load}>再読み込み</Button>
      ) : state.kind === 'ready' ? (
        state.actions.length === 0 ? (
          <Button size="small" variant="outlined" component={NextLink} href="/schedule">予定を登録</Button>
        ) : state.actions[0].href ? (
          <Button size="small" variant="contained" component={NextLink} href={state.actions[0].href}>
            {state.actions[0].actionLabel}
          </Button>
        ) : null
      ) : null}
    </Box>
  )
}
