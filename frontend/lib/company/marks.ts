export type MarkTone = 'neutral' | 'attention' | 'progress' | 'stop'

export type StatusMark = {
  mark: string
  label: string
  tone: MarkTone
}

/** 求人の公開状態。色だけに頼らず字形を添える。 */
export function jobPublishMark(published: boolean): StatusMark {
  return published
    ? { mark: '●', label: '公開中', tone: 'progress' }
    : { mark: '○', label: '下書き', tone: 'neutral' }
}

/** 企業情報の掲載審査。 */
export function reviewMark(published: boolean): StatusMark {
  return published
    ? { mark: '●', label: '掲載審査 済', tone: 'progress' }
    : { mark: '○', label: '掲載審査 未', tone: 'attention' }
}

const APPLICATION_MARKS: Record<string, StatusMark> = {
  applied: { mark: '●', label: '応募済み', tone: 'attention' },
  document_screening: { mark: '◐', label: '書類選考中', tone: 'attention' },
  document_passed: { mark: '◐', label: '書類通過', tone: 'attention' },
  interview_scheduled: { mark: '◐', label: '面接予定', tone: 'attention' },
  interview_in_progress: { mark: '◐', label: '面接中', tone: 'attention' },
  offered: { mark: '✓', label: '内定', tone: 'progress' },
  accepted: { mark: '✓', label: '内定承諾', tone: 'progress' },
  rejected: { mark: '×', label: '不採用', tone: 'stop' },
  withdrawn: { mark: '○', label: '辞退', tone: 'neutral' },
  not_applied: { mark: '○', label: '未応募', tone: 'neutral' },
}

export function applicationMark(status: string): StatusMark {
  return APPLICATION_MARKS[status] ?? { mark: '○', label: status, tone: 'neutral' }
}

/** パイプライン帯の並び。選考の流れ順で、終了状態は後ろに置く。 */
export const PIPELINE_STAGES = [
  'applied',
  'document_screening',
  'document_passed',
  'interview_scheduled',
  'interview_in_progress',
  'offered',
  'accepted',
  'rejected',
  'withdrawn',
] as const

const SCOUT_MARKS: Record<string, StatusMark> = {
  sent: { mark: '○', label: '未読', tone: 'attention' },
  viewed: { mark: '◐', label: '既読', tone: 'neutral' },
  accepted: { mark: '✓', label: '承諾', tone: 'progress' },
  declined: { mark: '×', label: '辞退', tone: 'stop' },
}

export function scoutMark(status: string): StatusMark {
  return SCOUT_MARKS[status] ?? { mark: '○', label: status, tone: 'neutral' }
}

export function memberMark(member: { disabled: boolean; invite_pending: boolean }): StatusMark {
  if (member.disabled) return { mark: '×', label: '無効', tone: 'stop' }
  if (member.invite_pending) return { mark: '◐', label: '招待中', tone: 'attention' }
  return { mark: '●', label: '有効', tone: 'progress' }
}
