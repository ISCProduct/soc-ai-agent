/**
 * @jest-environment jsdom
 */
import { TextDecoder } from 'node:util'
import { render, screen, waitFor } from '@testing-library/react'
import { useRouter, useSearchParams } from 'next/navigation'
import PageContent from '@/app/auth/callback/page-content'
import { authService } from '@/lib/auth'

// jsdom には TextDecoder が無い（ブラウザにはある）
Object.assign(global, { TextDecoder })

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

const payload = { user_id: 7, email: 'a@example.com', name: '学生', user_token: 'user-jwt', is_guest: false }

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
    expect(authService.saveAuth).toHaveBeenCalledWith(expect.objectContaining({ user_id: 7, user_token: 'user-jwt' }))
  })

  it('交換が拒否されたら（別のブラウザで始まったログインなど）保存せず、やり直しを促す', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, json: async () => ({ error: 'login_failed' }) }) as jest.Mock
    openWith('provider=google&code=someone-elses-code')

    expect(await screen.findByText(/もう一度ログインをやり直してください/)).toBeInTheDocument()
    expect(authService.saveAuth).not.toHaveBeenCalled()
    expect(push).not.toHaveBeenCalled()
  })

  it('旧方式の user= も移行期間中は読める', async () => {
    global.fetch = jest.fn() as jest.Mock
    // Backend の base64.URLEncoding（URL 安全な文字・パディングあり）と同じ形式
    const user = Buffer.from(JSON.stringify(payload)).toString('base64').replace(/\+/g, '-').replace(/\//g, '_')
    openWith(`provider=google&user=${user}`)

    await waitFor(() => expect(push).toHaveBeenCalledWith('/'))
    expect(global.fetch).not.toHaveBeenCalled()
    expect(authService.saveAuth).toHaveBeenCalledWith(expect.objectContaining({ user_id: 7 }))
  })
})
