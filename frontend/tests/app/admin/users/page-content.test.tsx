/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PageContent from '@/app/admin/users/page-content'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

// 学校絞り込みUI(SchoolFilterSelect)は独自に fetch するため、本テストの対象外として固定する
jest.mock('@/lib/admin-school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

const USER = {
  id: 7,
  email: 'student@example.com',
  name: '学生 太郎',
  is_guest: false,
  is_admin: false,
  created_at: '2026-09-01',
  updated_at: '2026-09-01',
}

const originalFetch = global.fetch

/** 一覧取得(GET)は成功させ、その後の操作系リクエスト(PUT/DELETE)だけ結果を差し替える */
function mockListThen(operationResult: () => Promise<unknown>) {
  global.fetch = jest.fn().mockImplementation((_input: RequestInfo | URL, init?: RequestInit) => {
    const method = (init?.method || 'GET').toUpperCase()
    if (method === 'GET') {
      return Promise.resolve({
        ok: true,
        status: 200,
        text: async () => JSON.stringify({ users: [USER], total: 1 }),
      })
    }
    return operationResult()
  })
}

describe('管理ユーザー画面 通信失敗時の挙動 (#1066)', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('権限切り替えが通信エラーで失敗しても、エラーが表示されボタンが操作可能に戻る', async () => {
    mockListThen(() => Promise.reject(new TypeError('Failed to fetch')))
    render(<PageContent />)

    const toggle = await screen.findByRole('button', { name: '管理者にする' })
    expect(toggle).not.toBeDisabled()

    fireEvent.click(toggle)

    expect(await screen.findByText('通信エラーが発生しました。しばらくしてから再試行してください。')).toBeInTheDocument()
    // ローディング固着の回帰ガード: disabled のまま固まらないこと
    await waitFor(() => expect(screen.getByRole('button', { name: '管理者にする' })).not.toBeDisabled())
    expect(screen.getByRole('button', { name: '削除' })).not.toBeDisabled()
  })

  it('削除が通信エラーで失敗しても、エラーが表示されボタンが操作可能に戻る', async () => {
    jest.spyOn(window, 'confirm').mockReturnValue(true)
    mockListThen(() => Promise.reject(new TypeError('Failed to fetch')))
    render(<PageContent />)

    const remove = await screen.findByRole('button', { name: '削除' })
    fireEvent.click(remove)

    expect(await screen.findByText('通信エラーが発生しました。しばらくしてから再試行してください。')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '削除' })).not.toBeDisabled())
  })

  it('サーバーがエラー文言を返した場合はその文言を表示する', async () => {
    mockListThen(async () => ({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: '自分自身の権限は変更できません' }),
    }))
    render(<PageContent />)

    fireEvent.click(await screen.findByRole('button', { name: '管理者にする' }))

    expect(await screen.findByText('自分自身の権限は変更できません')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: '管理者にする' })).not.toBeDisabled())
  })

  it('一覧取得が通信エラーで失敗したとき、黙って空にせずエラーを表示する', async () => {
    global.fetch = jest.fn().mockRejectedValue(new TypeError('Failed to fetch'))
    render(<PageContent />)

    expect(await screen.findByText('通信エラーが発生しました。しばらくしてから再試行してください。')).toBeInTheDocument()
  })

  it('一覧取得がALBのHTMLエラーページを受け取っても生HTMLを表示しない', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 503,
      text: async () => '<html><head><title>503 Service Temporarily Unavailable</title></head></html>',
    })
    render(<PageContent />)

    expect(await screen.findByText('ただいま接続できません。しばらくしてから再試行してください。')).toBeInTheDocument()
    expect(screen.queryByText(/<html>/)).not.toBeInTheDocument()
  })
})
