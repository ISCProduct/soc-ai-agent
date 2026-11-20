/** @jest-environment node */

import { readdirSync, readFileSync, statSync } from 'fs'
import { join } from 'path'

/**
 * 管理画面の MUI の表が、横スクロールできる入れ物（TableContainer）の中にあることを検査する（#2099）。
 *
 * MUI の <Table> は自分では横スクロールしない。TableContainer（overflow-x: auto）無しで置くと、
 * 列の多い表がスマホ幅で画面の外へはみ出す。users などは AdminTableWrapper
 * （中身は TableContainer + Table）で包んであるが、直書きの <Table> が残っていた。
 * 画面を足すたびに増えうるので、ここで落とす。
 *
 * components/ui/table の Table（shadcn。admin/costs が使う）は自分で overflow-x-auto の
 * div を持つので対象外。'@mui/material' から Table を import しているファイルだけを見る。
 */

function tsxFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const p = join(dir, name)
    if (statSync(p).isDirectory()) {
      out.push(...tsxFiles(p))
    } else if (name.endsWith('.tsx')) {
      out.push(p)
    }
  }
  return out
}

/** '@mui/material' からの名前付き import に Table が含まれるか */
function importsMuiTable(src: string): boolean {
  const re = /import\s*\{([^}]*)\}\s*from\s*'@mui\/material'/g
  let m: RegExpExecArray | null
  while ((m = re.exec(src)) !== null) {
    const names = m[1].split(',').map((s) => s.trim().split(/\s+as\s+/)[0])
    if (names.includes('Table')) return true
  }
  return false
}

function count(src: string, re: RegExp): number {
  return (src.match(re) ?? []).length
}

/** TableContainer の外に置かれた MUI の <Table> の行番号（1始まり）を返す */
function uncontainedTableLines(src: string): number[] {
  if (!importsMuiTable(src)) return []

  const lines: number[] = []
  const re = /<Table[\s>]/g
  let m: RegExpExecArray | null
  while ((m = re.exec(src)) !== null) {
    const before = src.slice(0, m.index)
    if (count(before, /<TableContainer[\s>]/g) <= count(before, /<\/TableContainer>/g)) {
      lines.push(before.split('\n').length)
    }
  }
  return lines
}

describe('管理画面の表の横スクロール (#2099)', () => {
  it('MUI の <Table> が TableContainer の外に直書きされていない', () => {
    const files = [...tsxFiles(join('app', 'admin')), ...tsxFiles(join('components', 'admin'))]
    const missing = files.flatMap((file) =>
      uncontainedTableLines(readFileSync(file, 'utf8')).map((line) => `${file}:${line}`),
    )

    if (missing.length > 0) {
      throw new Error(
        `TableContainer の外にある <Table> が ${missing.length} 箇所あります。` +
          'AdminTableWrapper か TableContainer で包んでください。\n  ' +
          missing.join('\n  '),
      )
    }
  })

  // 検査が常に空を返すようになっていないことを、修正前と同じ形の断片で確かめる。
  it('直書きの <Table> を見つけ、包まれたものと shadcn の Table は見逃す', () => {
    const bare = [
      "import { Table, TableBody } from '@mui/material'",
      '<Table size="small">',
      '<TableBody />',
      '</Table>',
    ].join('\n')
    const wrapped = [
      "import { Table, TableContainer } from '@mui/material'",
      '<TableContainer>',
      '<Table size="small">',
      '</Table>',
      '</TableContainer>',
    ].join('\n')
    const shadcn = ["import { Table } from '@/components/ui/table'", '<Table>', '</Table>'].join('\n')

    expect(uncontainedTableLines(bare)).toEqual([2])
    expect(uncontainedTableLines(wrapped)).toEqual([])
    expect(uncontainedTableLines(shadcn)).toEqual([])
  })
})
