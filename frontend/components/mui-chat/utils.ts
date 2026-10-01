import type { ChoiceOption } from './types'

export const makeMessageId = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

/**
 * 最初に出すメッセージ。職種の例は列挙しない。
 * この直後にクイック選択のチップ(JOB_QUICK_OPTIONS)が並ぶため、
 * 文中に別の職種リストを置くと選べない候補を見せることになる。
 */
export const INITIAL_GREETING =
  'こんにちは！IT業界専門のキャリアエージェントです。\n\n10〜15問の質問に答えていただくと、あなたに合いそうな企業をご提案します。\n答えた内容に合わせて次の質問が変わるので、思ったとおりに答えてください。\n\nまず、どんな仕事に興味がありますか？下のボタンから選ぶか、気になる仕事を入力してください。'

/**
 * アシスタント応答から A)/1) 形式の選択肢を抽出する。
 * 空行はスキップ。記号正規化は lib/chat-choices 側で行う。
 */
export function extractChoices(content: string): ChoiceOption[] {
  const lines = content.split('\n')
  const choices: ChoiceOption[] = []
  for (const line of lines) {
    const trimmedLine = line.trim()
    if (!trimmedLine) {
      continue
    }
    let match = trimmedLine.match(/^([A-E])\)\s*(.+)$/)
    if (!match) {
      match = trimmedLine.match(/^([A-E])[：、.．]\s*(.+)$/)
    }
    if (match) {
      choices.push({ value: match[1], label: match[1], text: match[2].trim() })
      continue
    }
    match = trimmedLine.match(/^(\d+)[\.\)．]\s*(.+)$/)
    if (match) {
      choices.push({ value: match[1], label: match[1], text: match[2].trim() })
    }
  }
  return choices
}

export const JOB_QUICK_OPTIONS = [
  '開発系エンジニア',
  'インフラエンジニア',
  '両方に興味がある',
  'まだ決めていない',
] as const

/** チャット画面のアクセント（サイドバーと同じブランドオレンジ） */
/**
 * ブランドの橙（ロゴと同じ #ec5b13）。
 *
 * 文字を載せない装飾にだけ使う。白地に対して 3.46:1 しかなく、
 * 文字色や「白文字を載せる塗り」にすると WCAG AA(4.5:1)を満たさない。
 * 非文字UI（進捗バーの塗りなど）は 3:1 でよいのでここだけに留める。
 */
export const CHAT_BRAND = '#ec5b13'

/**
 * 文字・塗り・罫線に使う色。学生テーマの primary（Wong の色覚セーフ青）。
 *
 * 以前は上の橙を吹き出しの塗りと「終了」ボタンの文字色に使っており、
 * どちらもコントラスト不足だった。白文字を載せて 5.19:1。
 */
export const CHAT_ACCENT = '#0072B2'
export const CHAT_ACCENT_HOVER = '#005B8E'

export const CHAT_WARN_EDGE = '#E69F00'
export const CHAT_WARN_TEXT = '#946200'
export const CHAT_STOP_EDGE = '#D55E00'
export const CHAT_STOP_TEXT = '#99370A'

/**
 * 選択肢行（A) / 1. など）を本文から除き、バブルとボタンの二重表示を防ぐ。
 */
export function stripChoiceLines(content: string): string {
  const kept = content.split('\n').filter((line) => {
    const trimmed = line.trim()
    if (!trimmed) return true
    if (/^([A-E])\)\s*.+$/.test(trimmed)) return false
    if (/^([A-E])[：、.．]\s*.+$/.test(trimmed)) return false
    if (/^(\d+)[\.\)．]\s*.+$/.test(trimmed)) return false
    return true
  })
  return kept.join('\n').replace(/\n{3,}/g, '\n\n').trim()
}

/**
 * ヘッダー進捗をサイドバーと同じ「想定総質問数」ベースで計算する。
 * asked ベースだと途中で 100% に見える問題を避ける。
 */
export function computeProgressTotals(args: {
  phases: { questions_asked?: number; valid_answers?: number; min_questions?: number; max_questions?: number }[] | null
  questionCount: number
  totalQuestions: number
}): { valid: number; required: number; percent: number } {
  const totalFallback = Math.max(1, args.totalQuestions || 15)
  if (args.phases && args.phases.length > 0) {
    let valid = 0
    let required = 0
    for (const phase of args.phases) {
      valid += phase.valid_answers || 0
      const need =
        (phase.max_questions && phase.max_questions > 0
          ? phase.max_questions
          : phase.min_questions) || 0
      required += need
    }
    if (required <= 0) required = totalFallback
    return {
      valid,
      required,
      percent: Math.min(100, Math.round((valid / required) * 100)),
    }
  }
  return {
    valid: args.questionCount,
    required: totalFallback,
    percent: Math.min(100, Math.round((args.questionCount / totalFallback) * 100)),
  }
}

/**
 * メッセージ一覧の自動スクロールを「下部付近にいるときだけ」許可する。
 */
export function shouldAutoScrollToBottom(args: {
  scrollHeight: number
  scrollTop: number
  clientHeight: number
  thresholdPx?: number
}): boolean {
  const threshold = args.thresholdPx ?? 120
  const distanceFromBottom = args.scrollHeight - args.scrollTop - args.clientHeight
  return distanceFromBottom <= threshold
}

/**
 * チャット終了時にセッション ID とメッセージ／キャッシュを削除する。
 * sessionId は remove 前に取得する（先に消すと chat_cache_ が残る）。
 */
export function clearChatSessionOnEnd(storage: {
  sessionStorage: Pick<Storage, 'getItem' | 'removeItem'>
  localStorage: Pick<Storage, 'removeItem'>
}): void {
  const currentSessionId = storage.sessionStorage.getItem('chatSessionId')
  if (currentSessionId) {
    storage.localStorage.removeItem(`chat_cache_${currentSessionId}`)
    storage.sessionStorage.removeItem(jobCategoryStorageKey(currentSessionId))
  }
  storage.sessionStorage.removeItem('chatSessionId')
  storage.sessionStorage.removeItem('chatMessages')
  storage.localStorage.removeItem('chatMessages')
  storage.localStorage.removeItem('chat_session_id')
}

export function jobCategoryStorageKey(sessionId: string): string {
  return `chat_job_category_id_${sessionId}`
}

export function readStoredJobCategoryId(
  sessionId: string,
  storage: Pick<Storage, 'getItem'> = sessionStorage,
): number {
  if (!sessionId) return 0
  const raw = storage.getItem(jobCategoryStorageKey(sessionId))
  const n = raw ? Number(raw) : 0
  return Number.isFinite(n) && n > 0 ? n : 0
}

export function writeStoredJobCategoryId(
  sessionId: string,
  jobCategoryId: number,
  storage: Pick<Storage, 'setItem' | 'removeItem'> = sessionStorage,
): void {
  if (!sessionId) return
  const key = jobCategoryStorageKey(sessionId)
  if (jobCategoryId > 0) {
    storage.setItem(key, String(jobCategoryId))
  } else {
    storage.removeItem(key)
  }
}

/** Ctrl+Enter / ⌘+Enter で送信。Enter 単独は改行。 */
export function shouldSendChatOnKeyDown(e: {
  key: string
  ctrlKey: boolean
  metaKey: boolean
  nativeEvent?: { isComposing?: boolean }
  isComposing?: boolean
}): boolean {
  if (e.key !== 'Enter') return false
  const composing = e.isComposing || e.nativeEvent?.isComposing
  if (composing) return false
  return e.ctrlKey || e.metaKey
}

/**
 * 無効回答の案内・打ち切りメッセージを見分ける目印。
 *
 * Backend/internal/services/chat/chat_answer_validator.go の
 * validationMarkers と同じ文字列。片方だけ変えると、画面側が案内文を
 * 「直近の質問」として拾い、選択肢の復元が壊れる。
 *
 * 旧文言も残す。既存セッションの DB には旧文言のまま保存されている。
 */
export const VALIDATION_FEEDBACK_MARKERS = [
  '質問に沿った内容でもう一度お願いします',
  'このチャットを終了しました',
  // 旧文言（2026-10 以前に保存されたもの）
  '書かれた内容にはお答えできません',
  '質問と関係のない内容が3回続いた',
] as const

/** 打ち切りの目印だけを見る（案内と打ち切りで表示を変えるため） */
export const VALIDATION_TERMINATION_MARKERS = [
  'このチャットを終了しました',
  // 旧文言
  '質問と関係のない内容が3回続いた',
  'チャットを終了させていただきます',
] as const

/** 打ち切りメッセージか */
export function isValidationTerminationMessage(content: string): boolean {
  const trimmed = content.trim()
  if (!trimmed) return false
  return VALIDATION_TERMINATION_MARKERS.some((marker) => trimmed.includes(marker))
}

/** 無効回答の案内・打ち切りメッセージか（選択肢抽出の対象外） */
export function isValidationFeedbackMessage(content: string): boolean {
  const trimmed = content.trim()
  if (!trimmed) return false
  return VALIDATION_FEEDBACK_MARKERS.some((marker) => trimmed.includes(marker))
}

/** 警告を飛ばして直近のアシスタント質問メッセージを返す */
export function findLastAssistantQuestionMessage<T extends { role: string; content: string }>(
  messages: T[],
): T | undefined {
  for (let i = messages.length - 1; i >= 0; i--) {
    const msg = messages[i]
    if (msg.role !== 'assistant') continue
    if (isValidationFeedbackMessage(msg.content)) continue
    return msg
  }
  return undefined
}



/**
 * メッセージ1件の読み上げ名。
 *
 * 支援技術には発言者と時刻の手がかりが一切無かった。
 * 左右の位置と色でしか区別しておらず、アイコンも代替テキストを持っていなかったため、
 * アイコンがあった頃から誰の発言かは伝わっていない。
 *
 * 画面には出さず名前としてだけ渡す。15問の短いやり取りに時刻を並べると
 * 本文より目立ってしまい、読む順番を乱す。
 */
export function messageAccessibleLabel(role: 'user' | 'assistant', at: Date): string {
  const who = role === 'user' ? 'あなた' : 'エージェント'
  if (Number.isNaN(at.getTime())) return who
  const time = `${at.getHours()}時${String(at.getMinutes()).padStart(2, '0')}分`
  return `${who}、${time}`
}
