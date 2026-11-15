import type { Utterance } from '../../types'

export interface TranscriptView {
  /** 画面に出す現在の質問文（読み上げ中の途中経過を優先） */
  currentQuestionText: string
  /** 現在の質問に対する回答（認識中の途中経過を優先） */
  liveAnswerText: string
}

/**
 * 「現在の質問」「ライブ回答」を既存の発話ストリームから導出する。新規のデータは持たない。
 * 確定済みの発話が無い間は直近のAI/ユーザー発話にフォールバックする。
 */
export function deriveTranscriptView(
  utterances: readonly Utterance[],
  partialAi: string,
  partialUser: string,
): TranscriptView {
  const lastAiIndex = utterances.map((u) => u.role).lastIndexOf('ai')
  const latestAi = lastAiIndex >= 0 ? utterances[lastAiIndex].text : ''
  // 現在の質問(最新のAI発話)より後に来たuser発話だけを「現在の質問への回答」として扱う。
  // 末尾のuser発話をそのまま使うと、次の質問が確定した直後(まだ何も話していない間)に
  // 前の質問への回答が「現在の質問の回答」として出続けてしまう(CodeRabbit指摘)。
  // AI発話が一度も無い(lastAiIndex=-1)間は、質問自体が無いのでuser発話も拾わない
  // (テスター指摘。通常の面接フローではAIが必ず先に質問するため起きないが念のため)。
  const latestUser =
    lastAiIndex >= 0
      ? (utterances.slice(lastAiIndex + 1).findLast((u) => u.role === 'user')?.text ?? '')
      : ''
  return {
    currentQuestionText: partialAi || latestAi,
    liveAnswerText: partialUser || latestUser,
  }
}

export type Urgency = 'safe' | 'warning' | 'critical'

/** この秒数以下で「注意」「危険」にする（質問タイマー） */
const CRITICAL_SECONDS = 30
const WARNING_SECONDS = 60

export function urgencyOf(remainingSeconds: number): Urgency {
  if (remainingSeconds <= CRITICAL_SECONDS) return 'critical'
  if (remainingSeconds <= WARNING_SECONDS) return 'warning'
  return 'safe'
}

export type ProgressDotState = 'done' | 'current' | 'todo'

/** 質問ごとの進行セグメント。currentQuestionIndex は 1 始まり。 */
export function progressDots(currentIndex: number, total: number): ProgressDotState[] {
  return Array.from({ length: Math.max(0, total) }, (_, i) => {
    const n = i + 1
    return n < currentIndex ? 'done' : n === currentIndex ? 'current' : 'todo'
  })
}

/** 残り時間の警告バナーの文言。表示しないときは null。 */
export function sessionWarningText(shown: boolean, remainingSeconds: number): string | null {
  if (!shown) return null
  if (remainingSeconds <= 0) return 'セッションが終了しました。'
  return `残り約${Math.ceil(remainingSeconds / 60)}分です。`
}
