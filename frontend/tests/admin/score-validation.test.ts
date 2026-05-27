import { correlationLabel, correlationColor, formatPercent } from '@/app/admin/score-validation/page'

describe('correlationLabel', () => {
  it('強い相関を返す (|r| >= 0.7)', () => {
    expect(correlationLabel(0.8)).toBe('強い相関')
    expect(correlationLabel(-0.9)).toBe('強い相関')
    expect(correlationLabel(0.7)).toBe('強い相関')
  })

  it('中程度の相関を返す (0.4 <= |r| < 0.7)', () => {
    expect(correlationLabel(0.5)).toBe('中程度の相関')
    expect(correlationLabel(-0.6)).toBe('中程度の相関')
    expect(correlationLabel(0.4)).toBe('中程度の相関')
  })

  it('弱い相関を返す (0.2 <= |r| < 0.4)', () => {
    expect(correlationLabel(0.25)).toBe('弱い相関')
    expect(correlationLabel(-0.3)).toBe('弱い相関')
    expect(correlationLabel(0.2)).toBe('弱い相関')
  })

  it('ほぼ無相関を返す (|r| < 0.2)', () => {
    expect(correlationLabel(0.1)).toBe('ほぼ無相関')
    expect(correlationLabel(0)).toBe('ほぼ無相関')
    expect(correlationLabel(-0.05)).toBe('ほぼ無相関')
  })
})

describe('correlationColor', () => {
  it('強い相関は青色を返す', () => {
    expect(correlationColor(0.8)).toBe('#1976d2')
    expect(correlationColor(-0.75)).toBe('#1976d2')
  })

  it('中程度の相関は緑色を返す', () => {
    expect(correlationColor(0.5)).toBe('#388e3c')
    expect(correlationColor(-0.45)).toBe('#388e3c')
  })

  it('弱い相関はオレンジ色を返す', () => {
    expect(correlationColor(0.25)).toBe('#f57c00')
    expect(correlationColor(-0.3)).toBe('#f57c00')
  })

  it('ほぼ無相関はグレーを返す', () => {
    expect(correlationColor(0.1)).toBe('#9e9e9e')
    expect(correlationColor(0)).toBe('#9e9e9e')
  })
})

describe('formatPercent', () => {
  it('小数を百分率文字列に変換する', () => {
    expect(formatPercent(0.5)).toBe('50.0%')
    expect(formatPercent(0.123)).toBe('12.3%')
    expect(formatPercent(1)).toBe('100.0%')
    expect(formatPercent(0)).toBe('0.0%')
  })

  it('小数点以下1桁でフォーマットされる', () => {
    expect(formatPercent(0.666)).toBe('66.6%')
    expect(formatPercent(0.999)).toBe('99.9%')
  })
})
