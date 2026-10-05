/**
 * 分析サイドバーの固定ナビ項目。
 * next/link で prefetch するため href を一元管理する。
 */
export const SIDEBAR_NAV_ITEMS = [
  { href: '/correlation-diagram', label: '企業相関図' },
  { href: '/chat-history', label: 'チャット履歴' },
  { href: '/resume', label: '履歴書レビュー' },
  { href: '/interview', label: '面接練習' },
  { href: '/interview/history', label: '面接履歴' },
  { href: '/es-rewrite', label: 'ESリライト・添削' },
  { href: '/schedule', label: '選考スケジュール' },
  { href: '/applications', label: '選考管理' },
  { href: '/profile', label: 'プロフィール設定' },
] as const

export const SIDEBAR_ADMIN_NAV = {
  href: '/admin',
  label: '管理者機能',
} as const

export const STUDENT_BOTTOM_NAV_ITEMS = [
  { href: '/', label: '診断' },
  { href: '/results', label: '結果' },
  { href: '/interview', label: '面接' },
  { href: '/resume', label: '履歴書' },
  { href: '/profile', label: '設定' },
] as const

const HIDE_BOTTOM_NAV = [
  '/login',
  '/forgot-password',
  '/reset-password',
  '/register',
  '/verify-email',
  '/verify-registration',
  '/auth',
  '/error',
  '/company-entry',
  '/company-portal',
  '/admin',
  '/onboarding',
]

export function shouldShowStudentBottomNav(pathname: string): boolean {
  return !HIDE_BOTTOM_NAV.some((p) => pathname === p || pathname.startsWith(`${p}/`))
}

/** モバイルの Bottom nav（固定表示）の実高さ相当。スクロール末尾の余白確保に使う。 */
export const BOTTOM_NAV_HEIGHT = 'calc(56px + env(safe-area-inset-bottom))'

/**
 * 画面下に固定表示するものの sx。Bottom nav の上へ逃がす（#1570）。
 *
 * MUI の Snackbar は下から 24px に出るので、56px の Bottom nav と重なって
 * 読めない・押せない。画面ごとに数字を書くと必ずどこかで取りこぼすため、
 * ここに集約する。md 以上は Bottom nav が出ないので既定へ戻す。
 */
export const ABOVE_BOTTOM_NAV_SX = {
  bottom: { xs: `calc(${BOTTOM_NAV_HEIGHT} + 16px)`, md: 24 },
} as const
