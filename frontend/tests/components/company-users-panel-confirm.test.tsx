/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { CompanyUsersPanel } from '@/components/admin/CompanyUsersPanel'

// 担当者の無効化の確認は window.confirm ではなく MUI の Dialog で行う（SOCAIAGENT-534）

jest.mock('@/lib/auth', () => ({
  authService: {
    getAdminFetchHeaders: () => ({}),
  },
}))

const ACTIVE_USER = {
  id: 3,
  email: 'tantou@example.com',
  name: '担当 花子',
  role: 'member',
  password_set: true,
  invite_pending: false,
  disabled: false,
  disabled_at: null,
}

const originalFetch = global.fetch

function mockFetch(users: unknown[]) {
  const fetchMock = jest.fn().mockImplementation(async (_input: RequestInfo | URL, init?: RequestInit) => {
    const method = (init?.method || 'GET').toUpperCase()
    if (method === 'GET') {
      return { ok: true, status: 200, json: async () => ({ items: users }), text: async () => JSON.stringify({ items: users }) }
    }
    return { ok: true, status: 200, text: async () => '{}', json: async () => ({}) }
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

function patchCalls(fetchMock: jest.Mock) {
  return fetchMock.mock.calls.filter((c) => (c[1]?.method || '').toUpperCase() === 'PATCH')
}

describe('CompanyUsersPanel 無効化の確認ダイアログ', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('無効化を押すと確認ダイアログが出て、確定するまで PATCH を呼ばない', async () => {
    const confirmSpy = jest.spyOn(window, 'confirm')
    const fetchMock = mockFetch([ACTIVE_USER])
    render(<CompanyUsersPanel companyId="5" />)

    fireEvent.click(await screen.findByRole('button', { name: '無効化' }))

    const dialog = await screen.findByRole('dialog', { name: '担当者を無効化' })
    expect(dialog).toHaveTextContent(
      'tantou@example.com を無効化します。この担当者は企業ポータルにログインできなくなります。よろしいですか？',
    )
    expect(screen.getByRole('button', { name: '無効化する' })).toHaveClass('MuiButton-colorError')
    expect(patchCalls(fetchMock)).toHaveLength(0)
    expect(confirmSpy).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: '無効化する' }))

    await waitFor(() => expect(patchCalls(fetchMock)).toHaveLength(1))
    const [url, init] = patchCalls(fetchMock)[0]
    expect(String(url)).toBe('/api/admin/companies/5/company-users/3')
    expect(JSON.parse(init.body)).toEqual({ disabled: true })
    expect(await screen.findByText('企業ユーザーを無効化しました')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('キャンセルすると PATCH を呼ばない', async () => {
    const fetchMock = mockFetch([ACTIVE_USER])
    render(<CompanyUsersPanel companyId="5" />)

    fireEvent.click(await screen.findByRole('button', { name: '無効化' }))
    fireEvent.click(await screen.findByRole('button', { name: 'キャンセル' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(patchCalls(fetchMock)).toHaveLength(0)
  })

  it('再有効化は取り消せる操作なので、確認なしでそのまま PATCH する', async () => {
    const fetchMock = mockFetch([{ ...ACTIVE_USER, disabled: true, disabled_at: '2026-11-01T00:00:00Z' }])
    render(<CompanyUsersPanel companyId="5" />)

    fireEvent.click(await screen.findByRole('button', { name: '再有効化' }))

    await waitFor(() => expect(patchCalls(fetchMock)).toHaveLength(1))
    expect(JSON.parse(patchCalls(fetchMock)[0][1].body)).toEqual({ disabled: false })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})
