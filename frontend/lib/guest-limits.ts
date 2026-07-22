export const GUEST_LIMITATIONS = [
  '結果・面接レポートのメール送信',
  '選考管理（応募状況の保存）',
  'GitHub スキル連携',
  'Googleカレンダー連携',
] as const

export const GUEST_EMAIL_DISABLED_REASON =
  'ゲストユーザーはメール送信できません。アカウント登録後にご利用ください。'

export const GUEST_APPLICATIONS_DISABLED_REASON =
  'ゲストユーザーは応募・選考管理を利用できません。アカウント登録後にご利用ください。'

export const GUEST_REGISTER_CTA_LABEL = 'アカウント登録して機能を解放'

/** ログイン画面の新規登録タブを開くクエリ */
export const LOGIN_REGISTER_TAB_PARAM = 'tab'
export const LOGIN_REGISTER_TAB_VALUE = 'register'

/** ゲスト向け登録導線（ログイン画面の新規登録タブへ） */
export const GUEST_REGISTER_PATH = `/login?${LOGIN_REGISTER_TAB_PARAM}=${LOGIN_REGISTER_TAB_VALUE}`

export function isLoginRegisterTab(tab: string | null | undefined): boolean {
  return tab === LOGIN_REGISTER_TAB_VALUE
}

export function canGuestSendEmail(isGuest: boolean | undefined | null): boolean {
  return !isGuest
}

export function getGuestEmailButtonProps(isGuest: boolean | undefined | null): {
  disabled: boolean
  title: string
} {
  if (isGuest) {
    return { disabled: true, title: GUEST_EMAIL_DISABLED_REASON }
  }
  return { disabled: false, title: '' }
}

export function getGuestApplicationsButtonProps(isGuest: boolean | undefined | null): {
  disabled: boolean
  title: string
} {
  if (isGuest) {
    return { disabled: true, title: GUEST_APPLICATIONS_DISABLED_REASON }
  }
  return { disabled: false, title: '' }
}
