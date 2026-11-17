/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { ResumeResultView } from '@/app/resume/components/ResumeResultView'
import type { WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import type { ReviewResult } from '@/app/resume/types'

const scores: WeightScore[] = [{ weight_category: '細部志向', score: 60 }]

function buildReview(overrides: Partial<ReviewResult['review']> = {}): ReviewResult {
  return {
    review: { id: 1, score: 80, summary: '内容を確認しました。', ...overrides },
    items: [{ id: 1, page_number: 1, severity: 'info', message: '具体性を足してください', suggestion: '数値を添える' }],
    annotated_available: false,
  }
}

function renderView(review: ReviewResult, extra: Partial<React.ComponentProps<typeof ResumeResultView>> = {}) {
  return render(
    <ResumeResultView review={review} scoresBefore={null} scoresAfter={null} annotateError="" onDownload={() => {}} ragReport="" {...extra} />,
  )
}

describe('ResumeResultView', () => {
  // スコア無しのとき BE は user_weight_scores へ何も書かない（#1529）。
  // 画面が「反映されました」と出すと、既存スコア行があるユーザーに虚偽の主張をすることになる。
  it('スコア無しならプロフィール反映バナーを出さず、指摘は届ける', () => {
    renderView(buildReview({ score: null }), { scoresBefore: scores, scoresAfter: scores })

    expect(screen.getByText(/総合スコアを算出できませんでした/)).toBeInTheDocument()
    expect(screen.queryByText(/プロフィールスコアに反映されました/)).not.toBeInTheDocument()
    expect(screen.getAllByText('具体性を足してください').length).toBeGreaterThan(0)
  })

  it('スコアありならバナーと総合スコアを出す', () => {
    renderView(buildReview({ score: 80 }), { scoresBefore: scores, scoresAfter: scores })

    expect(screen.getByText(/プロフィールスコアに反映されました/)).toBeInTheDocument()
    expect(screen.getByText('80')).toBeInTheDocument()
    expect(screen.getByText('/100')).toBeInTheDocument()
    expect(screen.queryByText(/総合スコアを算出できませんでした/)).not.toBeInTheDocument()
  })

  it('内訳があれば開いて項目別スコアを見られ、無ければ内訳の入口を出さない', () => {
    const { unmount } = renderView(buildReview({ item_scores_json: JSON.stringify({ specificity: 4, readability: 2 }) }))
    fireEvent.click(screen.getByRole('button', { name: /評価の内訳/ }))
    expect(screen.getByText('具体性')).toBeInTheDocument()
    expect(screen.getByText('4 / 5')).toBeInTheDocument()
    unmount()

    // #1529 以前の行は item_scores_json が無い → 内訳なしに倒す
    renderView(buildReview())
    expect(screen.queryByText('評価の内訳')).not.toBeInTheDocument()
  })

  it('選んだ指摘に、いまの文章と書き換え案を、AIの提案として見比べられる形で出す', () => {
    const review = buildReview()
    review.items = [
      {
        id: 1,
        page_number: 2,
        severity: 'warning',
        message: '成果が伝わりにくいです',
        suggestion: '数値と期間を足す',
        quote: '売上を伸ばしました',
        rewrite: '店舗売上を1年で前年比120%に伸ばしました',
      },
    ]
    renderView(review)

    expect(screen.getByText('いまの文章')).toBeInTheDocument()
    expect(screen.getByText('書き換え案（AIの提案）')).toBeInTheDocument()
    expect(screen.getByText('店舗売上を1年で前年比120%に伸ばしました')).toBeInTheDocument()
    expect(screen.getByText(/事実と違う内容になっていないか/)).toBeInTheDocument()
    expect(screen.getByText('補足: 数値と期間を足す')).toBeInTheDocument()
    expect(screen.queryByText(/改善案:/)).not.toBeInTheDocument()
  })

  it('読み取りが不確かな行には書き換え案を出さない', () => {
    const review = buildReview()
    review.items = [
      {
        id: 1,
        page_number: 1,
        severity: 'info',
        message: 'この行は判読できません',
        quote: '売土を伸ばしましt',
        rewrite: '売上を伸ばしました',
        rewrite_withheld: 'low_ocr_confidence',
      },
    ]
    renderView(review)

    expect(screen.getByText(/読み取りの信頼度が低いため、書き換え案は出していません/)).toBeInTheDocument()
    expect(screen.queryByText('売上を伸ばしました')).not.toBeInTheDocument()
    expect(screen.queryByText('書き換え案（AIの提案）')).not.toBeInTheDocument()
  })

  it('ルールで見つけた形式の指摘も、同じ一覧に「ルールで検出」と分けて並ぶ', () => {
    const review = buildReview()
    review.format_findings = [
      { rule: 'spoken_language', severity: 'warning', start: 0, end: 3, message: '文末が混ざっています', fix: '「です・ます」にそろえる', page_hint: 1, excerpt: 'だ。です。' },
    ]
    renderView(review)

    expect(screen.getByText('ルールで検出')).toBeInTheDocument()
    expect(screen.getAllByText('文末が混ざっています').length).toBeGreaterThan(0)
  })

  it('重要度で絞り込める', () => {
    const review = buildReview()
    review.items = [
      { id: 1, page_number: 1, severity: 'critical', message: '重大な指摘です' },
      { id: 2, page_number: 1, severity: 'info', message: '情報の指摘です' },
    ]
    renderView(review)

    fireEvent.click(screen.getByRole('button', { name: '情報 1' }))
    expect(screen.getByRole('button', { name: '情報の指摘です' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '重大な指摘です' })).not.toBeInTheDocument()
    expect(screen.getByText('指摘 1 / 1 を表示中')).toBeInTheDocument()
  })

  it('次の指摘へ進むと、書類のページも指摘のあるページに移る', () => {
    const review = buildReview()
    review.items = [
      { id: 1, page_number: 1, severity: 'warning', message: '1ページ目の指摘', quote: '一ページ目の原文' },
      { id: 2, page_number: 2, severity: 'warning', message: '2ページ目の指摘', quote: '二ページ目の原文' },
    ]
    renderView(review)
    expect(screen.getByText('1 / 2 ページ')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /次の指摘/ }))

    expect(screen.getByText('2 / 2 ページ')).toBeInTheDocument()
    expect(screen.getByText('指摘 2 / 2 を表示中')).toBeInTheDocument()
  })

  it('注釈つきPDFがあるときだけダウンロードを出し、押せる', () => {
    const onDownload = jest.fn()
    const review = buildReview()
    review.annotated_available = true
    renderView(review, { onDownload })

    fireEvent.click(screen.getByRole('button', { name: /注釈つきPDFをダウンロード/ }))
    expect(onDownload).toHaveBeenCalledTimes(1)
  })
})
