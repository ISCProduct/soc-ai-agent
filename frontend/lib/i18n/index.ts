/**
 * 多言語対応（i18n）の基盤（SOCAIAGENT-595）。
 *
 * - 画面の文言の正本は messages/ja.json。ほかの言語を足すときは、同じ構造の JSON を
 *   messages/ に置き、SUPPORTED_LOCALES と MESSAGES に加える（構造の一致はテストで確かめる）。
 * - 呼び出し方は next-intl と同じ名前・形（useTranslations('namespace') → t('key', { name })）に
 *   そろえてある。言語の切り替えが必要になったら、import 先を next-intl に替えて
 *   NextIntlClientProvider で包めば、辞書と、コンポーネントの中で useTranslations を呼んでいる箇所は使い回せる。
 * - ただし、モジュールの読み込み時に文言を固定している箇所は、そのままでは切り替わらない
 *   （lib/sidebar-nav.ts の label、app/layout.tsx の静的な metadata）。言語を切り替える段階で、
 *   label の代わりにキー（labelKey）を持たせて描画時に引く形と、generateMetadata へ直す必要がある（#2162 のレビュー）。
 * - 今は日本語だけなので、Provider は使わずに既定の言語の辞書を直接引く
 *   （Provider が無いと動かない形にすると、共通部品を描画する既存のテストをすべて包み直すことになる）。
 * - キーは辞書から型を作るので、辞書に無いキーを書くと型チェックで落ちる。CI では E2E ジョブの
 *   next build が型チェックを行う（jest は型を見ないので、jest だけでは検出しない）。
 */
// 辞書は静的に import するので、クライアントの部品から使うと辞書全体がバンドルに入る。今は小さいので
// このままにし、画面の辞書化や言語を増やす段階で、名前空間・言語ごとに分けて読み込む。
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
 *
 * 差し込みは {name} だけの簡易版で、next-intl の ICU 形式（{n, plural, …}、' によるエスケープ）は
 * 解釈しない。next-intl へ移るときに意味が変わらないよう、辞書には ICU 構文と ' を書かない
 * （tests/lib/i18n.test.ts で確かめる）。
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
    // 型で辞書にあるキーしか書けないが、辞書の取り違えに備えてキーを返す（空文字を出さない）。
    // 開発中は気づけるようにエラーを出す（本番ではログを汚さない）
    if (typeof raw !== 'string') {
      if (process.env.NODE_ENV !== 'production') {
        console.error(`[i18n] 辞書に無いキー: ${namespace}.${String(key)}（locale: ${locale}）`)
      }
      return `${namespace}.${String(key)}`
    }
    if (!values) return raw
    // in ではなく自分のプロパティかで見る。{constructor} のようなプレースホルダに、プロトタイプの値を
    // 差し込まないため（#2162 のレビュー）。Object.hasOwn は古い Safari（15.3 以前）に無いので使わない。
    return raw.replace(/\{(\w+)\}/g, (match, name: string) =>
      Object.prototype.hasOwnProperty.call(values, name) ? String(values[name]) : match,
    )
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
