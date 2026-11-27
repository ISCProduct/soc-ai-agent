'use client'

import { useCallback, useEffect, useState } from 'react'
import NextLink from 'next/link'
import { Box, Button, ButtonBase, Skeleton, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import { HOME_COLORS, HOME_LAYOUT } from '@/lib/home-theme'
import { canDismiss, type StudentGuidance } from '@/lib/teacher-guidance'
import { type NextAction } from '@/lib/next-actions'
import { AIBudgetNotice } from './AIBudgetNotice'
import { loadNextActions } from './NextActionsBar'
import { NoticeStack, type Notice } from './NoticeStack'

interface HomeRightPanelProps {
  guidances: StudentGuidance[]
  guidanceLoadFailed: boolean
  /** 案内を既読にできなかったときの文言 */
  guidanceError: string
  dismissingIds: Set<number>
  onDismissGuidance: (id: number) => void
  onRetryGuidances: () => void
  /** 先生の案内以外のお知らせ（履歴書・通知・新着情報と、その取得失敗） */
  notices: Notice[]
}

type ScheduleState = { kind: 'loading' } | { kind: 'error' } | { kind: 'ready'; actions: NextAction[] }

const sectionHeading = { fontWeight: 700, fontSize: 20, lineHeight: 1.5, color: HOME_COLORS.ink } as const
const bodyText = { fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.ink } as const

/** 「これからの予定」に並べる件数。残りは予定の画面で見る。 */
const MAX_SCHEDULE_ITEMS = 3

/**
 * ホームの右パネル（Figma「改善G／相談中心」ノード 37:435）。先生から・これからの予定・お知らせ。
 * 取得はそれぞれ独立していて、1つが失敗しても、ほかの欄とメインは使える。
 */
export function HomeRightPanel({
  guidances,
  guidanceLoadFailed,
  guidanceError,
  dismissingIds,
  onDismissGuidance,
  onRetryGuidances,
  notices,
}: HomeRightPanelProps) {
  const [schedule, setSchedule] = useState<ScheduleState>({ kind: 'loading' })

  const loadSchedule = useCallback(() => {
    setSchedule({ kind: 'loading' })
    loadNextActions()
      .then((actions) => setSchedule({ kind: 'ready', actions }))
      .catch(() => setSchedule({ kind: 'error' }))
  }, [])

  useEffect(() => {
    loadSchedule()
  }, [loadSchedule])

  return (
    <Box
      component="aside"
      aria-label="先生からの案内・予定・お知らせ"
      sx={{
        width: HOME_LAYOUT.panelWidth,
        flexShrink: 0,
        height: '100dvh',
        overflowY: 'auto',
        bgcolor: HOME_COLORS.white,
        borderLeft: `1px solid ${HOME_COLORS.border}`,
        px: 3,
        py: 4,
        display: 'flex',
        flexDirection: 'column',
        gap: 4,
      }}
    >
      <Box component="section" aria-labelledby="home-panel-guidance" sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <Typography id="home-panel-guidance" component="h2" sx={sectionHeading}>
          先生から
        </Typography>
        {guidanceLoadFailed && (
          <Alert
            severity="error"
            action={
              <Button color="inherit" size="small" onClick={onRetryGuidances}>
                再読み込み
              </Button>
            }
          >
            先生からの案内を読み込めませんでした。通信状況を確認して再読み込みしてください
          </Alert>
        )}
        {guidanceError && <Alert severity="error">{guidanceError}</Alert>}
        {!guidanceLoadFailed && guidances.length === 0 && (
          <Typography sx={bodyText}>先生からの案内はありません。</Typography>
        )}
        {guidances.map((g) => (
          <Box
            key={g.id}
            sx={{ bgcolor: HOME_COLORS.noticeBg, borderRadius: '8px', p: 2, display: 'flex', flexDirection: 'column', gap: 1 }}
          >
            <Typography sx={{ fontWeight: 700, fontSize: 16, lineHeight: 1.5, color: HOME_COLORS.noticeInk }}>お知らせ</Typography>
            <Typography sx={{ fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.noticeInk, overflowWrap: 'anywhere' }}>
              {g.message}
              {g.suggested_industries && g.suggested_industries.length > 0 && `（参考: ${g.suggested_industries.join('、')}）`}
            </Typography>
            <ButtonBase
              // 閉じる処理中は無効にする。連打すると同じ案内へ何度も POST が飛ぶ。
              disabled={!canDismiss(dismissingIds, g.id)}
              onClick={() => onDismissGuidance(g.id)}
              sx={{
                alignSelf: 'flex-start',
                minWidth: 96,
                height: 48,
                px: 3,
                bgcolor: HOME_COLORS.white,
                border: `1px solid ${HOME_COLORS.link}`,
                borderRadius: '8px',
                color: HOME_COLORS.link,
                fontSize: 16,
                fontWeight: 700,
                fontFamily: 'inherit',
                '&.Mui-disabled': { opacity: 0.6 },
              }}
            >
              読みました
            </ButtonBase>
          </Box>
        ))}
      </Box>

      <Box component="section" aria-labelledby="home-panel-schedule" sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <Typography id="home-panel-schedule" component="h2" sx={sectionHeading}>
          これからの予定
        </Typography>
        {schedule.kind === 'loading' && <Skeleton variant="text" aria-label="予定を読み込み中" height={28} />}
        {schedule.kind === 'error' && (
          <Alert
            severity="error"
            action={
              <Button color="inherit" size="small" onClick={loadSchedule}>
                再読み込み
              </Button>
            }
          >
            予定を読み込めませんでした
          </Alert>
        )}
        {schedule.kind === 'ready' && schedule.actions.length === 0 && <Typography sx={bodyText}>予定はまだありません。</Typography>}
        {schedule.kind === 'ready' &&
          schedule.actions.slice(0, MAX_SCHEDULE_ITEMS).map((a) => (
            <Typography key={a.key} sx={bodyText}>
              {/* 行き先の無い項目（改善の提案など）はリンクにしない */}
              {a.href ? (
                <Box component={NextLink} href={a.href} sx={{ color: HOME_COLORS.link, textDecoration: 'underline' }}>
                  {a.text}
                </Box>
              ) : (
                a.text
              )}
            </Typography>
          ))}
        {schedule.kind === 'ready' && schedule.actions.length > MAX_SCHEDULE_ITEMS && (
          <Typography sx={{ fontSize: 16, lineHeight: 1.7, color: HOME_COLORS.soft }}>
            {`ほか${schedule.actions.length - MAX_SCHEDULE_ITEMS}件`}
          </Typography>
        )}
        <Box
          component={NextLink}
          href="/schedule"
          sx={{
            display: 'flex',
            alignItems: 'center',
            minHeight: 44,
            fontSize: 16,
            lineHeight: 1.7,
            fontWeight: 700,
            color: HOME_COLORS.link,
            textDecoration: 'none',
            '&:hover': { textDecoration: 'underline' },
          }}
        >
          予定を追加する
        </Box>
      </Box>

      <Box component="section" aria-labelledby="home-panel-notices" sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        <Typography id="home-panel-notices" component="h2" sx={sectionHeading}>
          お知らせ
        </Typography>
        {notices.length === 0 ? <Typography sx={bodyText}>新しいお知らせはありません。</Typography> : <NoticeStack notices={notices} />}
        <AIBudgetNotice />
      </Box>
    </Box>
  )
}
