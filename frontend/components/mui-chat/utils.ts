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
