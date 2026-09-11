/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { WhatsNewView } from '@/app/whats-new/whats-new-view'

describe('WhatsNewView', () => {
  it('取得した更新情報を新しい順に表示する', () => {
    render(
      <WhatsNewView
        error={false}
        entries={[
          { title: '新機能A', summary: '説明A', merged_at: '2026-08-15T00:00:00Z' },
          { title: '過去の更新', summary: '説明B', merged_at: '2026-08-01T00:00:00Z' },
        ]}
      />,
    )

    expect(screen.getByText('新機能A')).toBeInTheDocument()
    expect(screen.getByText('過去の更新')).toBeInTheDocument()
  })

  it('取得失敗時はエラーメッセージを表示する', () => {
    render(<WhatsNewView entries={[]} error />)

    expect(screen.getByText(/取得に失敗しました/)).toBeInTheDocument()
  })

  it('更新情報が0件の場合はその旨を表示する', () => {
    render(<WhatsNewView entries={[]} error={false} />)

    expect(screen.getByText('更新情報はまだありません。')).toBeInTheDocument()
  })
})
