/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import { ConfirmDialog } from '@/components/common/ConfirmDialog'

// window.confirm の代わりに使う確認ダイアログ（SOCAIAGENT-534）

function renderDialog(overrides: Partial<React.ComponentProps<typeof ConfirmDialog>> = {}) {
  const onConfirm = jest.fn()
  const onCancel = jest.fn()
  const utils = render(
    <ConfirmDialog
      open
      title="ユーザーを完全削除"
      message="a@example.com を完全削除します。よろしいですか？"
      confirmLabel="完全削除する"
      destructive
      onConfirm={onConfirm}
      onCancel={onCancel}
      {...overrides}
    />,
  )
  return { onConfirm, onCancel, ...utils }
}

/** 解決を外から制御できる Promise */
function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((r) => {
    resolve = r
  })
  return { promise, resolve }
}

describe('ConfirmDialog', () => {
  it('open のときタイトルと本文を出し、タイトルでラベル付けされたダイアログになる', () => {
    renderDialog()
    const dialog = screen.getByRole('dialog', { name: 'ユーザーを完全削除' })
    expect(dialog).toBeInTheDocument()
    expect(screen.getByText('a@example.com を完全削除します。よろしいですか？')).toBeInTheDocument()
    // aria-labelledby が DialogTitle の id を指している
    const labelledBy = dialog.getAttribute('aria-labelledby')
    expect(labelledBy).toBeTruthy()
    expect(document.getElementById(labelledBy!)).toHaveTextContent('ユーザーを完全削除')
  })

  it('open でなければ何も出さない', () => {
    renderDialog({ open: false })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('破壊的な操作では確定ボタンを error 色にする', () => {
    renderDialog()
    expect(screen.getByRole('button', { name: '完全削除する' })).toHaveClass('MuiButton-colorError')
  })

  it('破壊的でない操作では確定ボタンを error 色にしない', () => {
    renderDialog({ destructive: false, confirmLabel: '公開する' })
    const button = screen.getByRole('button', { name: '公開する' })
    expect(button).not.toHaveClass('MuiButton-colorError')
    expect(button).toHaveClass('MuiButton-colorPrimary')
  })

  it('初期フォーカスは安全側のキャンセルに置く', () => {
    renderDialog()
    expect(screen.getByRole('button', { name: 'キャンセル' })).toHaveFocus()
  })

  it('キャンセルを押すと onCancel だけを呼び、onConfirm は呼ばない', () => {
    const { onConfirm, onCancel } = renderDialog()
    fireEvent.click(screen.getByRole('button', { name: 'キャンセル' }))
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('Esc を押すと onCancel だけを呼び、onConfirm は呼ばない', () => {
    const { onConfirm, onCancel } = renderDialog()
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('確定を押すと onConfirm を呼び、onCancel は呼ばない', async () => {
    const { onConfirm, onCancel } = renderDialog()
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '完全削除する' }))
    })
    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(onCancel).not.toHaveBeenCalled()
  })

  it('処理中は閉じず、二重に押せない。終わると操作できる状態に戻る', async () => {
    const { promise, resolve } = deferred()
    const onConfirm = jest.fn(() => promise)
    const { onCancel } = renderDialog({ onConfirm })

    const confirmButton = screen.getByRole('button', { name: '完全削除する' })
    const cancelButton = screen.getByRole('button', { name: 'キャンセル' })
    fireEvent.click(confirmButton)
    fireEvent.click(confirmButton)

    expect(onConfirm).toHaveBeenCalledTimes(1)
    expect(confirmButton).toBeDisabled()
    expect(cancelButton).toBeDisabled()

    // 処理中は Esc でもキャンセルでも閉じない
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    fireEvent.click(cancelButton)
    expect(onCancel).not.toHaveBeenCalled()

    await act(async () => {
      resolve()
      await promise
    })
    expect(confirmButton).not.toBeDisabled()
    expect(cancelButton).not.toBeDisabled()
  })

  it('呼び出し側から busy を渡すと、確定もキャンセルもできない', () => {
    const { onConfirm, onCancel } = renderDialog({ busy: true })
    fireEvent.click(screen.getByRole('button', { name: '完全削除する' }))
    fireEvent.keyDown(screen.getByRole('dialog'), { key: 'Escape' })
    expect(onConfirm).not.toHaveBeenCalled()
    expect(onCancel).not.toHaveBeenCalled()
  })
})
