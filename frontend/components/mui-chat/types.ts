import type { ChatChoiceOption } from '@/lib/chat-choices'
import type { PhaseProgress } from '@/lib/api'

export type { PhaseProgress }

/** チャットメッセージ1件 */
export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: Date
}

/** 選択肢チップ（chat-choices と同形） */
export type ChoiceOption = ChatChoiceOption

/** フェーズ進捗の集計表示用 */
export interface ProgressTotals {
  valid: number
  required: number
  percent: number
}
