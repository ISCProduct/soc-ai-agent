/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import {
  ResultsEmptyView,
  ResultsErrorView,
  ResultsLoadingView,
  ResultsNoMatchView,
} from '@/app/results/components/ResultsStatusViews'

describe('ResultsStatusViews', () => {
  it('loading ビューに分析中メッセージを表示する', () => {
    render(<ResultsLoadingView />)
    expect(screen.getByText('AIが企業を分析中...')).toBeInTheDocument()
  })

  it('empty ビューに更新・チャット導線を表示する', () => {
    render(<ResultsEmptyView onBackToChat={() => {}} />)
    expect(screen.getByText(/データがありません/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'ページを更新' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'チャットに戻る' })).toBeInTheDocument()
  })

  it('error ビューにエラー文とリセット導線を表示する', () => {
    render(
      <ResultsErrorView
        error={'企業データの取得に失敗しました'}
        onBack={() => {}}
        onReset={() => {}}
      />,
    )
    expect(screen.getByText('企業データの取得に失敗しました')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '最初からやり直す' })).toBeInTheDocument()
  })

  it('no-match ビューを表示する', () => {
    render(<ResultsNoMatchView onReset={() => {}} />)
    expect(screen.getByText('適合する企業が見つかりませんでした')).toBeInTheDocument()
  })
})
