/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import ScraperSessionsPage from '@/app/admin/scraper-sessions/page-content'
import ProfileRecalculationPage from '@/app/admin/profile-recalculation/page-content'

// #2099: スクレイパーセッション管理・プロファイル再計算の表が TableContainer 無しで直書きされ、
// スマホ幅で横にはみ出していた。表が横スクロールできる入れ物の中に描かれることを確かめる。

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

// システム管理者の判定は本テストの対象外。判定済み（表示してよい）として固定する。
jest.mock('@/lib/admin/use-require-platform-admin', () => ({
  useRequirePlatformAdmin: () => true,
}))

const originalFetch = global.fetch

function jsonResponse(body: unknown) {
  return Promise.resolve({ ok: true, status: 200, json: async () => body })
}

/** 表が MUI の TableContainer（overflow-x: auto）の中にあることを確かめる */
function expectInScrollContainer(table: HTMLElement) {
  expect(table.closest('.MuiTableContainer-root')).not.toBeNull()
}

afterEach(() => {
  global.fetch = originalFetch
})

describe('スクレイパーセッション管理の表 (#2099)', () => {
  it('セッション一覧の表が横スクロールできる入れ物の中にある', async () => {
    global.fetch = jest.fn().mockImplementation(() =>
      jsonResponse({
        sessions: [{ id: 1, site_key: 'career_tasu', cookies: 'a=b', updated_at: '2026-11-01T00:00:00Z' }],
      }),
    )
    render(<ScraperSessionsPage />)

    expect(await screen.findByText('career_tasu')).toBeInTheDocument()
    expectInScrollContainer(screen.getByRole('table'))
  })
})

describe('プロファイル再計算の表 (#2099)', () => {
  it('一括再計算の結果の表が横スクロールできる入れ物の中にある', async () => {
    global.fetch = jest.fn().mockImplementation(() =>
      jsonResponse({
        results: [{ company_id: 10, company_name: '株式会社テスト', updated: true, sample_count: 5 }],
        updated_count: 1,
        skipped_count: 0,
        total: 1,
      }),
    )
    render(<ProfileRecalculationPage />)

    fireEvent.click(screen.getByRole('button', { name: '一括再計算を実行' }))

    expect(await screen.findByText('株式会社テスト')).toBeInTheDocument()
    expectInScrollContainer(screen.getByRole('table'))
  })

  it('再計算履歴の表が横スクロールできる入れ物の中にある', async () => {
    global.fetch = jest.fn().mockImplementation(() =>
      jsonResponse({
        histories: [{ id: 3, company_id: 10, trigger: 'manual', sample_count: 5, created_at: '2026-11-01T00:00:00Z' }],
      }),
    )
    render(<ProfileRecalculationPage />)

    // 企業ID の入力欄は「個別再計算」と「再計算履歴」の2つある。後者は2つ目。
    fireEvent.change(screen.getAllByLabelText('企業ID')[1], { target: { value: '10' } })
    fireEvent.click(screen.getByRole('button', { name: '履歴を表示' }))

    expect(await screen.findByText('manual')).toBeInTheDocument()
    expectInScrollContainer(screen.getByRole('table'))
  })
})
