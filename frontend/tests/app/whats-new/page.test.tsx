/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import WhatsNewPage from '@/app/whats-new/page'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ back: jest.fn() }),
}))

describe('WhatsNewPage', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('取得した更新情報を新しい順に表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => [
        { title: '新機能A', summary: '説明A', merged_at: '2026-08-15T00:00:00Z' },
        { title: '過去の更新', summary: '説明B', merged_at: '2026-08-01T00:00:00Z' },
      ],
    })

    render(<WhatsNewPage />)

    expect(await screen.findByText('新機能A')).toBeInTheDocument()
    expect(screen.getByText('過去の更新')).toBeInTheDocument()
  })

  it('取得失敗時はエラーメッセージを表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: false })

    render(<WhatsNewPage />)

    expect(await screen.findByText(/取得に失敗しました/)).toBeInTheDocument()
  })

  it('更新情報が0件の場合はその旨を表示する', async () => {
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => [] })

    render(<WhatsNewPage />)

    expect(await screen.findByText('更新情報はまだありません。')).toBeInTheDocument()
  })
})
