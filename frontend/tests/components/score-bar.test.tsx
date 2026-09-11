/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { ScoreBar } from '@/components/admin/ScoreBar'

// 教員向け画面は数値がテキストで並ぶだけで大小が読み取れなかった(#1225)。
// バーの長さがスコアに対応していないと、比較できるという前提が崩れる。
describe('ScoreBar', () => {
  const barOf = (container: HTMLElement) =>
    container.querySelector('[role="img"] > div') as HTMLElement

  it('スコアに比例した幅でバーを描く', () => {
    const { container } = render(<ScoreBar label="技術志向" score={88} />)
    expect(barOf(container)).toHaveStyle({ width: '88%' })
  })

  it('スコアを数値でも併記する', () => {
    render(<ScoreBar label="技術志向" score={88} />)
    expect(screen.getByText('88')).toBeInTheDocument()
    expect(screen.getByText('技術志向')).toBeInTheDocument()
  })

  it('小数は第1位まで表示する', () => {
    render(<ScoreBar label="情報通信業" score={76.44} />)
    expect(screen.getByText('76.4')).toBeInTheDocument()
  })

  it('整数は小数点を付けない', () => {
    render(<ScoreBar label="医療・福祉" score={77} />)
    expect(screen.getByText('77')).toBeInTheDocument()
  })

  // 想定外の値でバーが枠を突き抜けたり負の幅になったりしないこと
  it.each([
    [120, '100%'],
    [-5, '0%'],
    [0, '0%'],
    [100, '100%'],
  ])('スコア %s は幅 %s に丸める', (score, want) => {
    const { container } = render(<ScoreBar label="x" score={score} />)
    expect(barOf(container)).toHaveStyle({ width: want })
  })

  it('スクリーンリーダー向けにスコアを読み上げられる', () => {
    render(<ScoreBar label="成長志向" score={69} />)
    expect(screen.getByRole('img', { name: '成長志向 69点' })).toBeInTheDocument()
  })
})
