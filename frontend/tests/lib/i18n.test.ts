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

  it('辞書に無いキーは空文字ではなくキーを返し、開発中はエラーを出す（型で防いでいるが念のため）', () => {
    const spy = jest.spyOn(console, 'error').mockImplementation(() => {})
    try {
      const t = createTranslator('nav') as unknown as (k: string) => string
      expect(t('sidebar.doesNotExist')).toBe('nav.sidebar.doesNotExist')
      expect(spy).toHaveBeenCalledWith(expect.stringContaining('辞書に無いキー: nav.sidebar.doesNotExist'))
    } finally {
      spy.mockRestore()
    }
  })

  // jest（SWC）は型を見ないので、このテスト自体は常に通る。型エラーの検出は、CI の E2E ジョブの
  // next build（型チェックあり）と、手元の tsc --noEmit で効く（#2162 のレビュー）。
  it('辞書に無いキーは型エラーになる（next build / tsc で検出）', () => {
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

describe('{name} の差し込みとプロトタイプ', () => {
  it('values に無いプレースホルダは、プロトタイプのプロパティ名でも差し込まない', () => {
    const original = MESSAGES.ja.nav.menu.current
    try {
      ;(MESSAGES.ja.nav.menu as { current: string }).current = '{constructor}と{toString}'
      const t = createTranslator('nav')
      expect(t('menu.current', { count: 1 })).toBe('{constructor}と{toString}')
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

  // messages/ にある辞書を全部読む。言語の JSON を足したのに登録し忘れた、構造がずれた、を検出する（#2162 のセルフレビュー）
  it('messages/ のすべての辞書が登録されていて、ja.json と同じ構造を持つ', () => {
    const dir = path.join(process.cwd(), 'messages')
    const files = fs.readdirSync(dir).filter((f) => f.endsWith('.json'))
    const expected = leafKeys(ja).sort()
    expect(files.map((f) => f.replace(/\.json$/, '')).sort()).toEqual([...SUPPORTED_LOCALES].sort())
    for (const file of files) {
      const dict: unknown = JSON.parse(fs.readFileSync(path.join(dir, file), 'utf8'))
      expect(leafKeys(dict).sort()).toEqual(expected)
    }
  })

  // 差し込みは {name} だけの簡易版。next-intl の ICU 形式（{n, plural, …}、' のエスケープ）を辞書に書くと、
  // next-intl へ移ったときに表示が変わるので書かない（#2162 のセルフレビュー）
  it("辞書に ICU 構文と ' を書いていない", () => {
    for (const locale of SUPPORTED_LOCALES) {
      for (const value of leafValues(MESSAGES[locale])) {
        const text = String(value)
        expect(text).not.toMatch(/\{[^}]*,/)
        expect(text).not.toContain("'")
      }
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
  'components/layout/GlobalNavMenu.tsx',
  'components/layout/StudentBottomNav.tsx',
  'app/layout.tsx',
]

/** コメントを取り除く（文字列の中の // は考えない簡易版。対象ファイルでは足りる）。 */
function stripComments(source: string): string {
  return source.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/.*$/gm, '$1')
}

// ひらがな・カタカナ・漢字・全角英数記号に加えて、日本語の句読点（、。「」）と半角カナも対象（#2162 のセルフレビュー）
const JAPANESE = /[　-〿぀-ヿ㐀-鿿！-｠｡-ﾟ]/

describe('辞書化したファイル', () => {
  it.each(DICTIONARY_ONLY_FILES)('%s に日本語の文言がベタ書きされていない', (file) => {
    const code = stripComments(fs.readFileSync(path.join(process.cwd(), file), 'utf8'))
    const offending = code.split('\n').filter((line) => JAPANESE.test(line))
    expect(offending).toEqual([])
  })
})

// ナビの文言を表示する部品に、辞書と同じ文言を直書きしない。ほかの日本語（ゲスト向けの案内など）が残っていて
// ファイルごとはチェックできない部品でも、ナビの項目名だけは辞書から引く（#2162 のセルフレビュー）。
const NAV_CONSUMER_FILES = ['components/home/AnalysisSidebar.tsx']

describe('ナビの文言を表示する部品', () => {
  const navLabels = leafValues(ja.nav).map(String)
  it.each(NAV_CONSUMER_FILES)('%s にナビの文言が直書きされていない', (file) => {
    const code = stripComments(fs.readFileSync(path.join(process.cwd(), file), 'utf8'))
    const hardcoded = navLabels.filter((label) => code.includes(`"${label}"`) || code.includes(`'${label}'`))
    expect(hardcoded).toEqual([])
  })
})
