import fs from 'node:fs'
import path from 'node:path'
import {
  DEFAULT_LOCALE,
  MESSAGES,
  SUPPORTED_LOCALES,
  createTranslator,
  getTranslations,
  htmlLang,
  ogLocale,
} from '@/lib/i18n'
import ja from '@/messages/ja.json'

// SOCAIAGENT-595: i18n の基盤。辞書（messages/ja.json）が正本で、呼び出しは next-intl と同じ形。

/** 辞書の葉（文字列）までのキーを "a.b.c" の形で集める。 */
function leafKeys(node: unknown, prefix = ''): string[] {
  if (typeof node === 'string') return [prefix]
  if (node === null || typeof node !== 'object') return [`${prefix}<不正な値>`]
  return Object.entries(node).flatMap(([k, v]) => leafKeys(v, prefix ? `${prefix}.${k}` : k))
}

function leafValues(node: unknown): unknown[] {
  if (node === null || typeof node !== 'object') return [node]
  return Object.values(node).flatMap(leafValues)
}

describe('createTranslator', () => {
  it('入れ子のキーで文言を引く', () => {
    const t = createTranslator('nav')
    expect(t('sidebar.resume')).toBe(ja.nav.sidebar.resume)
    expect(t('admin')).toBe(ja.nav.admin)
  })

  it('プレースホルダの無い文言に値を渡しても、文言はそのまま', () => {
    const t = createTranslator('nav')
    expect(t('menu.open', { unused: 1 })).toBe(ja.nav.menu.open)
  })

  it('辞書に無いキーは空文字ではなくキーを返す（型で防いでいるが念のため）', () => {
    const t = createTranslator('nav') as unknown as (k: string) => string
    expect(t('sidebar.doesNotExist')).toBe('nav.sidebar.doesNotExist')
  })

  it('辞書に無いキーは型エラーになる', () => {
    const t = createTranslator('nav')
    // @ts-expect-error 辞書に無いキー（tsc がこの行で落ちることを確かめる）
    t('sidebar.doesNotExist')
    // @ts-expect-error 名前空間の途中（文字列ではない）は指定できない
    t('sidebar')
    expect(true).toBe(true)
  })

  it('getTranslations（サーバー側）も同じ文言を返す', async () => {
    const t = await getTranslations('metadata')
    expect(t('siteName')).toBe(ja.metadata.siteName)
  })
})

describe('{name} の差し込み', () => {
  it('プレースホルダを値で置き換える', () => {
    const original = MESSAGES.ja.nav.menu.current
    try {
      ;(MESSAGES.ja.nav.menu as { current: string }).current = '{count}件を表示中（{missing}）'
      const t = createTranslator('nav')
      expect(t('menu.current', { count: 3 })).toBe('3件を表示中（{missing}）')
    } finally {
      ;(MESSAGES.ja.nav.menu as { current: string }).current = original
    }
  })
})

describe('辞書', () => {
  it('既定の言語は日本語で、<html lang> と OGP の locale が決まる', () => {
    expect(DEFAULT_LOCALE).toBe('ja')
    expect(htmlLang()).toBe('ja')
    expect(ogLocale()).toBe('ja_JP')
  })

  it('対応しているすべての言語の辞書が、ja.json と同じ構造を持つ', () => {
    const expected = leafKeys(ja).sort()
    for (const locale of SUPPORTED_LOCALES) {
      expect(leafKeys(MESSAGES[locale]).sort()).toEqual(expected)
    }
  })

  it('空の文言が無い', () => {
    for (const locale of SUPPORTED_LOCALES) {
      for (const value of leafValues(MESSAGES[locale])) {
        expect(typeof value === 'string' && value.trim().length > 0).toBe(true)
      }
    }
  })
})

// 辞書化したファイルに、画面の文言をベタ書きで戻さない（翻訳キー運用のチェック）。
// 辞書化を広げたら、ここにファイルを足す。
const DICTIONARY_ONLY_FILES = [
  'lib/sidebar-nav.ts',
  'components/GlobalNavMenu.tsx',
  'components/StudentBottomNav.tsx',
  'app/layout.tsx',
]

/** コメントを取り除く（文字列の中の // は考えない簡易版。対象ファイルでは足りる）。 */
function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1')
}

const JAPANESE = /[぀-ヿ㐀-鿿！-｠]/

describe('辞書化したファイル', () => {
  it.each(DICTIONARY_ONLY_FILES)('%s に日本語の文言がベタ書きされていない', (file) => {
    const code = stripComments(fs.readFileSync(path.join(process.cwd(), file), 'utf8'))
    const offending = code.split('\n').filter((line) => JAPANESE.test(line))
    expect(offending).toEqual([])
  })
})
