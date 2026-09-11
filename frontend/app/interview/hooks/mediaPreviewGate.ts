import type { InterviewStatus } from '../types'

/**
 * 面接遷移直後の getUserMedia 起動判定。
 * selection ではカメラ許可ダイアログを出さない。
 */
export function shouldStartInterviewMediaPreview(args: {
  loading: boolean
  status: InterviewStatus
}): boolean {
  if (args.loading) return false
  return args.status === 'lobby' || args.status === 'connecting' || args.status === 'connected'
}
