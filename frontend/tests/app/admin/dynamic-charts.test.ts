/**
 * @jest-environment node
 */
import fs from 'fs'
import path from 'path'

describe('管理画面の重いグラフは動的 import する', () => {
  const costs = fs.readFileSync(
    path.join(process.cwd(), 'app/admin/costs/page-content.tsx'),
    'utf-8',
  )
  const relations = fs.readFileSync(
    path.join(process.cwd(), 'app/admin/companies/[id]/relations/page-content.tsx'),
    'utf-8',
  )

  it('コスト画面は recharts をページ本体に静的 import しない', () => {
    expect(costs).not.toMatch(/from ['"]recharts['"]/)
    expect(costs).toMatch(/dynamic\(\s*\(\)\s*=>\s*import\('\.\/cost-bar-chart'\)/)
    expect(costs).toContain('ssr: false')
  })

  it('関連図画面は reactflow をページ本体に静的 import しない', () => {
    expect(relations).not.toMatch(/from ['"]reactflow['"]/)
    expect(relations).toMatch(
      /dynamic\(\s*\(\)\s*=>\s*import\('@\/components\/admin\/CompanyRelationGraph'\)/,
    )
    expect(relations).toContain('ssr: false')
  })
})
