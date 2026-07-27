import type { ChoiceOption } from './types'

export const makeMessageId = () => `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`

export const INITIAL_GREETING =
  'こんにちは！IT業界専門のキャリアエージェントです。\n\nこれから約10-15問の質問を通じて、あなたの適性を分析し、最適な企業をご提案します。\n質問は動的に生成されるため、あなたの回答に応じて変化します。\n\nまず、どのようなIT職種に興味がありますか？\n\n例：\n- Webエンジニア\n- インフラエンジニア\n- データサイエンティスト\n- セキュリティエンジニア\n- モバイルアプリ開発者'

/** リセット時に表示する初回メッセージ（太字強調あり） */
export const RESET_GREETING =
  'こんにちは！IT業界への就職をサポートする適性診断AIです。\n\nこれから約10-15問の質問を通じて、あなたの適性を分析し、最適な企業をご提案します。\n質問は**AIが動的に生成**するため、あなたの回答に応じて変化します。\n\nまず、どのようなIT職種に興味がありますか？\n\n例：\n- Webエンジニア\n- インフラエンジニア\n- データサイエンティスト\n- セキュリティエンジニア\n- モバイルアプリ開発者'

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

/** チャット画面のアクセント（MUI primary 青） */
export const CHAT_BRAND = '#1976d2'
export const CHAT_BRAND_HOVER = '#1565c0'

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
