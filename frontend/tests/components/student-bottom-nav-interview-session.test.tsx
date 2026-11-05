/** @jest-environment jsdom */

import { act, render } from '@testing-library/react'
import { StudentBottomNav } from '@/components/StudentBottomNav'
import { setInterviewSessionActive } from '@/app/interview/sessionChrome'

/**
 * AI面接のセッション画面（SessionScreen）表示中は学生下部ナビを隠すこと（#1569）。
 *
 * SelectionScreen・LobbyScreen・SessionScreen はすべて同一パス `/interview` 上で
 * 内部 status を切り替えて表示されるため、pathname だけでは区別できない。
 * SessionScreen は mount/unmount で setInterviewSessionActive を呼ぶ
 * （実際の呼び出しは SessionScreen.tsx 側）。ここでは StudentBottomNav が
 * そのフラグを正しく反映することを検証する。
 */

jest.mock('next/navigation', () => ({
  usePathname: () => '/interview',
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
}))

const STORED_USER = {
  user_id: 1,
  email: 'student@example.com',
  name: 'テストユーザー',
  is_guest: false,
  is_admin: false,
}

describe('StudentBottomNav とAI面接セッションの連動 (#1569)', () => {
  beforeEach(() => {
    window.localStorage.clear()
    window.sessionStorage.clear()
    setInterviewSessionActive(false)
  })

  afterEach(() => {
    setInterviewSessionActive(false)
  })

  it('セッション画面表示中(setInterviewSessionActive(true))は下部ナビを描画しない', async () => {
    window.localStorage.setItem('user', JSON.stringify(STORED_USER))

    const { findByRole, queryByRole } = render(<StudentBottomNav />)
    // 通常は(ログイン済み・/interview 自体はHIDE_BOTTOM_NAV対象外のため)表示される
    await findByRole('button', { name: '診断' })

    act(() => {
      setInterviewSessionActive(true)
    })

    expect(queryByRole('button', { name: '診断' })).not.toBeInTheDocument()

    act(() => {
      setInterviewSessionActive(false)
    })

    expect(await findByRole('button', { name: '診断' })).toBeInTheDocument()
  })
})
