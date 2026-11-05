/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import PageContent from '@/app/admin/student-insights/[id]/page-content'

const mockSearchParams = { value: new URLSearchParams('') }

jest.mock('next/navigation', () => ({
  useSearchParams: () => mockSearchParams.value,
}))

jest.mock('@/lib/auth', () => ({
  authService: { getAdminFetchHeaders: () => ({}) },
}))

const mockAccess = { restricted: true }
jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: () => Promise.resolve(mockAccess),
}))

describe('学生個別画面 (#1531)', () => {
  beforeEach(() => {
    global.fetch = jest.fn()
    mockSearchParams.value = new URLSearchParams('')
    mockAccess.restricted = true
  })
  afterEach(() => {
    jest.restoreAllMocks()
  })

  // 担当校ありの教員が school_id 無しで直リンクを開くと、以前はスピナーが
  // 回り続けて何も説明が出なかった（復帰手段も画面に無い）。
  it('担当校が指定されていなければ理由を出す（スピナーのまま止まらない）', async () => {
    render(<PageContent studentId="123" />)

    expect(await screen.findByText(/担当校が指定されていません/)).toBeInTheDocument()
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument()
    expect(global.fetch).not.toHaveBeenCalled()
  })

  // エラーと「履歴なし」を同時に出すと、教員がどちらを信じるか分からない。
  it('取得エラーのときに「履歴はまだありません」を同時に出さない', async () => {
    mockSearchParams.value = new URLSearchParams('school_id=101')
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: false })

    render(<PageContent studentId="123" />)

    expect(await screen.findByText(/ES添削の履歴を取得できませんでした/)).toBeInTheDocument()
    expect(screen.queryByText(/履歴はまだありません/)).not.toBeInTheDocument()
  })

  it('履歴が空なら「履歴はまだありません」を出す', async () => {
    mockSearchParams.value = new URLSearchParams('school_id=101')
    ;(global.fetch as jest.Mock).mockResolvedValue({ ok: true, json: async () => ({ es_reviews: [] }) })

    render(<PageContent studentId="123" />)

    expect(await screen.findByText(/履歴はまだありません/)).toBeInTheDocument()
  })

  it('履歴を新しい順に出し、未評価の企業適合性は出さない', async () => {
    mockSearchParams.value = new URLSearchParams('school_id=101')
    ;(global.fetch as jest.Mock).mockResolvedValue({
      ok: true,
      json: async () => ({
        es_reviews: [
          {
            id: 2, created_at: '2026-10-02T00:00:00Z', question: '設問B', question_type: 'ガクチカ',
            specificity_score: 8, star_score: 7, length_balance_score: 6, company_fit_score: null,
            feedback: 'フィードバックB', improved_text: '', es_text: '',
          },
        ],
      }),
    })

    render(<PageContent studentId="123" />)

    await waitFor(() => expect(screen.getByText('設問B')).toBeInTheDocument())
    expect(screen.getByText('フィードバックB')).toBeInTheDocument()
    // 未評価を 0 と出すと「低く評価された」に見える。
    expect(screen.queryByText('企業適合')).not.toBeInTheDocument()
  })
})
