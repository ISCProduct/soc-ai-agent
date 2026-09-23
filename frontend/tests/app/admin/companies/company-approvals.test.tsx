/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PageContent from '@/app/admin/companies/page-content'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

// 学校を選ばないと承認状態の取得が走らないため、選択済みの状態から始める。
jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  SchoolFilterSelect: ({ onChange }: { onChange: (id: number | undefined) => void }) => {
    return <button onClick={() => onChange(1)}>学校を選ぶ</button>
  },
}))

const COMPANY = {
  id: 10,
  name: 'テスト株式会社',
  industry: 'IT・ソフトウェア',
  data_status: 'published',
  is_active: true,
}

const originalFetch = global.fetch

/** この画面は res.json() を直接呼ぶ経路と adminFetchJson(res.text()) 経路が混在している */
function jsonRes(body: unknown) {
  const raw = JSON.stringify(body)
  return { ok: true, status: 200, text: async () => raw, json: async () => JSON.parse(raw) }
}

function errRes(status: number, raw: string) {
  return {
    ok: false,
    status,
    text: async () => raw,
    json: async () => JSON.parse(raw),
  }
}

/** 企業一覧は成功させ、company-approvals の応答だけ差し替える */
function mockApprovals(response: () => Promise<unknown>) {
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    const url = String(input)
    if (url.includes('company-approvals')) return response()
    if (url.includes('/api/admin/companies?')) {
      return jsonRes({ companies: [COMPANY], total: 1 })
    }
    return jsonRes({})
  })
}

async function selectSchool() {
  render(<PageContent />)
  await waitFor(() => expect(screen.getByText('学校を選ぶ')).toBeInTheDocument())
  fireEvent.click(screen.getByText('学校を選ぶ'))
}

describe('学校別の企業承認 通信失敗時の挙動 (#1452)', () => {
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('取得が 500 で失敗したとき、断定的な「未承認」を出さない', async () => {
    mockApprovals(async () => errRes(500, '{"error":"internal"}'))
    await selectSchool()

    await waitFor(() => expect(screen.getByText('承認状態 不明')).toBeInTheDocument())
    expect(screen.queryByText('未承認')).not.toBeInTheDocument()
    expect(screen.queryByText('承認済み')).not.toBeInTheDocument()
  })

  it('ALB の HTML エラーページを生表示しない', async () => {
    const html = '<html><head><title>502 Bad Gateway</title></head><body><h1>502 Bad Gateway</h1></body></html>'
    mockApprovals(async () => errRes(502, html))
    await selectSchool()

    await waitFor(() => expect(screen.getByText('承認状態 不明')).toBeInTheDocument())
    expect(screen.queryByText(/Bad Gateway/)).not.toBeInTheDocument()
    expect(screen.queryByText(/<html/)).not.toBeInTheDocument()
  })

  it('通信断で失敗したときも「未承認」にはしない', async () => {
    mockApprovals(() => Promise.reject(new TypeError('Failed to fetch')))
    await selectSchool()

    await waitFor(() => expect(screen.getByText('承認状態 不明')).toBeInTheDocument())
    expect(screen.queryByText('未承認')).not.toBeInTheDocument()
  })

  it('取得成功かつ未承認なら従来どおり「未承認」を出す（後方互換）', async () => {
    mockApprovals(async () => jsonRes({ company_ids: [] }))
    await selectSchool()

    await waitFor(() => expect(screen.getByText('未承認')).toBeInTheDocument())
    expect(screen.queryByText('承認状態 不明')).not.toBeInTheDocument()
  })

  it('取得成功かつ承認済みなら「承認済み」を出す', async () => {
    mockApprovals(async () => jsonRes({ company_ids: [COMPANY.id] }))
    await selectSchool()

    await waitFor(() => expect(screen.getByText('承認済み')).toBeInTheDocument())
  })

  it('承認操作が失敗したとき、エラーが出てチップの見た目は変わらない', async () => {
    let call = 0
    global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('company-approvals')) {
        const method = (init?.method || 'GET').toUpperCase()
        if (method === 'GET') {
          call += 1
          return jsonRes({ company_ids: [] })
        }
        return errRes(500, '{"error":"承認に失敗しました"}')
      }
      if (url.includes('/api/admin/companies?')) {
        return jsonRes({ companies: [COMPANY], total: 1 })
      }
      return jsonRes({})
    })

    await selectSchool()
    await waitFor(() => expect(screen.getByText('未承認')).toBeInTheDocument())

    fireEvent.click(screen.getByText('未承認'))

    await waitFor(() => expect(screen.getByText('承認に失敗しました')).toBeInTheDocument())
    // 成功したかのように見せない
    expect(screen.getByText('未承認')).toBeInTheDocument()
    expect(screen.queryByText('承認済み')).not.toBeInTheDocument()
    expect(call).toBeGreaterThan(0)
  })
})
