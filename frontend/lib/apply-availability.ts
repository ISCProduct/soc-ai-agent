// 応募ボタンを開けているか（Backend の STUDENT_APPLY_ENABLED）。SOCAIAGENT-403。
// 取得できない・読めないときは閉じている側に倒す（開いていると誤表示して 403 を踏ませない）。

export const APPLY_CLOSED_LABEL = '応募は準備中'
export const APPLY_CLOSED_REASON = '応募機能は現在準備中です。公開までお待ちください。'

export function getApplyClosedButtonProps(enabled: boolean): {
  disabled: boolean
  title: string
} {
  return enabled ? { disabled: false, title: '' } : { disabled: true, title: APPLY_CLOSED_REASON }
}

export async function fetchApplyEnabled(headers: Record<string, string>): Promise<boolean> {
  try {
    const res = await fetch('/api/applications/availability', { headers, cache: 'no-store' })
    if (!res.ok) return false
    const data: { enabled?: unknown } = await res.json()
    return data.enabled === true
  } catch {
    return false
  }
}
