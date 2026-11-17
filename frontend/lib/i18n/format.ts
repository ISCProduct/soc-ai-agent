/**
 * 日付と数値の表示ロケール（SOCAIAGENT-596）。
 *
 * 画面の文言は messages/ja.json、こちらは Intl に渡すロケール。
 * 辞書の Locale（lib/i18n の ja）とキーを揃える。タイムゾーンは呼び出し側の指定をそのまま使う。
 * 言語を増やすときは INTL_LOCALES に足す。プロフィールの言語設定や middleware の切替はまだ持たない。
 */
export const INTL_LOCALES = { ja: 'ja-JP' } as const

export type IntlLocaleKey = keyof typeof INTL_LOCALES

export function intlLocale(locale: IntlLocaleKey = 'ja'): string {
  return INTL_LOCALES[locale]
}

export function formatDateTime(
  value: Date | string | number,
  options?: Intl.DateTimeFormatOptions,
  locale: IntlLocaleKey = 'ja',
): string {
  const date = value instanceof Date ? value : new Date(value)
  return date.toLocaleString(intlLocale(locale), options)
}

export function formatDate(
  value: Date | string | number,
  options?: Intl.DateTimeFormatOptions,
  locale: IntlLocaleKey = 'ja',
): string {
  const date = value instanceof Date ? value : new Date(value)
  return date.toLocaleDateString(intlLocale(locale), options)
}

export function formatNumber(
  value: number,
  options?: Intl.NumberFormatOptions,
  locale: IntlLocaleKey = 'ja',
): string {
  return value.toLocaleString(intlLocale(locale), options)
}
