/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { useRouter } from 'next/navigation'
import PageContent from '@/app/favorites/page-content'

const push = jest.fn()

jest.mock('next/navigation', () => ({
  useRouter: jest.fn(),
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ 'X-User-Token': 'test-token' }),
    getStoredUser: () => null,
  },
}))

const company = {
  id: 7,
  match_id: 20,
  category_name: '北見工業',
  industry: '製造',
  job_title: '生産技術',
  score: 81,
  category_scores: { technical: 81.2 },
  is_favorited: true,
}

function mockFavorites(body: unknown, ok = true) {
  global.fetch = jest.fn().mockResolvedValue({
    ok,
    json: async () => body,
  }) as jest.Mock
}

describe('お気に入り画面', () => {
  beforeEach(() => {
    push.mockReset()
    ;(useRouter as jest.Mock).mockReturnValue({ push })
  })

  it('読み込みに失敗したときは空の一覧として扱わない', async () => {
    mockFavorites({}, false)
    render(<PageContent />)
    expect(screen.getByRole('status', { name: 'お気に入りを読み込み中' })).toBeInTheDocument()
    expect(await screen.findByText('お気に入りを読み込めませんでした')).toBeInTheDocument()
    expect(screen.queryByText('お気に入りにした企業はまだありません')).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2))
  })

  it('0件のときは次にマッチング結果を見る導線を出す', async () => {
    mockFavorites({ recommendations: [] })
    render(<PageContent />)
    expect(await screen.findByText('お気に入りにした企業はまだありません')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'おすすめ企業を見る' }))
    expect(push).toHaveBeenCalledWith('/')
  })

  it('企業名は企業詳細へ飛び、ハートは診断をまたいで外す', async () => {
    mockFavorites({ recommendations: [company] })
    render(<PageContent />)
    expect(await screen.findByRole('button', { name: '北見工業' })).toBeInTheDocument()
    expect(screen.getAllByText('81').length).toBeGreaterThan(0)

    fireEvent.click(screen.getByRole('button', { name: '北見工業' }))
    expect(push).toHaveBeenCalledWith('/company/7')

    ;(global.fetch as jest.Mock).mockResolvedValueOnce({ ok: true, json: async () => ({}) })
    fireEvent.click(screen.getByRole('button', { name: '北見工業をお気に入りから外す' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: '北見工業' })).not.toBeInTheDocument())
    expect(global.fetch).toHaveBeenLastCalledWith('/api/chat/favorites/remove', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ company_id: 7 }),
    }))
  })
})
