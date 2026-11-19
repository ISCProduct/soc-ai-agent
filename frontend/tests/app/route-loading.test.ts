/** @jest-environment node */
import { existsSync, readFileSync } from 'fs'
import { join } from 'path'

/**
 * 主要なルートに segment の loading.tsx があり、遷移先の読み込み中にすぐ表示が出ること（#2084）。
 * 無いと、遷移のあいだ前の画面のまま止まって見え、表示はアプリ全体の app/loading.tsx 任せになる。
 */
const APP = join(__dirname, '..', '..', 'app')

const STUDENT_ROUTES = [
  // もとからあるもの
  'interview',
  'profile',
  'results',
  'resume',
  'correlation-diagram',
  // #2084 で足したもの
  'applications',
  'schedule',
  'chat-history',
  'es-rewrite',
  'scout',
  'company',
]

describe('ルートごとの loading.tsx (#2084)', () => {
  it.each(STUDENT_ROUTES)('学生画面の /%s に loading.tsx があり、RouteLoading を使う', (route) => {
    const file = join(APP, route, 'loading.tsx')
    expect(existsSync(file)).toBe(true)
    const src = readFileSync(file, 'utf8')
    expect(src).toContain("from '@/components/common/RouteLoading'")
    expect(src).toMatch(/<RouteLoading message="[^"]+を読み込み中\.\.\." \/>/)
  })

  it('管理画面の loading.tsx は、管理画面のレイアウトの中に出るので全画面・ブランド名にしない', () => {
    const src = readFileSync(join(APP, 'admin', 'loading.tsx'), 'utf8')
    expect(src).toContain('<PageLoading')
    expect(src).toContain('showBrand={false}')
    expect(src).toContain('fullScreen={false}')
  })
})
