import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

export interface StudentGuidance {
  id: number
  kind: string
  message: string
  suggested_industries?: string[]
  created_at: string
}

export async function fetchActiveGuidances(): Promise<StudentGuidance[]> {
  const res = await fetch(`${BACKEND_URL}/api/user/guidances`, {
    cache: 'no-store',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('guidances')
  const data = (await res.json()) as { guidances?: StudentGuidance[] }
  return data.guidances ?? []
}

export async function dismissGuidance(id: number): Promise<void> {
  const res = await fetch(`${BACKEND_URL}/api/user/guidances/${id}/dismiss`, {
    method: 'POST',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('dismiss-guidance')
}

export async function sendTeacherGuidance(
  studentId: number,
  body: { kind: string; message?: string; suggested_industries?: string[] },
): Promise<void> {
  const res = await fetch(`/api/admin/teacher/students/${studentId}/guidances`, {
    method: 'POST',
    headers: {
      ...authService.getAdminFetchHeaders(),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(body),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as { message?: string; error?: string }
    throw new Error(data.message || data.error || '案内の送信に失敗しました')
  }
}

/**
 * canDismiss は案内を閉じる操作を受け付けてよいかを返す。
 *
 * 学生側のホームは閉じるボタンに進行中の状態を持っておらず、連打すると
 * 同じ案内へ何度も POST が飛んでいた（教員側の送信ボタンは disabled で
 * 防いでいるのに、学生側だけ抜けていた）。
 *
 * app/page-content.tsx は jsdom で描画できない（依存が重く OOM する）ため、
 * 判定だけをここに置いてテストする。
 */
export function canDismiss(dismissingIds: Set<number>, id: number): boolean {
  return !dismissingIds.has(id)
}

/**
 * withDismissing / withoutDismissing は進行中IDの集合を更新する。
 * Set を直接変更すると React が再描画しないため、必ず新しい Set を返す。
 */
export function withDismissing(dismissingIds: Set<number>, id: number): Set<number> {
  const next = new Set(dismissingIds)
  next.add(id)
  return next
}

export function withoutDismissing(dismissingIds: Set<number>, id: number): Set<number> {
  const next = new Set(dismissingIds)
  next.delete(id)
  return next
}
