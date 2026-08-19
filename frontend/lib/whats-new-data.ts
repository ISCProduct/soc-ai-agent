import { authService } from '@/lib/auth'

export interface WhatsNewEntry {
  title: string
  summary: string
  merged_at: string
}

export async function fetchWhatsNewEntries(): Promise<WhatsNewEntry[]> {
  const res = await fetch('/api/whats-new', {
    cache: 'no-store',
    headers: authService.getUserFetchHeaders(),
  })
  if (!res.ok) throw new Error('whats-new')
  return res.json() as Promise<WhatsNewEntry[]>
}

const LAST_SEEN_KEY = 'whatsNewLastSeenMergedAt'

export function hasUnreadWhatsNew(entries: WhatsNewEntry[]): boolean {
  if (typeof window === 'undefined' || entries.length === 0) return false
  const latest = entries[0].merged_at
  return window.localStorage.getItem(LAST_SEEN_KEY) !== latest
}

export function markWhatsNewAsSeen(entries: WhatsNewEntry[]): void {
  if (typeof window === 'undefined' || entries.length === 0) return
  window.localStorage.setItem(LAST_SEEN_KEY, entries[0].merged_at)
}
