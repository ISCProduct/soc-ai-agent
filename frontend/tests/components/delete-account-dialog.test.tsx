/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { DeleteAccountDialog } from '@/components/profile/DeleteAccountDialog'

// #2212: 退会の処理中は、Esc・幕のタップ・キャンセルで閉じない。処理の前は閉じられる。
describe('DeleteAccountDialog', () => {
  const setup = (deleting: boolean) => {
    const onClose = jest.fn()
    const onConfirm = jest.fn()
    render(<DeleteAccountDialog open deleting={deleting} onClose={onClose} onConfirm={onConfirm} />)
    return { onClose, onConfirm }
  }
  const pressEscape = () => fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
  const clickBackdrop = () => {
    const backdrop = document.querySelector('.MuiBackdrop-root')
    if (!backdrop) throw new Error('幕（Backdrop）がありません')
    fireEvent.click(backdrop)
  }

  it('処理の前は Esc で閉じる', () => {
    const { onClose } = setup(false)
    pressEscape()
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('処理の前は幕のタップで閉じる', () => {
    const { onClose } = setup(false)
    clickBackdrop()
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('処理の前は「キャンセル」で閉じ、「退会する」で退会する', () => {
    const { onClose, onConfirm } = setup(false)
    fireEvent.click(screen.getByRole('button', { name: 'キャンセル' }))
    expect(onClose).toHaveBeenCalledTimes(1)
    fireEvent.click(screen.getByRole('button', { name: '退会する' }))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('処理中は Esc でも幕のタップでも閉じない', () => {
    const { onClose } = setup(true)
    pressEscape()
    clickBackdrop()
    expect(onClose).not.toHaveBeenCalled()
  })

  it('処理中はボタンを押せず、処理中であることを示す', () => {
    setup(true)
    expect(screen.getByRole('button', { name: 'キャンセル' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '退会処理中...' })).toBeDisabled()
  })
})
