import { createTranslator } from '@/lib/i18n'

// 表示名は辞書（messages/ja.json の nav）が正本（SOCAIAGENT-595）。
const t = createTranslator('nav')

/**
 * 分析サイドバーの固定ナビ項目。
 * next/link で prefetch するため href を一元管理する。
 */
export const SIDEBAR_NAV_ITEMS = [
  { href: '/correlation-diagram', label: t('sidebar.correlationDiagram') },
  { href: '/chat-history', label: t('sidebar.chatHistory') },
  { href: '/resume', label: t('sidebar.resume') },
  { href: '/assessments', label: t('sidebar.personalityAssessment') },
  { href: '/ability-assessments', label: t('sidebar.abilityAssessment') },
  { href: '/interview', label: t('sidebar.interview') },
  { href: '/interview/history', label: t('sidebar.interviewHistory') },
  { href: '/es-rewrite', label: t('sidebar.esRewrite') },
  { href: '/schedule', label: t('sidebar.schedule') },
  { href: '/applications', label: t('sidebar.applications') },
  { href: '/scout', label: t('sidebar.scout') },
  { href: '/profile', label: t('sidebar.profile') },
] as const

export const SIDEBAR_ADMIN_NAV = {
  href: '/admin',
  label: t('admin'),
} as const

export const STUDENT_BOTTOM_NAV_ITEMS = [
  { href: '/', label: t('bottom.diagnosis') },
  { href: '/results', label: t('bottom.results') },
  { href: '/interview', label: t('bottom.interview') },
  { href: '/resume', label: t('bottom.resume') },
  // 行き先の見出し（プロフィール）と名前を揃える（#1576）。
  { href: '/profile', label: t('bottom.profile') },
] as const

/**
 * ホーム以外の画面のメニュー先頭に置く項目（#1576）。
 * SIDEBAR_NAV_ITEMS はホームのサイドバーと共用で、ホーム自身と結果画面を含まない。
 * そのままだと PC で /schedule などからホームにもマッチング結果にも戻れなかった。
 */
export const GLOBAL_NAV_HOME_ITEMS = [
  { href: '/', label: t('home.home') },
  { href: '/results', label: t('home.results') },
] as const

/** 今いる画面がナビ項目に当たるか。/ は完全一致、それ以外は配下も含める。 */
export function isCurrentNav(pathname: string, href: string): boolean {
  return href === '/' ? pathname === '/' : pathname === href || pathname.startsWith(`${href}/`)
}

/**
 * `allHrefs` の中に前方一致が重なる項目があるとき（例: /interview と /interview/history）、
 * 最も長く一致する項目だけを現在地とする。GlobalNavMenu / AnalysisSidebar で共通利用（#403）。
 */
export function isCurrentNavExclusive(pathname: string, href: string, allHrefs: readonly string[]): boolean {
  return isCurrentNav(pathname, href) && !allHrefs.some((o) => o.length > href.length && isCurrentNav(pathname, o))
}

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
