/**
 * 分析サイドバーの固定ナビ項目。
 * next/link で prefetch するため href を一元管理する。
 */
export const SIDEBAR_NAV_ITEMS = [
  { href: '/Correlation-diagram', label: '企業相関図' },
  { href: '/chat-history', label: 'チャット履歴' },
  { href: '/resume', label: '履歴書レビュー' },
  { href: '/interview', label: '面接練習' },
  { href: '/es-rewrite', label: 'ESリライト・添削' },
  { href: '/schedule', label: '選考スケジュール' },
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
  '/admin',
  '/onboarding',
]

export function shouldShowStudentBottomNav(pathname: string): boolean {
  return !HIDE_BOTTOM_NAV.some((p) => pathname === p || pathname.startsWith(`${p}/`))
}
