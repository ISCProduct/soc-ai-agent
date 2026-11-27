/**
 * @jest-environment jsdom
 */
import { Suspense, type ComponentType, type ReactElement, type ReactNode } from 'react'
import { existsSync } from 'fs'
import { join } from 'path'
import { render, screen } from '@testing-library/react'
import AdminLayout from '@/app/admin/layout'
import AdminLoading from '@/app/admin/loading'
import { AdminAuthGate } from '@/components/admin/AdminAuthGate'
import { requireAdminUser } from '@/lib/auth/server'

jest.mock('@/lib/auth/server', () => ({
  requireAdminUser: jest.fn(),
}))
jest.mock('@/components/admin/AdminShell', () => ({
  AdminShell: ({ children }: { children: ReactNode }) => <div data-testid="admin-shell">{children}</div>,
}))

const mockRequireAdminUser = requireAdminUser as jest.MockedFunction<typeof requireAdminUser>

/**
 * 主要なルートに segment の loading.tsx があり、遷移先の読み込み中にすぐ表示が出ること（#2084）。
 * 無いと、遷移のあいだ前の画面のまま止まって見え、表示はアプリ全体の app/loading.tsx 任せになる。
 */
const APP = join(__dirname, '..', '..', 'app')
const BRAND = 'IT業界キャリアエージェント'

const STUDENT_ROUTES: [route: string, message: string][] = [
  // もとからあるもの
  ['interview', '面接ページを読み込み中...'],
  ['profile', 'プロフィールを読み込み中...'],
  ['results', 'マッチング結果を読み込み中...'],
  ['resume', '履歴書レビューを読み込み中...'],
  ['correlation-diagram', '企業相関図を読み込み中...'],
  // #2084 で足したもの
  ['applications', '選考管理を読み込み中...'],
  ['schedule', '選考スケジュールを読み込み中...'],
  ['chat-history', 'チャット履歴を読み込み中...'],
  ['es-rewrite', 'ESリライト・添削を読み込み中...'],
  ['scout', 'スカウトを読み込み中...'],
  ['company', '企業情報を読み込み中...'],
]

async function loadingOf(route: string): Promise<ComponentType> {
  return (await import(`@/app/${route}/loading`)).default
}

describe('ルートごとの loading.tsx (#2084)', () => {
  it.each(STUDENT_ROUTES)('学生画面の /%s は、読み込み中の表示（%s）を出す', async (route, message) => {
    // Next.js は segment のフォルダの loading.tsx を読み込み中の境界にする
    expect(existsSync(join(APP, route, 'loading.tsx'))).toBe(true)
    const Loading = await loadingOf(route)
    render(<Loading />)
    expect(screen.getByText(message)).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
  })

  it('管理画面の loading.tsx は、管理画面のレイアウトの中に出るので、学生画面のブランド名を出さない', () => {
    render(<AdminLoading />)
    expect(screen.getByText('読み込み中...')).toBeInTheDocument()
    expect(screen.getByRole('progressbar')).toBeInTheDocument()
    expect(screen.queryByText(BRAND)).not.toBeInTheDocument()
  })
})

// #2625 のレビュー: 管理画面の外から初めて開いたとき・再読み込みのときは、レイアウトが認証を待つ。
// その待ちがレイアウトの直下にあると、親の app/loading.tsx（学生画面のブランド名）が出ていた。
describe('管理画面のレイアウトの認証待ち', () => {
  beforeEach(() => {
    mockRequireAdminUser.mockReset()
  })

  it('認証を待つ処理を Suspense の内側に置き、待つ間は管理画面用の表示（ブランド名なし）を出す', () => {
    const tree = AdminLayout({ children: <p>本文</p> }) as ReactElement<{ fallback: ReactNode; children: ReactElement }>
    expect(tree.type).toBe(Suspense)
    expect(tree.props.children.type).toBe(AdminAuthGate)

    render(<>{tree.props.fallback}</>)
    expect(screen.getByText('読み込み中...')).toBeInTheDocument()
    expect(screen.queryByText(BRAND)).not.toBeInTheDocument()
  })

  it('管理者と確かめてから、管理画面のシェルの中に本文を描く', async () => {
    mockRequireAdminUser.mockResolvedValue({ user_id: 1 } as Awaited<ReturnType<typeof requireAdminUser>>)
    render(await AdminAuthGate({ children: <p>本文</p> }))
    expect(mockRequireAdminUser).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('admin-shell')).toHaveTextContent('本文')
  })

  it('管理者でなければ（requireAdminUser がリダイレクトで止めると）、シェルも本文も描かない', async () => {
    const redirect = new Error('NEXT_REDIRECT')
    mockRequireAdminUser.mockRejectedValue(redirect)
    await expect(AdminAuthGate({ children: <p>本文</p> })).rejects.toBe(redirect)
  })
})
