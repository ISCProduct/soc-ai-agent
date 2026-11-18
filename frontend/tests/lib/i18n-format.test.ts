import { SUPPORTED_LOCALES } from '@/lib/i18n'
import { INTL_LOCALES, formatDate, formatDateTime, formatNumber } from '@/lib/i18n/format'

const options: Intl.DateTimeFormatOptions = {
  timeZone: 'Asia/Tokyo',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
}

describe('日付と数値のロケール', () => {
  it('辞書の言語と Intl のロケールが揃っている', () => {
    expect(Object.keys(INTL_LOCALES).sort()).toEqual([...SUPPORTED_LOCALES].sort())
  })

  it('日付と日時は ja-JP で、渡したタイムゾーンのまま出す', () => {
    const value = '2026-06-15T03:00:00Z'
    expect(formatDateTime(value, options)).toBe(new Date(value).toLocaleString('ja-JP', options))
    expect(formatDate(value, { timeZone: 'Asia/Tokyo', year: 'numeric', month: '2-digit', day: '2-digit' })).toBe(
      new Date(value).toLocaleDateString('ja-JP', { timeZone: 'Asia/Tokyo', year: 'numeric', month: '2-digit', day: '2-digit' }),
    )
  })

  it('数値は ja-JP の桁区切りにする', () => {
    expect(formatNumber(12000)).toBe('12,000')
  })
})