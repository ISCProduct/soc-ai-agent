/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import ScoreUpdateBanner from '@/components/ScoreUpdateBanner'
import type { WeightScore } from '@/components/ScoreUpdateBanner'

describe('ScoreUpdateBanner レンダリング (#1015)', () => {
  const after: WeightScore[] = [{ weight_category: '論理性', score: 60 }]

  it('beforeScoresがnull（スコアAPI失敗）の場合、比較不能である旨を表示する', () => {
    render(<ScoreUpdateBanner beforeScores={null} afterScores={after} />)

    expect(screen.getByText('スコア比較なし（更新前のスコア取得に失敗しました）')).toBeInTheDocument()
  })

  it('beforeScoresがあり変化がない場合は、通常の反映メッセージを表示する', () => {
    render(<ScoreUpdateBanner beforeScores={after} afterScores={after} />)

    expect(screen.getByText('今回のセッション結果がプロフィールに反映されました。')).toBeInTheDocument()
    expect(screen.queryByText('スコア比較なし（更新前のスコア取得に失敗しました）')).not.toBeInTheDocument()
  })

  it('afterScoresが空の場合は何も表示しない', () => {
    const { container } = render(<ScoreUpdateBanner beforeScores={null} afterScores={[]} />)
    expect(container).toBeEmptyDOMElement()
  })
})
