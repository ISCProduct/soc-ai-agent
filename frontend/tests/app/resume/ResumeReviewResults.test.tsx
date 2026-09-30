/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { ResumeReviewResults } from '@/app/resume/components/ResumeReviewResults'
import type { WeightScore } from '@/components/ScoreUpdateBanner'
import type { ReviewResult } from '@/app/resume/types'

const scores: WeightScore[] = [{ weight_category: '細部志向', score: 60 }]

function buildReview(overrides: Partial<ReviewResult['review']>): ReviewResult {
  return {
    review: {
      id: 1,
      score: 80,
      summary: '内容を確認しました。',
      ...overrides,
    },
    items: [
      { id: 1, page_number: 1, severity: 'info', message: '具体性を足してください', suggestion: '数値を添える' },
    ],
    annotated_available: false,
  }
}

describe('ResumeReviewResults', () => {
  // スコア無しのとき BE は user_weight_scores へ何も書かない（#1529）。
  // 画面が「反映されました」と出すと、既存スコア行があるユーザー（再訪の学生の大半）に
  // 虚偽の主張をすることになる。差分0件でもバナー本体は表示されてしまうため、
  // バナーそのものを出さない。
  it('スコア無しならプロフィール反映バナーを出さない', () => {
    render(
      <ResumeReviewResults
        review={buildReview({ score: null })}
        scoresBefore={scores}
        scoresAfter={scores}
        annotateError=""
        onDownload={() => {}}
      />,
    )

    expect(screen.getByText(/総合スコアを算出できませんでした/)).toBeInTheDocument()
    expect(screen.queryByText(/プロフィールスコアに反映されました/)).not.toBeInTheDocument()
    expect(screen.queryByText(/プロフィールに反映されました/)).not.toBeInTheDocument()
    // 指摘事項は届く
    expect(screen.getByText('具体性を足してください')).toBeInTheDocument()
  })

  it('スコアありならバナーと総合スコアを出す', () => {
    render(
      <ResumeReviewResults
        review={buildReview({ score: 80 })}
        scoresBefore={scores}
        scoresAfter={scores}
        annotateError=""
        onDownload={() => {}}
      />,
    )

    expect(screen.getByText(/プロフィールスコアに反映されました/)).toBeInTheDocument()
    expect(screen.getByText('総合スコア: 80 / 100')).toBeInTheDocument()
    expect(screen.queryByText(/総合スコアを算出できませんでした/)).not.toBeInTheDocument()
  })

  it('内訳があれば項目別スコアを表示し、無ければ内訳を出さない', () => {
    const { unmount } = render(
      <ResumeReviewResults
        review={buildReview({ item_scores_json: JSON.stringify({ specificity: 4, readability: 2 }) })}
        scoresBefore={null}
        scoresAfter={null}
        annotateError=""
        onDownload={() => {}}
      />,
    )
    expect(screen.getByText('評価の内訳')).toBeInTheDocument()
    expect(screen.getByText('具体性')).toBeInTheDocument()
    expect(screen.getByText('4 / 5')).toBeInTheDocument()
    unmount()

    // #1529 以前の行は item_scores_json が無い → 内訳なしに倒す
    render(
      <ResumeReviewResults
        review={buildReview({})}
        scoresBefore={null}
        scoresAfter={null}
        annotateError=""
        onDownload={() => {}}
      />,
    )
    expect(screen.queryByText('評価の内訳')).not.toBeInTheDocument()
  })
})
