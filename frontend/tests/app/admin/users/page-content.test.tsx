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

// 学校絞り込みUI(SchoolFilterSelect)は独自に fetch するため、本テストの対象外として固定する。
// 既定は担当校なし(restricted: false)。テストごとに差し替えられるよう jest.fn で持つ。
const mockGetAdminSchoolAccess = jest.fn(async () => ({ restricted: false, schools: [] }))
jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: () => mockGetAdminSchoolAccess(),
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

// 担当校スコープの取得が失敗したとき、フェイルオープンして教員に権限変更ボタンを
// 見せていた(#1448)。バックエンドは403を返すので権限昇格ではないが、
// 「押せば必ず403になるボタンを出さない」という #1157 の意図が取得失敗時に崩れていた。
describe('管理ユーザー画面 担当校スコープの取得失敗 (#1448)', () => {
  afterEach(() => {
    global.fetch = originalFetch
    mockGetAdminSchoolAccess.mockImplementation(async () => ({ restricted: false, schools: [] }))
  })

  /** 一覧取得だけ成功させる */
  function mockListOk() {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ users: [USER], total: 1 }),
    })
  }

  it('担当校スコープの取得に失敗したら権限変更ボタンを出さず、失敗を表示する', async () => {
    // SchoolFilterSelect も同じ関数を呼ぶため Once だとそちらに消費される。
    // 画面全体で失敗する状況を再現したいので実装ごと差し替える。
    mockGetAdminSchoolAccess.mockImplementation(() => Promise.reject(new Error('boom')))
    mockListOk()
    render(<PageContent />)

    expect(await screen.findByText('権限情報の取得に失敗しました')).toBeInTheDocument()
    // 行が描画されたことを、権限とは無関係に常に出る削除ボタンで待つ
    await screen.findByRole('button', { name: '削除' })
    expect(screen.queryByRole('button', { name: '管理者にする' })).not.toBeInTheDocument()
  })

  it('担当校ありの管理者には権限変更ボタンを出さない', async () => {
    mockGetAdminSchoolAccess.mockImplementation(async () => ({ restricted: true, schools: [] }))
    mockListOk()
    render(<PageContent />)

    await screen.findByRole('button', { name: '削除' })
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: '管理者にする' })).not.toBeInTheDocument()
    })
  })

  it('担当校なしの管理者には従来どおり権限変更ボタンを出す', async () => {
    mockListOk()
    render(<PageContent />)

    expect(await screen.findByRole('button', { name: '管理者にする' })).toBeInTheDocument()
  })
})
