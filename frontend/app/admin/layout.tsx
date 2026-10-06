import type { ReactNode } from 'react'
import { requireAdminUser } from '@/lib/auth/server'
import { AdminShell } from '@/components/admin/AdminShell'

/**
 * 管理画面の共通レイアウト。
 *
 * 各 page.tsx も `requireAdminUser()` を呼んでいるが、ここでも通す。
 * 新しい管理画面を足したときに page.tsx 側の呼び出しを忘れても、
 * 認証なしで描かれることが無いようにするため（多重に通しても redirect は冪等）。
 */
export default async function AdminLayout({ children }: { children: ReactNode }) {
  await requireAdminUser()
  return <AdminShell>{children}</AdminShell>
}
