import type { ReactNode } from 'react'
import { requireAdminUser } from '@/lib/auth/server'
import { AdminShell } from '@/components/admin/AdminShell'
import { PageLoading } from '@/components/common/PageLoading'

/**
 * 管理画面の認証を待つ間の表示（#2084）。管理画面の外から初めて開いたとき・再読み込みのときに出る。
 * まだサイドバーは無いので全画面にするが、学生画面のブランド名は出さない。
 */
export function AdminAuthLoading() {
  return <PageLoading message="読み込み中..." showBrand={false} />
}

/**
 * 管理者であることを確かめてから、管理画面のシェル（サイドバー）を描く。
 * レイアウトの Suspense の内側に置き、待つ間は AdminAuthLoading を出す。認証を待つ処理がレイアウトの直下にあると、
 * その間は親の app/loading.tsx（学生画面の全画面表示とブランド名）が出てしまうため（#2625 のレビュー）。
 */
export async function AdminAuthGate({ children }: { children: ReactNode }) {
  await requireAdminUser()
  return <AdminShell>{children}</AdminShell>
}
