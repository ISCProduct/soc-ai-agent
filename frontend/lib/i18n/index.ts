/**
 * 多言語対応（i18n）の基盤（SOCAIAGENT-595）。
 *
 * - 画面の文言の正本は messages/ja.json。ほかの言語を足すときは、同じ構造の JSON を
 *   messages/ に置き、SUPPORTED_LOCALES と MESSAGES に加える（構造の一致はテストで確かめる）。
 * - 呼び出し方は next-intl と同じ名前・形（useTranslations('namespace') → t('key', { name })）に
 *   そろえてある。言語の切り替えが必要になったら、import 先を next-intl に替えて
 *   NextIntlClientProvider で包めば、辞書と呼び出し側はそのまま使える。
 * - 今は日本語だけなので、Provider は使わずに既定の言語の辞書を直接引く
 *   （Provider が無いと動かない形にすると、共通部品を描画する既存のテストをすべて包み直すことになる）。
 * - キーは辞書から型を作るので、辞書に無いキーを書くと tsc で落ちる。
 */
import ja from '@/messages/ja.json'

/** 対応している言語。先頭が既定。 */
export const SUPPORTED_LOCALES = ['ja'] as const
export type Locale = (typeof SUPPORTED_LOCALES)[number]
export const DEFAULT_LOCALE: Locale = 'ja'

/** 辞書の形。ja.json を正本とする。 */
export type Messages = typeof ja

export const MESSAGES: Record<Locale, Messages> = { ja }

/** Open Graph の locale（例: ja_JP）。 */
const OG_LOCALES: Record<Locale, string> = { ja: 'ja_JP' }

/** <html lang> に入れる値。 */
export function htmlLang(locale: Locale = DEFAULT_LOCALE): string {
  return locale
}

/** Open Graph の locale を返す。 */
export function ogLocale(locale: Locale = DEFAULT_LOCALE): string {
  return OG_LOCALES[locale]
}

export function getMessages(locale: Locale = DEFAULT_LOCALE): Messages {
  return MESSAGES[locale]
}

/** 辞書の最上位のキー（metadata / nav など）。 */
export type Namespace = keyof Messages & string

/** オブジェクトの葉（文字列）までのキーを "a.b" の形で列挙する型。 */
type Leaves<T, P extends string = ''> = {
  [K in keyof T & string]: T[K] extends string ? `${P}${K}` : T[K] extends object ? Leaves<T[K], `${P}${K}.`> : never
}[keyof T & string]

/** 名前空間の中のキー（例: nav なら "sidebar.resume"、"menu.open"）。 */
export type MessageKey<N extends Namespace> = Leaves<Messages[N]>

/** {name} に差し込む値。 */
export type TranslationValues = Record<string, string | number>

export type Translator<N extends Namespace> = (key: MessageKey<N>, values?: TranslationValues) => string

/**
 * 名前空間を指定して、文言を引く関数を作る。
 * {name} は values の同名の値に置き換える（値が無ければそのまま残す）。
 */
export function createTranslator<N extends Namespace>(namespace: N, locale: Locale = DEFAULT_LOCALE): Translator<N> {
  const dict: unknown = getMessages(locale)[namespace]
  return (key, values) => {
    const raw = String(key)
      .split('.')
      .reduce<unknown>(
        (node: unknown, part: string) =>
          node !== null && typeof node === 'object' ? (node as Record<string, unknown>)[part] : undefined,
        dict,
      )
    // 型で辞書にあるキーしか書けないが、辞書の取り違えに備えてキーを返す（空文字を出さない）
    if (typeof raw !== 'string') return `${namespace}.${key}`
    if (!values) return raw
    return raw.replace(/\{(\w+)\}/g, (match, name: string) => (name in values ? String(values[name]) : match))
  }
}

/** クライアントコンポーネント用（next-intl の useTranslations と同じ形）。 */
export function useTranslations<N extends Namespace>(namespace: N): Translator<N> {
  return createTranslator(namespace)
}

/** サーバー側（metadata など）用（next-intl の getTranslations と同じ形）。 */
export async function getTranslations<N extends Namespace>(namespace: N): Promise<Translator<N>> {
  return createTranslator(namespace)
}
