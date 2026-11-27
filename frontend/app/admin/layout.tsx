import { Suspense, type ReactNode } from 'react'
import { AdminAuthGate, AdminAuthLoading } from '@/components/admin/AdminAuthGate'

/**
 * 管理画面の共通レイアウト。
 *
 * 各 page.tsx も `requireAdminUser()` を呼んでいるが、ここでも通す（AdminAuthGate）。
 * 新しい管理画面を足したときに page.tsx 側の呼び出しを忘れても、
 * 認証なしで描かれることが無いようにするため（多重に通しても redirect は冪等）。
 * 認証を待つ間は、管理画面用の読み込み表示を出す（#2084）。
 */
export default function AdminLayout({ children }: { children: ReactNode }) {
  return (
    <Suspense fallback={<AdminAuthLoading />}>
      <AdminAuthGate>{children}</AdminAuthGate>
    </Suspense>
  )
}
