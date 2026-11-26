import { readdirSync, readFileSync, statSync } from 'fs'
import { join, relative } from 'path'

/**
 * MUI Dialog のアクセシブルな名前のガード（#2101 / SOCAIAGENT-535）。
 *
 * `<Dialog>` に aria-labelledby（または aria-label）が無いと、スクリーンリーダーは
 * 「ダイアログ」とだけ読み上げ、何のダイアログかが伝わらない。
 * 新しく足したダイアログで付け忘れないよう、ソースの開始タグを検査する。
 *
 * - 開始タグに aria-labelledby か aria-label があること
 * - aria-labelledby / aria-describedby に固定の文字列を書いたときは、同じファイルに
 *   その id を持つ要素があること（DialogTitle に id を付け忘れると名前が空になる）
 * - 固定の id を複数のダイアログで使い回さないこと（同じ画面で重なると別の要素を指す）
 */
const FRONTEND_ROOT = join(__dirname, '..', '..')
const SCAN_DIRS = ['app', 'components']

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    return statSync(path).isDirectory() ? walk(path) : path.endsWith('.tsx') ? [path] : []
  })
}

/**
 * `<Dialog` の開始タグ（属性を含めて `>` まで）を取り出す。
 * 属性の値の `{...}` の中の `=>` や文字列、属性の間のコメントで止まらないよう、
 * 波かっこの深さ・引用符・コメントを追って、深さ 0 の `>` で切る。
 */
function extractDialogOpenTags(src: string): string[] {
  const tags: string[] = []
  const re = /<Dialog(?=[\s>/])/g
  let m: RegExpExecArray | null
  while ((m = re.exec(src)) !== null) {
    let depth = 0
    let quote: string | null = null
    let i = m.index + m[0].length
    for (; i < src.length; i++) {
      const c = src[i]
      if (quote) {
        if (c === '\\') i++
        else if (c === quote) quote = null
        continue
      }
      if (c === '"' || c === "'" || c === '`') {
        quote = c
      } else if (c === '/' && src[i + 1] === '/') {
        const end = src.indexOf('\n', i)
        i = end === -1 ? src.length : end
      } else if (c === '/' && src[i + 1] === '*') {
        const end = src.indexOf('*/', i + 2)
        i = end === -1 ? src.length : end + 1
      } else if (c === '{') {
        depth++
      } else if (c === '}') {
        depth--
      } else if (c === '>' && depth === 0) {
        break
      }
    }
    tags.push(src.slice(m.index, i + 1))
  }
  return tags
}

function hasAccessibleName(tag: string): boolean {
  return /\saria-label(ledby)?=/.test(tag)
}

function literalAttr(tag: string, name: string): string | null {
  const m = tag.match(new RegExp(`\\s${name}="([^"]+)"`))
  return m ? m[1] : null
}

type DialogTag = { file: string; tag: string }

function dialogTags(): DialogTag[] {
  return SCAN_DIRS.flatMap((dir) => walk(join(FRONTEND_ROOT, dir)))
    .flatMap((path) => {
      const file = relative(FRONTEND_ROOT, path).split('\\').join('/')
      return extractDialogOpenTags(readFileSync(path, 'utf8')).map((tag) => ({ file, tag }))
    })
}

describe('開始タグの取り出し', () => {
  it('属性の中の => やコメントで止まらず、開始タグの終わりまで取り出す', () => {
    const src = [
      '<Dialog',
      '  open={open}',
      '  onClose={() => {}} // 閉じられないようにする',
      '  aria-labelledby="x-title"',
      '>',
      '  <DialogTitle id="x-title">t</DialogTitle>',
      '</Dialog>',
      '<DialogTitle>別の部品は対象外</DialogTitle>',
    ].join('\n')
    const tags = extractDialogOpenTags(src)
    expect(tags).toHaveLength(1)
    expect(tags[0]).toContain('aria-labelledby="x-title"')
    expect(tags[0].endsWith('>')).toBe(true)
  })

  it('aria 属性の無いタグはそのまま取り出す（検査で落ちる）', () => {
    const tags = extractDialogOpenTags('<Dialog open={a > b} onClose={close}>')
    expect(tags).toEqual(['<Dialog open={a > b} onClose={close}>'])
    expect(hasAccessibleName(tags[0])).toBe(false)
  })
})

describe('MUI Dialog のアクセシブルな名前', () => {
  const tags = dialogTags()

  it('検査の対象のダイアログが見つかる（走査が壊れていない）', () => {
    expect(tags.length).toBeGreaterThan(0)
    expect(tags.map((t) => t.file)).toContain('app/results/components/LowMatchConfirmDialog.tsx')
  })

  it('すべての <Dialog> に aria-labelledby か aria-label がある', () => {
    const missing = tags.filter(({ tag }) => !hasAccessibleName(tag)).map(({ file }) => file)
    expect(missing).toEqual([])
  })

  it('aria-labelledby / aria-describedby の id を持つ要素が、同じファイルにちょうど1つある', () => {
    const broken: string[] = []
    for (const { file, tag } of tags) {
      const src = readFileSync(join(FRONTEND_ROOT, file), 'utf8')
      for (const attr of ['aria-labelledby', 'aria-describedby']) {
        const id = literalAttr(tag, attr)
        if (!id) continue
        const count = src.split(` id="${id}"`).length - 1
        if (count !== 1) broken.push(`${file}: ${attr}="${id}" の要素が ${count} 個`)
      }
    }
    expect(broken).toEqual([])
  })

  it('固定の aria-labelledby / aria-describedby の id を、複数のダイアログで使い回していない', () => {
    const owners = new Map<string, string[]>()
    for (const { file, tag } of tags) {
      for (const attr of ['aria-labelledby', 'aria-describedby']) {
        const id = literalAttr(tag, attr)
        if (!id) continue
        owners.set(id, [...(owners.get(id) ?? []), file])
      }
    }
    const shared = [...owners].filter(([, files]) => files.length > 1).map(([id, files]) => `${id}: ${files.join(', ')}`)
    expect(shared).toEqual([])
  })
})
