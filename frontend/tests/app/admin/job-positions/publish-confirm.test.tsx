/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import PageContent from '@/app/admin/job-positions/page-content'

// 未公開企業の求人の承認は、window.confirm ではなく確認ダイアログで確定してから行う（SOCAIAGENT-534）。
// 求人管理の画面を通して、キャンセル・確定・失敗のそれぞれで PATCH とダイアログの状態を確かめる。

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

// 承認・却下のボタンはシステム管理者（担当校なし）にだけ出る
jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

const POSITIONS = [
  { id: 1, company_id: 10, title: '求人A', data_status: 'draft', company: { id: 10, name: '未公開A社', data_status: 'draft' } },
  { id: 2, company_id: 20, title: '求人B', data_status: 'draft', company: { id: 20, name: '未公開B社', data_status: 'draft' } },
  { id: 3, company_id: 30, title: '求人C', data_status: 'draft', company: { id: 30, name: '公開C社', data_status: 'published' } },
]

const originalFetch = global.fetch

function res(body: unknown, ok = true) {
  return { ok, status: ok ? 200 : 500, json: async () => body }
}

function mockFetch({ publishOk = true, publishThrows = false }: { publishOk?: boolean; publishThrows?: boolean } = {}) {
  const fetchMock = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.startsWith('/api/admin/job-positions?')) return res({ positions: POSITIONS })
    if (url.endsWith('/publish') && publishThrows) throw new TypeError('Failed to fetch')
    if (url.endsWith('/publish')) return publishOk ? res({}) : res({ error: '承認できませんでした' }, false)
    return res({})
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

const publishCalls = (fetchMock: jest.Mock) =>
  fetchMock.mock.calls.filter((c) => String(c[0]).endsWith('/publish')).map((c) => [String(c[0]), c[1]?.method])
const listCalls = (fetchMock: jest.Mock) =>
  fetchMock.mock.calls.filter((c) => String(c[0]).startsWith('/api/admin/job-positions?')).length

/** 求人のタイトルから、その求人のカードの「承認」ボタンを押す */
async function clickApprove(title: string) {
  render(<PageContent />)
  await screen.findByText(title)
  // 承認ボタンは、担当校の取得（非同期）でシステム管理者と分かってから出る
  const buttons = await screen.findAllByRole('button', { name: '承認' })
  const index = POSITIONS.findIndex((p) => p.title === title)
  fireEvent.click(buttons[index])
}

describe('求人管理 未公開企業の求人の承認', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('未公開企業の求人は確認ダイアログを出し、キャンセルしたら PATCH しない', async () => {
    const confirmSpy = jest.spyOn(window, 'confirm')
    const fetchMock = mockFetch()
    await clickApprove('求人A')

    const dialog = await screen.findByRole('dialog', { name: '未公開企業の求人を承認' })
    expect(within(dialog).getByText(/この企業はまだ公開されていません/)).toBeInTheDocument()
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(publishCalls(fetchMock)).toEqual([])

    fireEvent.click(within(dialog).getByRole('button', { name: 'キャンセル' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(publishCalls(fetchMock)).toEqual([])
  })

  it('確定すると、選んだ求人の ID だけを PATCH し、ダイアログを閉じて一覧を取り直す', async () => {
    const fetchMock = mockFetch()
    await clickApprove('求人B')
    const listBefore = listCalls(fetchMock)

    const dialog = await screen.findByRole('dialog', { name: '未公開企業の求人を承認' })
    fireEvent.click(within(dialog).getByRole('button', { name: '承認する' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(publishCalls(fetchMock)).toEqual([['/api/admin/job-positions/2/publish', 'PATCH']])
    await waitFor(() => expect(listCalls(fetchMock)).toBe(listBefore + 1))
  })

  it('公開済みの企業の求人は、確認ダイアログを出さずにそのまま PATCH する', async () => {
    const fetchMock = mockFetch()
    await clickApprove('求人C')

    await waitFor(() => expect(publishCalls(fetchMock)).toEqual([['/api/admin/job-positions/3/publish', 'PATCH']]))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('承認に失敗したら、ダイアログを閉じてエラーを出す', async () => {
    const fetchMock = mockFetch({ publishOk: false })
    await clickApprove('求人A')

    const dialog = await screen.findByRole('dialog', { name: '未公開企業の求人を承認' })
    fireEvent.click(within(dialog).getByRole('button', { name: '承認する' }))

    expect(await screen.findByText('承認できませんでした')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(publishCalls(fetchMock)).toEqual([['/api/admin/job-positions/1/publish', 'PATCH']])
  })

  it('通信が切れて承認できなかったときも、ダイアログを閉じてエラーを出す', async () => {
    mockFetch({ publishThrows: true })
    await clickApprove('求人A')

    const dialog = await screen.findByRole('dialog', { name: '未公開企業の求人を承認' })
    fireEvent.click(within(dialog).getByRole('button', { name: '承認する' }))

    expect(await screen.findByText('承認に失敗しました。通信状況を確かめて、もう一度お試しください')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })
})
