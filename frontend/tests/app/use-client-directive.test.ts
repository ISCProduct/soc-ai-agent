/** @jest-environment node */
import { readdirSync, readFileSync, statSync } from 'fs'
import { join, relative } from 'path'

/**
 * 'use client' の宣言はファイルの先頭に1回だけ書く（#2100）。
 * 2回目以降は効かない死んだ宣言で、どちらが効いているのか読み手を迷わせる。
 */
const ROOT = join(__dirname, '..', '..')

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    if (statSync(path).isDirectory()) return walk(path)
    return /\.(ts|tsx)$/.test(path) ? [path] : []
  })
}

const files = ['app', 'components', 'lib'].flatMap((d) => walk(join(ROOT, d)))

describe("'use client' の宣言 (#2100)", () => {
  it('検査の対象のファイルがある（空振りしていない）', () => {
    expect(files.length).toBeGreaterThan(100)
  })

  it("'use client' を2回以上書いているファイルが無い", () => {
    const duplicated = files
      .filter((f) => (readFileSync(f, 'utf8').match(/^['"]use client['"];?\s*$/gm) ?? []).length > 1)
      .map((f) => relative(ROOT, f))
    expect(duplicated).toEqual([])
  })
})
