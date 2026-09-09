/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { LowMatchConfirmDialog } from '@/components/LowMatchConfirmDialog'

// 応募をブロックする機能ではない（#1028 のガードレール）。
// 「このまま応募する」が必ず選べること、「やめる」で応募が走らないことを固定する。
describe('LowMatchConfirmDialog', () => {
  const setup = () => {
    const onCancel = jest.fn()
    const onConfirm = jest.fn()
    render(
      <LowMatchConfirmDialog
        open
        companyName="株式会社サンプル"
        matchScore={32.4}
        onCancel={onCancel}
        onConfirm={onConfirm}
      />,
    )
    return { onCancel, onConfirm }
  }

  it('企業名とスコアを示す', () => {
    setup()
    expect(screen.getByText(/株式会社サンプル/)).toBeInTheDocument()
    expect(screen.getByText(/32点/)).toBeInTheDocument()
  })

  it('応募を続行できる', () => {
    const { onConfirm } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'このまま応募する' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('やめる選択で応募は実行されない', () => {
    const { onCancel, onConfirm } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'ほかの企業も見る' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  // 低マッチを理由に応募を諦めさせる文面にしない
  it('応募できないと誤解させる文言を含まない', () => {
    setup()
    const text = document.body.textContent ?? ''
    for (const ng of ['応募できません', '推奨しません', 'やめましょう']) {
      expect(text).not.toContain(ng)
    }
    expect(text).toContain('応募できないわけではありません')
  })
})
