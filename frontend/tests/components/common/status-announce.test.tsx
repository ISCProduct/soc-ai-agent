/**
 * @jest-environment jsdom
 */
import { readdirSync, readFileSync, statSync } from 'fs'
import { join, relative } from 'path'
import { render, screen } from '@testing-library/react'
import { Alert } from '@/components/common/Alert'
import { PageLoading } from '@/components/common/PageLoading'
import { ResumeRunningCard } from '@/app/resume/components/ResumeRunningCard'

// #2217: 読み込み中・待ち・結果の状態をスクリーンリーダーに届ける。
describe('Alert の role（severity ごと）', () => {
  it('error だけを role="alert"（割り込み）にする', () => {
    render(<Alert severity="error">保存できませんでした</Alert>)
    expect(screen.getByRole('alert')).toHaveTextContent('保存できませんでした')
  })

  it.each(['info', 'success', 'warning'] as const)('%s は role="status"（割り込まない）にする', (severity) => {
    render(<Alert severity={severity}>お知らせ</Alert>)
    expect(screen.getByRole('status')).toHaveTextContent('お知らせ')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('severity を省いたとき（MUI の既定は success）も role="status"', () => {
    render(<Alert>保存しました</Alert>)
    expect(screen.getByRole('status')).toHaveTextContent('保存しました')
  })

  it('画面ごとに role を明示したら、その指定を優先する', () => {
    render(
      <Alert severity="warning" role="alert">
        すぐに対応が要ります
      </Alert>,
    )
    expect(screen.getByRole('alert')).toHaveTextContent('すぐに対応が要ります')
  })
})

describe('学生画面は MUI の Alert を直接使わない', () => {
  // 直接使うと、info・success・warning も role="alert" になってしまう。管理画面・企業ポータル・掲載申請は対象外
  const EXCLUDE = ['app/admin/', 'components/admin/', 'company-portal', 'app/company-entry/', 'components/common/Alert.tsx']
  const root = join(__dirname, '..', '..', '..')
  const walk = (dir: string): string[] =>
    readdirSync(dir).flatMap((name) => {
      const path = join(dir, name)
      return statSync(path).isDirectory() ? walk(path) : path.endsWith('.tsx') ? [path] : []
    })
  const files = ['app', 'components']
    .flatMap((d) => walk(join(root, d)))
    .map((f) => relative(root, f).replaceAll('\\', '/'))
    .filter((f) => !EXCLUDE.some((x) => f.includes(x)))

  it('対象のファイルがある（検査が空振りしていない）', () => {
    expect(files.length).toBeGreaterThan(50)
  })

  it.each(files)('%s', (file) => {
    const src = readFileSync(join(root, file), 'utf8')
    const muiImports = [...src.matchAll(/import\s*\{([^}]*)\}\s*from\s*'@mui\/material'/g)].map((m) => m[1])
    for (const names of muiImports) {
      expect(names.split(',').map((s) => s.trim())).not.toContain('Alert')
    }
    expect(src).not.toMatch(/from\s*'@mui\/material\/Alert'/)
  })
})

describe('読み込み中・待ちの表示', () => {
  it('ページの読み込み画面は role="status" と aria-busy="true" を持つ', () => {
    render(<PageLoading message="マッチング結果を準備しています..." />)
    const status = screen.getByRole('status')
    expect(status).toHaveAttribute('aria-busy', 'true')
    expect(status).toHaveTextContent('マッチング結果を準備しています...')
  })

  it('職務経歴書の解析中は、根拠の無い秒数を出さず、画面を離れると中断することを伝える', () => {
    render(<ResumeRunningCard hasReport={false} fileName="a.pdf" targetLabel="A社" candidateLabel="新卒" onCancel={() => {}} />)
    const status = screen.getByRole('status')
    expect(status).toHaveTextContent('画面を離れると中断される')
    expect(status).not.toHaveTextContent('30〜60秒')
  })
})
