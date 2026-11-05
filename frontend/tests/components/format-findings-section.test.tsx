/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import FormatFindingsSection, { type FormatFinding } from '@/components/FormatFindingsSection'

const f = (over: Partial<FormatFinding>): FormatFinding => ({
  rule: 'mixed_style',
  severity: 'warning',
  start: 0,
  end: 1,
  message: 'メッセージ',
  fix: '',
  ...over,
})

describe('FormatFindingsSection', () => {
  it('format_findings が無い（古い応答）ときは何も表示しない', () => {
    const { container } = render(<FormatFindingsSection findings={undefined} />)
    expect(container.firstChild).toBeNull()
  })

  it('空なら「問題なし」と表示する', () => {
    render(<FormatFindingsSection findings={[]} />)
    expect(screen.getByText('必ず直す')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('形式の問題は見つかりませんでした')
  })

  it('重要度順に並べ、上位5件だけ表示して残りを折りたたむ', () => {
    const findings = [
      f({ severity: 'info', message: 'i1', start: 1 }),
      f({ severity: 'info', message: 'i2', start: 2 }),
      f({ severity: 'info', message: 'i3', start: 3 }),
      f({ severity: 'warning', message: 'w1', start: 1 }),
      f({ severity: 'warning', message: 'w2', start: 2 }),
      f({ severity: 'critical', message: 'c1', start: 9 }),
    ]
    render(<FormatFindingsSection findings={findings} />)
    let items = screen.getAllByTestId('format-finding')
    expect(items.map((el) => el.textContent?.match(/[ciw]\d/)?.[0])).toEqual(['c1', 'w1', 'w2', 'i1', 'i2'])
    expect(items[0]).toHaveTextContent('重大')

    fireEvent.click(screen.getByRole('button', { name: '残り1件を表示' }))
    items = screen.getAllByTestId('format-finding')
    expect(items).toHaveLength(6)
    fireEvent.click(screen.getByRole('button', { name: '折りたたむ' }))
    expect(screen.getAllByTestId('format-finding')).toHaveLength(5)
  })

  it('5件以下ならボタンを出さない', () => {
    render(<FormatFindingsSection findings={[f({}), f({ start: 2 })]} />)
    expect(screen.queryByRole('button')).toBeNull()
  })

  it('本文を渡すと、UTF-16 の位置で該当箇所を抜き出す', () => {
    // 「𠮷」は UTF-16 で2単位。「バイト」は 5〜8
    const text = '𠮷野家でバイトした。'
    render(<FormatFindingsSection findings={[f({ rule: 'spoken_language', start: 5, end: 8 })]} text={text} />)
    expect(screen.getByText('「バイト」')).toBeInTheDocument()
  })

  it('位置が本文の外なら抜き出さない', () => {
    render(<FormatFindingsSection findings={[f({ start: 3, end: 99 })]} text="短い" />)
    expect(screen.queryByText(/「/)).toBeNull()
  })

  it('履歴書ではページ番号と直し方を表示する', () => {
    render(
      <FormatFindingsSection findings={[f({ page_hint: 2, block_index: 0, fix: 'アルバイト' })]} showPage />,
    )
    expect(screen.getByText('ページ 2')).toBeInTheDocument()
    expect(screen.getByText('直し方: アルバイト')).toBeInTheDocument()
  })
})
