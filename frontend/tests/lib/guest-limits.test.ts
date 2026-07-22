import {
  GUEST_LIMITATIONS,
  GUEST_EMAIL_DISABLED_REASON,
  GUEST_REGISTER_PATH,
  canGuestSendEmail,
  getGuestEmailButtonProps,
} from '@/lib/guest-limits'

describe('guest-limits', () => {
  it('ゲスト制限一覧に主要機能が含まれる', () => {
    expect(GUEST_LIMITATIONS).toEqual(
      expect.arrayContaining([
        expect.stringContaining('メール送信'),
        expect.stringContaining('選考管理'),
        expect.stringContaining('GitHub'),
        expect.stringContaining('カレンダー'),
      ])
    )
  })

  it('canGuestSendEmail はゲスト以外のみ true', () => {
    expect(canGuestSendEmail(true)).toBe(false)
    expect(canGuestSendEmail(false)).toBe(true)
    expect(canGuestSendEmail(null)).toBe(true)
    expect(canGuestSendEmail(undefined)).toBe(true)
  })

  it('getGuestEmailButtonProps はゲスト時に disabled + 理由', () => {
    expect(getGuestEmailButtonProps(true)).toEqual({
      disabled: true,
      title: GUEST_EMAIL_DISABLED_REASON,
    })
    expect(getGuestEmailButtonProps(false)).toEqual({
      disabled: false,
      title: '',
    })
  })

  it('登録導線はログイン画面', () => {
    expect(GUEST_REGISTER_PATH).toBe('/login')
  })
})
