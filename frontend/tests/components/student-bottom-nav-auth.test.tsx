/** @jest-environment jsdom */

import { render } from '@testing-library/react'
import { StudentBottomNav } from '@/components/StudentBottomNav'

/**
 * 未ログインの訪問者に学生用ナビを出さないこと（#1653）。
 *
 * `/` が公開LPになったため、パス判定だけだと展示会のQRから来た来場者に
 * 診断・結果・面接・履歴書・設定が見える。いずれも認証必須の画面。
 * `/` を HIDE_BOTTOM_NAV へ足す手は使えない。ログイン済み学生の `/` は
 * 診断タブそのものだから。
 *
 * このナビは xs 幅だけに出る（`display: { xs: 'block', md: 'none' }`）ので、
 * PCでの目視確認では壊れていても気づけない。テストで固定する。
 */

jest.mock('next/navigation', () => ({
  // LPもログイン済みダッシュボードも `/`。パス判定では区別できないことを示す。
  usePathname: () => '/',
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
}))

const STORED_USER = {
  user_id: 1,
  email: 'student@example.com',
  name: 'テストユーザー',
  is_guest: false,
  is_admin: false,
}

describe('StudentBottomNav の表示条件', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.sessionStorage.clear()
  })

  it('未ログイン（ストレージが空）では描画しない', async () => {
    const { container, findByRole } = render(<StudentBottomNav />)
    await expect(findByRole('button', { name: '診断' })).rejects.toThrow()
    expect(container).toBeEmptyDOMElement()
  })

  it('ログイン済みなら従来どおり5項目を描画する', async () => {
    window.localStorage.setItem('user', JSON.stringify(STORED_USER))

    const { findByRole, getByRole } = render(<StudentBottomNav />)
    expect(await findByRole('button', { name: '診断' })).toBeInTheDocument()
    for (const label of ['結果', '面接', '履歴書', '設定']) {
      expect(getByRole('button', { name: label })).toBeInTheDocument()
    }
  })
})
