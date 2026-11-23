/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import { StudentSearchContent } from '@/components/company-portal/StudentSearchContent'
import { companyStudentService } from '@/lib/company/students'

jest.mock('next/navigation', () => {
  const router = { replace: jest.fn(), push: jest.fn() }
  return { useRouter: () => router }
})

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ id: 1 }),
    ensureFreshToken: jest.fn().mockResolvedValue(undefined),
    getAuthHeaders: () => ({}),
  },
}))

jest.mock('@/lib/company/students', () => ({
  companyStudentService: {
    search: jest.fn().mockResolvedValue({ items: [], total: 0 }),
    listTagNames: jest.fn().mockResolvedValue({ items: [] }),
  },
}))

const search = companyStudentService.search as jest.Mock

describe('StudentSearchContent', () => {
  beforeEach(() => {
    search.mockReset()
    search.mockResolvedValue({ items: [], total: 0 })
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: [] }),
    }) as unknown as typeof fetch
  })

  it('学生を探す見出しを出す', async () => {
    render(<StudentSearchContent />)

    expect(await screen.findByRole('heading', { name: '学生を探す' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'ダッシュボードへ' })).not.toBeInTheDocument()
  })

  it('検索の応答が前後しても、最後に出した検索の結果だけを表示する', async () => {
    const resolvers: Array<(v: unknown) => void> = []
    search.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)))
    const student = (name: string) => ({ items: [{ user_id: name.length, name, tags: [] }], total: 1 })

    const { container } = render(<StudentSearchContent />)
    const form = container.querySelector('form') as HTMLFormElement
    // 初回取得と検索2回を出す（入力欄での Enter はボタンの無効化に関係なく送信される）
    fireEvent.submit(form)
    fireEvent.submit(form)
    expect(resolvers).toHaveLength(3)

    // 最後の検索 → 先の検索 → 初回取得 の順に返る
    await act(async () => resolvers[2](student('最新の学生')))
    await act(async () => resolvers[1](student('古い検索の学生')))
    await act(async () => resolvers[0](student('初回の学生')))

    expect(await screen.findByText('最新の学生')).toBeInTheDocument()
    expect(screen.queryByText('古い検索の学生')).not.toBeInTheDocument()
    expect(screen.queryByText('初回の学生')).not.toBeInTheDocument()
  })

  it('古い検索が後から返っても、最新の検索が終わるまで読み込み中のまま', async () => {
    const resolvers: Array<(v: unknown) => void> = []
    search.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)))

    const { container } = render(<StudentSearchContent />)
    fireEvent.submit(container.querySelector('form') as HTMLFormElement)
    await act(async () => resolvers[0]({ items: [], total: 0 }))

    expect(screen.getByRole('button', { name: '検索' })).toBeDisabled()
    await act(async () => resolvers[1]({ items: [], total: 0 }))
    expect(screen.getByRole('button', { name: '検索' })).toBeEnabled()
  })
})
