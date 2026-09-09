import { readFileSync } from 'fs'
import { join } from 'path'

// tsconfig.json の include は `next dev` が自動で書き換える。
// 過去に `.next/dev/dev/types/**/*.ts` という dev が重複したパスが混入し、
// 存在しないディレクトリを参照したまま残っていた。
//
// tsc は include に存在しないパスがあってもエラーにしないため、
// 気づかないまま残る。静的に検出する。
describe('tsconfig.json の include', () => {
  const tsconfig = JSON.parse(
    readFileSync(join(__dirname, '..', 'tsconfig.json'), 'utf-8'),
  ) as { include: string[] }

  it('同じセグメントが連続するパスを含まない', () => {
    const duplicated = tsconfig.include.filter((pattern) => {
      const segments = pattern.split('/')
      return segments.some((seg, i) => i > 0 && seg === segments[i - 1])
    })
    expect(duplicated).toEqual([])
  })

  it('重複した項目を含まない', () => {
    expect(tsconfig.include).toEqual([...new Set(tsconfig.include)])
  })

  it('Next.js の型定義ディレクトリを参照している', () => {
    // これが消えると .next 配下の生成型が型検査から漏れる
    expect(tsconfig.include).toContain('.next/types/**/*.ts')
  })
})
