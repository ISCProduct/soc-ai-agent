export type ChatChoiceOption = {
  value: string
  label: string
  text: string
}

function normalizeChoiceKey(s: string): string {
  return s
    .trim()
    .toLowerCase()
    .replace(/[\s　、。・（）()：:．.／/]/g, '')
}

function isOtherChoice(text: string): boolean {
  return text.includes('その他')
}

function choiceTextsMatch(answer: string, option: string): boolean {
  const a = normalizeChoiceKey(answer)
  const o = normalizeChoiceKey(option)
  if (!a || !o) return false
  if (a === o) return true
  if ([...a].length >= 4 && [...o].length >= 4) {
    return a.includes(o) || o.includes(a)
  }
  return false
}

/**
 * 選択肢表示中の自由入力を、可能な限り選択肢記号へ正規化する。
 * 「その他」活性時やラベル不一致の自由記述はそのまま返す。
 */
export function resolveChatOutgoingMessage(
  input: string,
  choices: ChatChoiceOption[],
  otherChoiceActive: boolean,
): string {
  const trimmed = input.trim()
  if (!trimmed) return trimmed
  if (choices.length === 0 || otherChoiceActive) return trimmed

  const upper = trimmed.toUpperCase()
  if (/^[A-E]$/.test(upper)) return upper
  if (/^[1-5]$/.test(trimmed)) return trimmed

  for (const choice of choices) {
    if (isOtherChoice(choice.text)) continue
    if (choiceTextsMatch(trimmed, choice.text)) {
      return choice.value
    }
  }

  return trimmed
}
