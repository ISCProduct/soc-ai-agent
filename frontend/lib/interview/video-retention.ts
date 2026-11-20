import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

export type VideoRetention = {
  days: number | null
  unlimited: boolean
}

const ENDPOINT = `${BACKEND_URL}/api/user/interview-video-retention`

/** 同意文と面接結果に出す、学校の保持期間の一文。 */
export function videoRetentionSentence(retention: VideoRetention | null, status: 'loading' | 'error' | 'ready'): string {
  if (status === 'loading') return '録画を残す期間を確認しています。'
  if (status === 'error' || retention == null) {
    return '録画を残す期間を読み込めませんでした。学校が 30日・90日・180日・期限なし のどれかを選びます。選んでいない学校は90日です。退会したあとは約30日で削除します。'
  }
  if (retention.unlimited || retention.days == null) {
    return 'この学校では、録画の保管期限を決めていません。退会したあとは約30日で削除します。'
  }
  return `この学校では、録画を${retention.days}日で削除します。それより前に退会した場合は、退会後約30日で削除します。`
}

export async function fetchVideoRetention(): Promise<VideoRetention> {
  const response = await fetch(ENDPOINT, { headers: authService.getUserFetchHeaders() })
  if (!response.ok) throw new Error('failed')
  const data: unknown = await response.json()
  if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('invalid')
  const row = data as { days?: unknown; unlimited?: unknown }
  const unlimited = row.unlimited === true
  const days = typeof row.days === 'number' ? row.days : null
  if (!unlimited && days == null) throw new Error('invalid')
  return { days, unlimited }
}
