/** @jest-environment node */

import { readdirSync, readFileSync, statSync } from 'fs'
import { join } from 'path'

/**
 * アイコンだけの IconButton に名前があることを検査する（#1575）。
 *
 * aria-label も title も無いと、スクリーンリーダーでは用途が読み上げられない。
 * 見た目からも意味が取りにくい（削除なのか編集なのか分からない）。
 *
 * 29箇所あったものを一度に直したが、画面を足すたびに増える。
 * 人手の走査では追いつかないので、増えたらここで落とす。
 *
 * Tooltip で囲んであっても aria-label は要る。Tooltip の title は
 * ホバー・フォーカス時の視覚的な補助で、読み上げ名の代わりにはならない
 * （MUI は Tooltip の子へ aria-label を自動付与しない）。
 */

function tsxFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) {
      if (name === 'node_modules' || name === '.next') continue
      out.push(...tsxFiles(p))
    } else if (name.endsWith('.tsx')) {
      out.push(p)
    }
  }
  return out
}

/** `<IconButton` から対応する `>` までを返す。属性値の {} 内の `>` は読み飛ばす。 */
function openingTag(src: string, start: number): string {
  let i = start
  let depth = 0
  while (i < src.length) {
    const c = src[i]
    if (c === '{') depth++
    else if (c === '}') depth--
    else if (c === '>' && depth === 0) break
    i++
  }
  return src.slice(start, i)
}

describe('IconButton のアクセシブルな名前 (#1575)', () => {
  it('aria-label も title も無い IconButton が無い', () => {
    const missing: string[] = []

    for (const file of [...tsxFiles('app'), ...tsxFiles('components')]) {
      const src = readFileSync(file, 'utf8')
      const re = /<IconButton\b/g
      let m: RegExpExecArray | null
      while ((m = re.exec(src)) !== null) {
        const tag = openingTag(src, m.index)
        if (!/aria-label|title=|aria-labelledby/.test(tag)) {
          missing.push(`${file}:${src.slice(0, m.index).split('\n').length}`)
        }
      }
    }

    if (missing.length > 0) {
      throw new Error(
        `名前の無い IconButton が ${missing.length} 箇所あります。aria-label を付けてください。\n  ` +
          missing.join('\n  '),
      )
    }
  })
})
