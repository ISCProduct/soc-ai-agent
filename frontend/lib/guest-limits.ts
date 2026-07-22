export const GUEST_LIMITATIONS = [
  '結果・面接レポートのメール送信',
  '選考管理（応募状況の保存）',
  'GitHub スキル連携',
  'Googleカレンダー連携',
] as const

export const GUEST_EMAIL_DISABLED_REASON = 'ゲストユーザーはメール送信できません。アカウント登録後にご利用ください。'

export const GUEST_REGISTER_CTA_LABEL = 'アカウント登録して機能を解放'

/** ゲスト向け登録導線（ログイン画面の登録タブへ） */
export const GUEST_REGISTER_PATH = '/login'

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
