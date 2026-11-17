import { authService } from '@/lib/auth'

/** アプリ内通知（#2033）。締切・選考結果・スカウト・先生の案内。 */
export type AppNotification = {
  id: number
  kind: 'application' | 'scout' | 'guidance' | 'deadline'
  title: string
  body: string
  link: string
  read_at: string | null
  created_at: string
}

export const NOTIFICATION_KIND_LABELS: Record<AppNotification['kind'], string> = {
  application: '選考',
  scout: 'スカウト',
  guidance: '先生から',
  deadline: '予定',
}

/** 通知の遷移先はアプリ内のパスだけを許す（外部 URL へは飛ばさない）。 */
export function safeNotificationLink(link: string): string | null {
  return link.startsWith('/') && !link.startsWith('//') ? link : null
}

export async function fetchUnreadCount(): Promise<number> {
  const res = await fetch('/api/notifications/unread-count', { headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`unread-count ${res.status}`)
  const data = (await res.json()) as { unread_count?: number }
  return data.unread_count ?? 0
}

/**
 * 既読の状態が変わったことを、同じページの他の表示（ヘッダのベルとホームのお知らせ）へ知らせる。
 * それぞれが別に一覧を持っているので、片方で既読にしたらもう片方も取り直す。
 */
export const NOTIFICATIONS_CHANGED_EVENT = 'app-notifications-changed'

export function emitNotificationsChanged(): void {
  if (typeof window !== 'undefined') window.dispatchEvent(new Event(NOTIFICATIONS_CHANGED_EVENT))
}

export async function fetchNotifications(limit = 20, beforeId?: number): Promise<{ notifications: AppNotification[]; unread_count: number }> {
  const params = new URLSearchParams({ limit: String(limit) })
  if (beforeId) params.set('before_id', String(beforeId))
  const res = await fetch(`/api/notifications?${params}`, { headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`notifications ${res.status}`)
  const data = (await res.json()) as { notifications?: AppNotification[]; unread_count?: number }
  return { notifications: data.notifications ?? [], unread_count: data.unread_count ?? 0 }
}

/** 未読だけを取る（種類を除ける）。新しい既読が多くても、古い未読を取りこぼさない。 */
export async function fetchUnreadNotifications(limit: number, excludeKinds: AppNotification['kind'][] = []): Promise<AppNotification[]> {
  const params = new URLSearchParams({ limit: String(limit), unread: '1' })
  for (const k of excludeKinds) params.append('exclude_kind', k)
  const res = await fetch(`/api/notifications?${params}`, { headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`notifications ${res.status}`)
  const data = (await res.json()) as { notifications?: AppNotification[] }
  return data.notifications ?? []
}

export async function markNotificationRead(id: number): Promise<void> {
  const res = await fetch(`/api/notifications/${id}/read`, { method: 'POST', headers: authService.getUserFetchHeaders() })
  if (!res.ok && res.status !== 404) throw new Error(`read ${res.status}`)
}

export async function markAllNotificationsRead(): Promise<void> {
  const res = await fetch('/api/notifications/read-all', { method: 'POST', headers: authService.getUserFetchHeaders() })
  if (!res.ok) throw new Error(`read-all ${res.status}`)
}
