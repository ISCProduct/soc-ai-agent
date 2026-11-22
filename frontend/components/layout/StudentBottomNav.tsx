'use client'

import { usePathname, useRouter } from 'next/navigation'
import { BottomNavigation, BottomNavigationAction, Paper } from '@mui/material'
import ChatIcon from '@mui/icons-material/Chat'
import BusinessIcon from '@mui/icons-material/Business'
import RecordVoiceOverIcon from '@mui/icons-material/RecordVoiceOver'
import DescriptionIcon from '@mui/icons-material/Description'
import ManageAccountsIcon from '@mui/icons-material/ManageAccounts'
import { STUDENT_BOTTOM_NAV_ITEMS, isCurrentNav, shouldShowStudentBottomNav } from '@/lib/sidebar-nav'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { useInterviewSessionActive } from '@/app/interview/sessionChrome'
import { useHasStoredUser } from './useHasStoredUser'

const ICONS = {
  '/': <ChatIcon />,
  '/results': <BusinessIcon />,
  '/interview': <RecordVoiceOverIcon />,
  '/resume': <DescriptionIcon />,
  '/profile': <ManageAccountsIcon />,
} as const

export function StudentBottomNav() {
  const pathname = usePathname() || ''
  const router = useRouter()

  // 未ログインの訪問者には出さない（#1653）。
  //
  // `/` が公開LPになったため、パスだけで判定すると展示会のQRから来た来場者に
  // 診断・結果・面接… が見える。いずれも認証必須の画面で、押すと /login へ飛ぶ。
  // `/` を HIDE_BOTTOM_NAV へ足す手は使えない。ログイン済み学生の `/` は
  // 診断タブそのものだから。
  //
  // セッション Cookie は httpOnly でクライアントから読めないので、ストレージを
  // 手掛かりにする。サーバー側（RootLayout）で Cookie を見る手もあるが、
  // cookies() を root layout で呼ぶと全ルートが動的レンダリングになり、
  // 実測で静的ページが 21 → 3 に減った（/login・/privacy・/company-portal 配下など）。
  // ナビの出し分けのために全ページのプリレンダリングを捨てるのは割に合わない。
  //
  // 残差: セッションが切れてストレージだけ残った状態では LP でもナビが出る。
  // ここは従来と同じ挙動で、押しても /login へ落ちるだけなので許容する。
  // 判定は GlobalNavMenu（メニューボタン）と共有する（#2222）。
  const { mounted, hasStoredUser } = useHasStoredUser()

  // AI面接のセッション画面（/interview の内部 status）表示中は隠す。
  // SelectionScreen/LobbyScreen と同一パスのため pathname では区別できない（#1569）。
  const interviewSessionActive = useInterviewSessionActive()

  if (!shouldShowStudentBottomNav(pathname)) return null
  if (interviewSessionActive) return null
  if (!mounted || !hasStoredUser) return null

  const current =
    STUDENT_BOTTOM_NAV_ITEMS.find((i) => isCurrentNav(pathname, i.href))?.href ?? false

  return (
    <Paper
      elevation={8}
      sx={{
        position: 'fixed',
        // 印刷（ポートフォリオの PDF など）に載せない
        '@media print': { display: 'none' },
        bottom: 0,
        left: 0,
        right: 0,
        zIndex: (t) => t.zIndex.appBar,
        display: { xs: 'block', md: 'none' },
        pb: 'env(safe-area-inset-bottom)',
      }}
    >
      <BottomNavigation
        showLabels
        value={current}
        onChange={(_, href: string) => {
          router.push(href === '/results' ? getResultsPathOrChat() : href)
        }}
      >
        {STUDENT_BOTTOM_NAV_ITEMS.map((item) => (
          <BottomNavigationAction
            key={item.href}
            value={item.href}
            label={item.label}
            icon={ICONS[item.href]}
            // 現在地を読み上げに伝える（#2222）。見た目は色だけでなく、上の線と太字でも示す
            aria-current={current === item.href ? 'page' : undefined}
            sx={{
              // MUI既定の minWidth=80px は5項目で400px必要になり、
              // 400px未満の端末で両端が画面外へ出る（320pxでは診断が x=-40、
              // 設定の右端が 360 まで押し出されて押せない）。
              // 0 にして等分させる。文字は隠さない。
              minWidth: 0,
              px: 0.25,
              '&.Mui-selected': {
                boxShadow: 'inset 0 3px 0 currentColor',
              },
              '& .MuiBottomNavigationAction-label': {
                // 等分すると320pxでは1項目64px。「プロフィール」(6字)が収まる大きさまで
                // 縮める。読めなくなる大きさにはしない。
                fontSize: { xs: '0.625rem', sm: '0.75rem' },
                whiteSpace: 'nowrap',
              },
              // MUI の選択時の文字の拡大（0.875rem）はやめ、大きさはそのままで太字にする（はみ出さないように）
              '& .MuiBottomNavigationAction-label.Mui-selected': {
                fontSize: { xs: '0.625rem', sm: '0.75rem' },
                fontWeight: 700,
              },
            }}
          />
        ))}
      </BottomNavigation>
    </Paper>
  )
}
