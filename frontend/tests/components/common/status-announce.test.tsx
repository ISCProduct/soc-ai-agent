/**
 * @jest-environment jsdom
 */
import { readdirSync, readFileSync, statSync } from 'fs'
import { join, relative } from 'path'
import { render, screen } from '@testing-library/react'
import { Alert } from '@/components/common/Alert'
import { PageLoading } from '@/components/common/PageLoading'

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
      // `Alert as MuiAlert` のような別名の import も取りこぼさない（#2217 のレビュー）
      const imported = names.split(',').map((s) => s.trim().split(/\s+as\s+/)[0].replace(/^type\s+/, ''))
      expect(imported).not.toContain('Alert')
    }
    expect(src).not.toMatch(/from\s*'@mui\/material\/Alert'/)
  })
})

// #2217 のレビュー: 操作の結果として後から出す通知（成功・Snackbar）と、失敗の通知（見た目が warning のものを含む）は、
// 要素ごと差し込まれても確実に読まれるよう role="alert" を明示する。画面を開いたときから出ている注意は status のまま
describe('操作の結果・失敗の通知は role="alert" を明示する', () => {
  const root = join(__dirname, '..', '..', '..')
  const ACTION_RESULT_FILES: Array<[string, number]> = [
    ['app/applications/page-content.tsx', 1], // 選考管理の Snackbar
    ['app/favorites/page-content.tsx', 1], // お気に入りの Snackbar
    ['app/results/components/ResultsListView.tsx', 1], // 結果の Snackbar
    ['app/forgot-password/page-content.tsx', 1], // 再設定メールの送信
    ['app/reset-password/page-content.tsx', 1], // パスワードの再設定
    ['components/auth/LoginPage.tsx', 2], // 確認メールの再送・新規登録の確認メール
    ['components/profile/ScoutPreferencesCard.tsx', 1], // 保存
    ['components/profile/TrainingConsentCard.tsx', 2], // 保存・文言の更新で保存しなかった
    ['app/profile/page-content.tsx', 2], // カレンダーの連携・解除の結果、連携の状態の取得の失敗
    ['app/ability-assessments/[sessionId]/page-content.tsx', 1], // 時間切れ（回答欄がすぐ無効になる）
    ['app/results/components/list/AnalysisErrorAlert.tsx', 1], // 分析の取得の失敗
    ['app/resume/components/ResumeResultSummary.tsx', 1], // 注記の取得の失敗
    ['app/interview/components/InterviewSummary.tsx', 1], // 該当発言が見つからない
    ['components/interview/AxisComparison.tsx', 1], // 同じ回を2つ選んだ
    ['components/es/EsTemplatePanel.tsx', 1], // 例文の操作の結果
    ['components/github-skills/components/GitHubRepoSummaries.tsx', 1], // 操作の失敗
    ['components/github-skills/components/GitHubSkillsView.tsx', 1], // 読み込みの失敗
    ['app/companies/page-content.tsx', 2], // 絞り込みの選択肢・お気に入りの読み込みの失敗
  ]
  it.each(ACTION_RESULT_FILES)('%s', (file, count) => {
    const src = readFileSync(join(root, file), 'utf8')
    expect((src.match(/role="alert"/g) ?? []).length).toBeGreaterThanOrEqual(count)
  })
})

describe('読み込み中・待ちの表示', () => {
  it('ページの読み込み画面は role="status" を持ち、aria-busy は付けない（busy の間は読み上げが抑えられるため）', () => {
    render(<PageLoading message="マッチング結果を準備しています..." />)
    const status = screen.getByRole('status')
    expect(status).not.toHaveAttribute('aria-busy')
    expect(status).toHaveTextContent('マッチング結果を準備しています...')
  })

})
