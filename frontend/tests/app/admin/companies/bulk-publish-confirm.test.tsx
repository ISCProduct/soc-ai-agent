/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PageContent from '@/app/admin/companies/page-content'

// 一括公開の確認は window.confirm ではなく MUI の Dialog で行う（SOCAIAGENT-534）

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

// 一括公開の選択UIはシステム管理者（担当校なし）にだけ出る
jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  SchoolFilterSelect: () => null,
}))

/** 公開前で主情報（会社概要・技術情報・関連企業）がそろっている＝一括公開の対象になる企業 */
const DRAFT_COMPANY = {
  id: 10,
  name: 'テスト株式会社',
  industry: 'IT・ソフトウェア',
  data_status: 'draft',
  is_active: true,
  description: '説明',
  website_url: 'https://example.com',
  info_fetched_at: '2026-11-01T00:00:00Z',
  tech_stack: '["Go"]',
  tech_fetched_at: '2026-11-01T00:00:00Z',
  relations_fetched_at: '2026-11-01T00:00:00Z',
}

const originalFetch = global.fetch

function jsonRes(body: unknown) {
  const raw = JSON.stringify(body)
  return { ok: true, status: 200, text: async () => raw, json: async () => JSON.parse(raw) }
}

function mockFetch() {
  const fetchMock = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('/api/admin/companies?')) {
      return jsonRes({ companies: [DRAFT_COMPANY], total: 1 })
    }
    return jsonRes({})
  })
  global.fetch = fetchMock as unknown as typeof fetch
  return fetchMock
}

function publishCalls(fetchMock: jest.Mock) {
  return fetchMock.mock.calls.filter((c) => String(c[0]).endsWith('/publish'))
}

async function selectAndRequestBulkPublish() {
  render(<PageContent />)
  fireEvent.click(await screen.findByRole('checkbox', { name: 'テスト株式会社を選択' }))
  fireEvent.click(await screen.findByRole('button', { name: '選択した 1 社を学生に公開' }))
}

describe('企業管理 一括公開の確認ダイアログ', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('一括公開を押すと確認ダイアログが出て、確定するまで公開の API を呼ばない', async () => {
    const confirmSpy = jest.spyOn(window, 'confirm')
    const fetchMock = mockFetch()
    await selectAndRequestBulkPublish()

    const dialog = await screen.findByRole('dialog', { name: '企業を一括公開' })
    expect(dialog).toHaveTextContent('選択した 1 社を学生に公開しますか？')
    expect(publishCalls(fetchMock)).toHaveLength(0)
    expect(confirmSpy).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: '公開する' }))

    await waitFor(() => expect(publishCalls(fetchMock)).toHaveLength(1))
    expect(String(publishCalls(fetchMock)[0][0])).toBe('/api/admin/companies/10/publish')
    expect(await screen.findByText('1 社を学生に公開しました。')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('キャンセルすると公開の API を呼ばない', async () => {
    const fetchMock = mockFetch()
    await selectAndRequestBulkPublish()

    await screen.findByRole('dialog', { name: '企業を一括公開' })
    fireEvent.click(screen.getByRole('button', { name: 'キャンセル' }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(publishCalls(fetchMock)).toHaveLength(0)
  })
})
