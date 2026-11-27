/**
 * @jest-environment jsdom
 */
import { act, render, screen } from '@testing-library/react'
import PageContent from '@/app/company/[id]/page-content'

let mockId = '1'
jest.mock('next/navigation', () => ({
  useParams: () => ({ id: mockId }),
  useRouter: () => ({ back: jest.fn(), push: jest.fn() }),
}))
jest.mock('next/dynamic', () => () => function DiagramStub() {
  return null
})

type Deferred = { resolve: (value: unknown) => void }
const pending = new Map<string, Deferred>()
const originalFetch = global.fetch

const okJson = (body: unknown) => ({ ok: true, status: 200, json: async () => body })

beforeEach(() => {
  mockId = '1'
  pending.clear()
  // 応答は手動で返す（返す順番をテストで入れ替えるため）
  global.fetch = jest.fn().mockImplementation(
    (input: RequestInfo | URL) =>
      new Promise((resolve) => {
        pending.set(String(input), { resolve })
      }),
  ) as unknown as typeof fetch
})

afterEach(() => {
  global.fetch = originalFetch
})

async function respond(url: string, body: unknown) {
  await act(async () => {
    pending.get(url)?.resolve(okJson(body))
  })
}

async function respondError(url: string) {
  await act(async () => {
    pending.get(url)?.resolve({ ok: false, status: 500, json: async () => ({}) })
  })
}

describe('企業詳細の古い応答の破棄', () => {
  it('別の企業へ移った後に前の企業の応答が返っても、表示は新しい企業のまま', async () => {
    const { rerender } = render(<PageContent />)
    mockId = '2'
    rerender(<PageContent />)

    await respond('/api/companies/2', { id: 2, name: '新しい株式会社' })
    await respond('/api/companies/2/job-positions', { positions: [{ id: 20, title: '新しい求人' }] })
    // 前の企業の応答が後から返る
    await respond('/api/companies/1', { id: 1, name: '古い株式会社' })
    await respond('/api/companies/1/job-positions', { positions: [{ id: 10, title: '古い求人' }] })

    expect(await screen.findByText('新しい株式会社')).toBeInTheDocument()
    expect(screen.queryByText('古い株式会社')).not.toBeInTheDocument()
    expect(screen.getByText('新しい求人')).toBeInTheDocument()
    expect(screen.queryByText('古い求人')).not.toBeInTheDocument()
  })

  it('移った先の企業の求人取得に失敗しても、前の企業の求人を残さない', async () => {
    const { rerender } = render(<PageContent />)
    await respond('/api/companies/1', { id: 1, name: '前の株式会社' })
    await respond('/api/companies/1/job-positions', { positions: [{ id: 10, title: '前の求人' }] })
    expect(await screen.findByText('前の求人')).toBeInTheDocument()

    mockId = '2'
    rerender(<PageContent />)
    await respond('/api/companies/2', { id: 2, name: '次の株式会社' })
    await respondError('/api/companies/2/job-positions')

    expect(await screen.findByText('次の株式会社')).toBeInTheDocument()
    expect(screen.queryByText('前の求人')).not.toBeInTheDocument()
  })
})
