/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import { ResumePdfOverlay } from '@/app/resume/components/ResumePdfOverlay'
import type { Finding } from '@/app/resume/utils'

const base: Omit<Finding, 'no' | 'box'> = { page: 1, severity: 'warning', source: 'ai', message: '指摘', quote: '', rewrite: '', withheld: false, supplement: '' }

const placed: Finding = { ...base, no: 1, quote: '原文A', box: { x0: 100, y0: 200, x1: 300, y1: 260, pageWidth: 1000, pageHeight: 2000 } }
const unplaced: Finding = { ...base, no: 2, quote: '原文B', box: null }

describe('ResumePdfOverlay', () => {
  it('位置のある指摘だけを、ページに対する割合の位置に重ねる', () => {
    render(<ResumePdfOverlay findings={[placed, unplaced]} selectedNo={null} onSelect={() => {}} />)

    const button = screen.getByRole('button', { name: /指摘1を選ぶ/ })
    expect(screen.queryByRole('button', { name: /指摘2を選ぶ/ })).not.toBeInTheDocument()
    const box = button.parentElement as HTMLElement
    expect(box).toHaveStyle({ left: '10%', top: '10%', width: '20%', height: '3%' })
  })

  it('押すと、その指摘を選ぶ。選択中は aria-pressed で伝える', () => {
    const onSelect = jest.fn()
    const { rerender } = render(<ResumePdfOverlay findings={[placed]} selectedNo={null} onSelect={onSelect} />)
    fireEvent.click(screen.getByRole('button', { name: /指摘1を選ぶ/ }))
    expect(onSelect).toHaveBeenCalledWith(1)
    expect(screen.getByRole('button', { name: /指摘1を選ぶ/ })).toHaveAttribute('aria-pressed', 'false')

    rerender(<ResumePdfOverlay findings={[placed]} selectedNo={1} onSelect={onSelect} />)
    expect(screen.getByRole('button', { name: /指摘1を選ぶ/ })).toHaveAttribute('aria-pressed', 'true')
  })
})
