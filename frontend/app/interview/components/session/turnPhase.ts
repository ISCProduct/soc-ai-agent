/**
 * 面接の「今だれの番か」と、それに応じたボタン・字幕の文言を決める。
 * if の連鎖を避け、状態 → 表示 の対応を PHASE_VIEW の表で持つ。
 */
export type TurnPhase = 'listening' | 'thinking' | 'recording' | 'yourTurn'

export type PhaseIcon = 'speaker' | 'mic' | 'stop' | 'spinner'

export interface PhaseView {
  talkLabel: string
  talkIcon: PhaseIcon
  /** quiet: 押せない待機 / primary: 次の操作 / recording: 録音中 */
  talkTone: 'quiet' | 'primary' | 'recording'
  /** 質問表示の1行目（いまの状況） */
  statusText: string
}

// 「AI発話中」「AIが考えています」とは呼ばない。面接練習は「面接官と話している」という
// 前提が成り立ってこそ練習になり、話し相手をAIと呼ぶとその前提が崩れる。
export const PHASE_VIEW: Record<TurnPhase, PhaseView> = {
  listening: { talkLabel: '質問を聞いています', talkIcon: 'speaker', talkTone: 'quiet', statusText: '面接官が質問しています' },
  thinking: { talkLabel: '考えています', talkIcon: 'spinner', talkTone: 'quiet', statusText: '面接官が考えています…' },
  recording: { talkLabel: '話し終わった', talkIcon: 'stop', talkTone: 'recording', statusText: '回答中' },
  yourTurn: { talkLabel: '話す', talkIcon: 'mic', talkTone: 'primary', statusText: 'あなたの番です。準備ができたら「話す」を押してください' },
}

interface TurnFlags {
  isRecording: boolean
  turnPending: boolean
  aiSpeaking: boolean
}

/** 録音中は最優先（止める手段を隠さない）。次に送信中、面接官の発話中、それ以外はあなたの番。 */
export function turnPhaseOf({ isRecording, turnPending, aiSpeaking }: TurnFlags): TurnPhase {
  if (isRecording) return 'recording'
  if (turnPending) return 'thinking'
  if (aiSpeaking) return 'listening'
  return 'yourTurn'
}

/** 未接続・送信中・面接官の発話中は押せない（面接官の発話に被せて録音しない）。 */
export function talkDisabled(flags: TurnFlags & { isConnected: boolean }): boolean {
  return !flags.isConnected || flags.turnPending || flags.aiSpeaking
}
