/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import {
  ResultsEmptyView,
  ResultsErrorView,
  ResultsLoadingView,
  ResultsNoMatchView,
} from '@/app/results/components/ResultsStatusViews'

describe('ResultsStatusViews', () => {
  it('loading ビューは一覧の形の Skeleton と計算中であることを示す', () => {
    render(<ResultsLoadingView />)
    expect(screen.getByText('希望条件との一致度を計算しています')).toBeInTheDocument()
    expect(screen.getByRole('status', { name: 'おすすめ企業を読み込み中' })).toBeInTheDocument()
  })

  // #2220: 0件はエラーの見た目にせず、何が無いか・なぜか・次にすること（主な操作は1つ）を出す
  describe('empty ビュー（推薦0件）', () => {
    it('何が無いかを名指しし、エラー（alert）にしない', () => {
      render(<ResultsEmptyView reason="matching_results_empty" onBackToChat={() => {}} />)
      expect(screen.getByText('おすすめ企業がまだありません')).toBeInTheDocument()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    })

    it('根拠が足りないときは、回答の根拠が足りない旨と、診断チャットへの導線を主な操作として出す', () => {
      const onBackToChat = jest.fn()
      render(<ResultsEmptyView reason="insufficient_user_scores" onBackToChat={onBackToChat} />)
      expect(screen.getByText(/回答の根拠が足りません/)).toBeInTheDocument()
      const buttons = screen.getAllByRole('button')
      expect(buttons).toHaveLength(1)
      fireEvent.click(screen.getByRole('button', { name: '診断チャットを続ける' }))
      expect(onBackToChat).toHaveBeenCalledTimes(1)
    })

    it('診断直後の計算待ちでは「もう一度読み込む」を主にし、チャットへの導線は控えめに出す', () => {
      const onReload = jest.fn()
      const onBackToChat = jest.fn()
      render(<ResultsEmptyView reason="matching_results_empty" onBackToChat={onBackToChat} onReload={onReload} />)
      const primary = screen.getByRole('button', { name: 'もう一度読み込む' })
      expect(primary).toHaveClass('MuiButton-contained')
      expect(screen.getByRole('button', { name: '診断チャットに戻る' })).not.toHaveClass('MuiButton-contained')
      fireEvent.click(primary)
      expect(onReload).toHaveBeenCalledTimes(1)
    })

    it('企業側のデータが無いときは、公開を待つ旨を出す', () => {
      render(<ResultsEmptyView reason="insufficient_company_data" onBackToChat={() => {}} />)
      expect(screen.getByText(/公開済みの企業データがありません/)).toBeInTheDocument()
    })
  })

  // #2220: 本当のエラーだけを出す。role="alert"・字形・主な操作は「もう一度読み込む」で、3つ並べない
  describe('error ビュー（取得の失敗）', () => {
    it('role="alert" にエラー文と字形があり、主な操作は「もう一度読み込む」', () => {
      const onReload = jest.fn()
      render(<ResultsErrorView error="企業データの取得に失敗しました" onBack={() => {}} onReload={onReload} />)
      const alert = screen.getByRole('alert')
      expect(alert).toHaveTextContent('企業データの取得に失敗しました')
      expect(alert.querySelector('svg')).not.toBeNull()
      const primary = screen.getByRole('button', { name: 'もう一度読み込む' })
      expect(primary).toHaveClass('MuiButton-contained')
      fireEvent.click(primary)
      expect(onReload).toHaveBeenCalledTimes(1)
    })

    it('操作を3つ並べない（「最初からやり直す」を出さない）', () => {
      const onBack = jest.fn()
      render(<ResultsErrorView error="企業データの取得に失敗しました" onBack={onBack} onReload={() => {}} />)
      expect(screen.getAllByRole('button')).toHaveLength(2)
      expect(screen.queryByRole('button', { name: '最初からやり直す' })).not.toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: '診断チャットに戻る' }))
      expect(onBack).toHaveBeenCalledTimes(1)
    })
  })

  it('no-match ビューを表示する', () => {
    render(<ResultsNoMatchView onReset={() => {}} />)
    expect(screen.getByText('適合する企業が見つかりませんでした')).toBeInTheDocument()
  })
})
