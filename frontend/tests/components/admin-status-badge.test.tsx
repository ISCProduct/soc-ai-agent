/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { StatusBadge } from '@/components/admin/StatusBadge'
import { FreshnessMark } from '@/components/admin/FreshnessMark'
import { EmptyState } from '@/components/common/EmptyState'
import { ErrorState } from '@/components/common/ErrorState'

describe('StatusBadge', () => {
  // §19: 色だけで状態を伝えない。Chip の color だけで分けていた状態を字形併記へ直した。
  // 色は jsdom では検証できないので、字形がラベルに入っていることで担保する。
  it.each([
    ['published', '公開'],
    ['rejected', '却下'],
    ['draft', '下書き'],
    ['started', '進行中'],
    ['finished', '完了'],
    ['error', 'エラー'],
  ])('%s は字形付きで表示する', (status, label) => {
    render(<StatusBadge status={status} />)
    const el = screen.getByText(new RegExp(label))
    // ラベルだけでなく字形が前置されていること
    expect(el.textContent).not.toBe(label)
    expect(el.textContent).toContain(label)
  })

  it('公開と却下が字形だけで区別できる', () => {
    const { unmount } = render(<StatusBadge status="published" />)
    const published = screen.getByText(/公開/).textContent
    unmount()
    render(<StatusBadge status="rejected" />)
    const rejected = screen.getByText(/却下/).textContent
    // 色を落としても別物として読めること
    expect(published?.replace('公開', '')).not.toBe(rejected?.replace('却下', ''))
  })

  it('未知の状態には色をつけず、値をそのまま出す', () => {
    render(<StatusBadge status="some_new_state" />)
    expect(screen.getByText('some_new_state')).toBeInTheDocument()
  })
})

describe('FreshnessMark', () => {
  it('ラベルと内訳を両方表示する', () => {
    render(<FreshnessMark freshness="expired" detail="92日経過" />)
    expect(screen.getByText('期限切れ')).toBeInTheDocument()
    expect(screen.getByText('92日経過')).toBeInTheDocument()
  })

  it('compact でも意味が読み上げに残る', () => {
    render(<FreshnessMark freshness="due" detail="あと3日" compact />)
    // 字形だけでは読めないため、role=img + aria-label で意味を渡している
    expect(screen.getByRole('img', { name: '期限が近い（あと3日）' })).toBeInTheDocument()
  })

  it('取得済みは compact でも列幅を保つ', () => {
    render(<FreshnessMark freshness="fresh" compact />)
    expect(screen.getByRole('img', { name: '取得済み' })).toBeInTheDocument()
  })
})

describe('EmptyState', () => {
  // §24: 「データがありません」で終わらせず、理由と次の操作を出す。
  it('理由と次の操作を表示する', () => {
    render(
      <EmptyState
        title="まだ応募企業がありません"
        description="気になる企業を探して応募候補に追加してみましょう。"
        action={<button type="button">企業を探す</button>}
      />,
    )
    expect(screen.getByText('まだ応募企業がありません')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '企業を探す' })).toBeInTheDocument()
  })

  it('操作が無くても表示できる', () => {
    render(<EmptyState title="対象の学生が見つかりません" />)
    expect(screen.getByText('対象の学生が見つかりません')).toBeInTheDocument()
  })
})

describe('ErrorState', () => {
  // §25: 何が起きたかと、何をすればよいかを出す。
  it('再試行を渡したときだけボタンを出す', () => {
    const onRetry = jest.fn()
    const { unmount } = render(
      <ErrorState
        title="企業情報を取得できませんでした"
        description="通信状態を確認して、もう一度お試しください。"
        onRetry={onRetry}
      />,
    )
    expect(screen.getByRole('alert')).toBeInTheDocument()
    screen.getByRole('button', { name: '再読み込み' }).click()
    expect(onRetry).toHaveBeenCalledTimes(1)
    unmount()

    render(<ErrorState title="この操作は許可されていません" />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
