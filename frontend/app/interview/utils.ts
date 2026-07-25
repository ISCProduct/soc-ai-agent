/**
 * 面接機能向けのヘルパー（React state 非依存。単体テストしやすい境界）。
 */
import type { InterviewCompany } from './types'

/**
 * DB 登録企業と企業名が一致すれば id を解決する。
 * 未登録・検索失敗時は `id=0` のまま名前だけ返す（#567）。
 */
export async function resolveCompanyByName(
  name: string,
  fallback?: Partial<InterviewCompany>,
): Promise<InterviewCompany> {
  const trimmed = name.trim()
  if (!trimmed) {
    return { id: 0, name: '', ...fallback }
  }
  try {
    const params = new URLSearchParams({ limit: '20', offset: '0', name: trimmed })
    const res = await fetch(`/api/companies?${params}`, { cache: 'no-store' })
    if (res.ok) {
      const data = await res.json()
      const list: InterviewCompany[] = Array.isArray(data?.companies) ? data.companies : []
      const exact = list.find((c) => c.name === trimmed)
      if (exact) {
        return { ...exact, ...fallback, id: exact.id, name: exact.name }
      }
    }
  } catch {
    // ignore — 解決失敗時は id=0
  }
  return { id: 0, name: trimmed, ...fallback }
}

/**
 * 面接アバターの性別を交互に切り替える。
 * localStorage のカウンタを進め、偶数目は female / 奇数目は male。
 */
export function getNextAvatarGender(): 'male' | 'female' {
  try {
    const key = 'interview_avatar_index'
    const current = Number(localStorage.getItem(key) || '0')
    const next = current + 1
    localStorage.setItem(key, String(next))
    return next % 2 === 0 ? 'female' : 'male'
  } catch {
    return 'male'
  }
}
