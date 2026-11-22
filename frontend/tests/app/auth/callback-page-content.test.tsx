/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import { useRouter, useSearchParams } from 'next/navigation'
import PageContent from '@/app/auth/callback/page-content'
import { authService } from '@/lib/auth'

const push = jest.fn()

jest.mock('next/navigation', () => ({
  useRouter: jest.fn(),
  useSearchParams: jest.fn(),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getUser: jest.fn().mockRejectedValue(new Error('no session yet')),
    saveAuth: jest.fn().mockResolvedValue(undefined),
  },
}))

// 交換の中継ルートはトークンを取り除いて返す
const payload = { user_id: 7, email: 'a@example.com', name: '学生', is_guest: false }

function openWith(query: string) {
  ;(useRouter as jest.Mock).mockReturnValue({ push })
  ;(useSearchParams as jest.Mock).mockReturnValue(new URLSearchParams(query))
  render(<PageContent />)
}

describe('/auth/callback (SOCAIAGENT-683)', () => {
  beforeEach(() => {
    jest.clearAllMocks()
  })

  it('code が来たら交換し、ログイン状態を保存してホームへ進む', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => payload }) as jest.Mock
    openWith('provider=google&code=short-code')

    await waitFor(() => expect(push).toHaveBeenCalledWith('/'))
    const [url, init] = (global.fetch as jest.Mock).mock.calls[0]
    expect(url).toBe('/api/auth/oauth/exchange')
    expect(JSON.parse(init.body)).toEqual({ code: 'short-code' })
    expect(authService.saveAuth).toHaveBeenCalledWith(expect.objectContaining({ user_id: 7 }))
  })

  it('交換が拒否されたら（別のブラウザで始まったログインなど）保存せず、やり直しを促す', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, json: async () => ({ error: 'login_failed' }) }) as jest.Mock
    openWith('provider=google&code=someone-elses-code')

    expect(await screen.findByText(/もう一度ログインをやり直してください/)).toBeInTheDocument()
    expect(authService.saveAuth).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })

  it('旧方式の user= は読まない（SOCAIAGENT-707）', async () => {
    global.fetch = jest.fn() as jest.Mock
    const user = Buffer.from(JSON.stringify(payload)).toString('base64url')
    openWith(`provider=google&user=${user}`)

    expect(await screen.findByText('ユーザー情報が見つかりません')).toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
    expect(authService.saveAuth).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })
})
