/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import PageContent from '@/app/schedule/page-content'

// 予定の削除は、window.confirm ではなく確認ダイアログで確定してから行う（SOCAIAGENT-534）。
// 編集ダイアログから削除するときは、編集ダイアログを残したまま確認ダイアログを重ね、確定したときだけ両方を閉じる。

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), back: jest.fn(), replace: jest.fn() }),
}))

jest.mock('@/lib/auth', () => ({
  authService: { getStoredUser: () => ({ user_id: 1 }) },
}))

/** 今月10日の予定（UTC 03:00。日本時間でも UTC でも同じ日になる） */
function thisMonthIso() {
  const now = new Date()
  return new Date(Date.UTC(now.getFullYear(), now.getMonth(), 10, 3, 0)).toISOString()
}

const EVENT = { id: 7, user_id: 1, company_name: 'テスト株式会社', stage: '一次面接', title: '', scheduled_at: thisMonthIso(), notes: '' }

const originalFetch = global.fetch

function mockFetch({ deleteOk = true }: { deleteOk?: boolean } = {}) {
  const fetchMock = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (init?.method === 'DELETE') {
      return deleteOk ? { ok: true, status: 204, json: async () => ({}) } : { ok: false, status: 500, json: async () => ({}) }
    }
    if (url === '/api/schedule') return { ok: true, status: 200, json: async () => [EVENT] }
    return { ok: true, status: 200, json: async () => ({}) }
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

const deleteCalls = (fetchMock: jest.Mock) =>
  fetchMock.mock.calls.filter((c) => c[1]?.method === 'DELETE').map((c) => String(c[0]))
const listCalls = (fetchMock: jest.Mock) =>
  fetchMock.mock.calls.filter((c) => String(c[0]) === '/api/schedule' && !c[1]?.method).length

async function renderWithEvent() {
  render(<PageContent />)
  await screen.findAllByText('テスト株式会社')
}

/** 一覧の予定の編集ボタンから編集ダイアログを開き、その中の「削除」を押す */
async function requestDeleteFromEditDialog() {
  await renderWithEvent()
  fireEvent.click(screen.getByRole('button', { name: '予定を編集' }))
  expect(await screen.findByText('イベントを編集')).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: '削除' }))
  return screen.findByRole('dialog', { name: 'イベントを削除' })
}

describe('選考スケジュール 予定の削除の確認', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('一覧の削除ボタンで確認ダイアログを出し、キャンセルしたら DELETE しない', async () => {
    const confirmSpy = jest.spyOn(window, 'confirm')
    const fetchMock = mockFetch()
    await renderWithEvent()

    fireEvent.click(screen.getByRole('button', { name: '予定を削除' }))
    const dialog = await screen.findByRole('dialog', { name: 'イベントを削除' })
    expect(within(dialog).getByText('このイベントを削除しますか？')).toBeInTheDocument()
    expect(confirmSpy).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'キャンセル' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'イベントを削除' })).not.toBeInTheDocument())
    expect(deleteCalls(fetchMock)).toEqual([])
  })

  it('編集ダイアログから削除すると、編集ダイアログを残したまま確認を重ね、キャンセルしたら編集に戻る', async () => {
    const fetchMock = mockFetch()
    const dialog = await requestDeleteFromEditDialog()
    // 確認中も編集ダイアログは開いたまま
    expect(screen.getByText('イベントを編集')).toBeInTheDocument()

    fireEvent.click(within(dialog).getByRole('button', { name: 'キャンセル' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'イベントを削除' })).not.toBeInTheDocument())
    expect(screen.getByText('イベントを編集')).toBeInTheDocument()
    expect(deleteCalls(fetchMock)).toEqual([])
  })

  it('確定すると、対象の予定を DELETE し、両方のダイアログを閉じて一覧を取り直す', async () => {
    const fetchMock = mockFetch()
    const dialog = await requestDeleteFromEditDialog()
    const listBefore = listCalls(fetchMock)

    fireEvent.click(within(dialog).getByRole('button', { name: '削除する' }))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'イベントを削除' })).not.toBeInTheDocument())
    await waitFor(() => expect(screen.queryByText('イベントを編集')).not.toBeInTheDocument())
    expect(deleteCalls(fetchMock)).toEqual(['/api/schedule/7'])
    await waitFor(() => expect(listCalls(fetchMock)).toBe(listBefore + 1))
  })

  it('削除に失敗したら、編集ダイアログを閉じてエラーを出す', async () => {
    const fetchMock = mockFetch({ deleteOk: false })
    const dialog = await requestDeleteFromEditDialog()

    fireEvent.click(within(dialog).getByRole('button', { name: '削除する' }))

    expect(await screen.findByText('削除に失敗しました')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'イベントを削除' })).not.toBeInTheDocument())
    await waitFor(() => expect(screen.queryByText('イベントを編集')).not.toBeInTheDocument())
    expect(deleteCalls(fetchMock)).toEqual(['/api/schedule/7'])
  })
})
