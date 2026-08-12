/** 選考ステータス（BE ValidStatuses と一致。docs/requirements/application-status-transition.md） */

export const APPLICATION_STATUSES = [
  'not_applied',
  'applied',
  'document_screening',
  'document_passed',
  'interview_scheduled',
  'interview_in_progress',
  'offered',
  'accepted',
  'withdrawn',
  'rejected',
] as const

export type ApplicationStatus = (typeof APPLICATION_STATUSES)[number]

export const STATUS_LABELS: Record<string, string> = {
  not_applied: '未応募',
  applied: '応募済み',
  document_screening: '書類選考中',
  document_passed: '書類通過',
  interview_scheduled: '面接予定',
  interview_in_progress: '面接中',
  offered: '内定',
  accepted: '内定承諾',
  withdrawn: '辞退',
  rejected: '不採用',
  interview: '面接中',
  declined: '辞退',
}

export const STATUS_COLORS: Record<string, 'default' | 'primary' | 'secondary' | 'error' | 'info' | 'success' | 'warning'> = {
  not_applied: 'default',
  applied: 'default',
  document_screening: 'info',
  document_passed: 'info',
  interview_scheduled: 'primary',
  interview_in_progress: 'primary',
  interview: 'primary',
  offered: 'success',
  accepted: 'success',
  withdrawn: 'default',
  declined: 'default',
  rejected: 'error',
}

const USER_TRANSITIONS: Record<string, string[]> = {
  applied: ['withdrawn'],
  document_screening: ['withdrawn'],
  document_passed: ['withdrawn'],
  interview_scheduled: ['withdrawn'],
  interview_in_progress: ['withdrawn'],
  offered: ['accepted', 'withdrawn'],
}

const TERMINAL = new Set(['accepted', 'withdrawn', 'rejected', 'declined'])

/** レガシーコードを現行コードへ寄せる（表示・遷移判定用） */
export function normalizeApplicationStatus(status: string): string {
  if (status === 'interview') return 'interview_in_progress'
  if (status === 'declined') return 'withdrawn'
  return status
}

export function isTerminalStatus(status: string): boolean {
  return TERMINAL.has(normalizeApplicationStatus(status))
}

/** ユーザーが選べる次ステータス（現在値は含めない） */
export function userNextStatuses(current: string): string[] {
  return USER_TRANSITIONS[normalizeApplicationStatus(current)] ?? []
}

export function canUserTransition(current: string, next: string): boolean {
  return userNextStatuses(current).includes(normalizeApplicationStatus(next))
}
