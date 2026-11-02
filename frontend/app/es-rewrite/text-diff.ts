export type DiffKind = 'unchanged' | 'added' | 'removed'

export type DiffSegment = {
  text: string
  kind: DiffKind
  start: number
  end: number
}

export type TextDiff = {
  original: DiffSegment[]
  improved: DiffSegment[]
}

type Token = {
  text: string
  start: number
  end: number
}

const TOKEN_PATTERN =
  /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}]|[A-Za-z0-9]+|\s+|[^\s]/gu
const MAX_DIFF_CELLS = 4_000_000

function tokenize(text: string): Token[] {
  if (typeof Intl.Segmenter === 'function') {
    const segmenter = new Intl.Segmenter('ja', { granularity: 'word' })
    return Array.from(segmenter.segment(text), part => ({
      text: part.segment,
      start: part.index,
      end: part.index + part.segment.length,
    }))
  }

  const tokens: Token[] = []
  const pattern = new RegExp(TOKEN_PATTERN.source, 'gu')
  let match: RegExpExecArray | null
  while ((match = pattern.exec(text)) !== null) {
    tokens.push({ text: match[0], start: match.index, end: match.index + match[0].length })
  }
  return tokens
}

function appendSegment(segments: DiffSegment[], token: Token, kind: DiffKind): void {
  const previous = segments[segments.length - 1]
  if (previous && previous.kind === kind && previous.end === token.start) {
    previous.text += token.text
    previous.end = token.end
    return
  }
  segments.push({ text: token.text, kind, start: token.start, end: token.end })
}

export function createTextDiff(originalText: string, improvedText: string): TextDiff {
  const original = tokenize(originalText)
  const improved = tokenize(improvedText)
  const originalSegments: DiffSegment[] = []
  const improvedSegments: DiffSegment[] = []

  let prefix = 0
  while (
    prefix < original.length &&
    prefix < improved.length &&
    original[prefix].text === improved[prefix].text
  ) {
    appendSegment(originalSegments, original[prefix], 'unchanged')
    appendSegment(improvedSegments, improved[prefix], 'unchanged')
    prefix += 1
  }

  let suffix = 0
  while (
    suffix < original.length - prefix &&
    suffix < improved.length - prefix &&
    original[original.length - suffix - 1].text === improved[improved.length - suffix - 1].text
  ) {
    suffix += 1
  }

  const originalEnd = original.length - suffix
  const improvedEnd = improved.length - suffix
  const originalMiddle = original.slice(prefix, originalEnd)
  const improvedMiddle = improved.slice(prefix, improvedEnd)
  const rows = originalMiddle.length + 1
  const columns = improvedMiddle.length + 1

  if (rows * columns > MAX_DIFF_CELLS) {
    for (const token of originalMiddle) appendSegment(originalSegments, token, 'removed')
    for (const token of improvedMiddle) appendSegment(improvedSegments, token, 'added')
  } else {
    const lcs = new Uint16Array(rows * columns)
    for (let row = rows - 2; row >= 0; row -= 1) {
      for (let column = columns - 2; column >= 0; column -= 1) {
        const index = row * columns + column
        lcs[index] =
          originalMiddle[row].text === improvedMiddle[column].text
            ? lcs[(row + 1) * columns + column + 1] + 1
            : Math.max(lcs[(row + 1) * columns + column], lcs[index + 1])
      }
    }

    let row = 0
    let column = 0
    while (row < originalMiddle.length && column < improvedMiddle.length) {
      if (originalMiddle[row].text === improvedMiddle[column].text) {
        appendSegment(originalSegments, originalMiddle[row], 'unchanged')
        appendSegment(improvedSegments, improvedMiddle[column], 'unchanged')
        row += 1
        column += 1
      } else if (
        lcs[(row + 1) * columns + column] >= lcs[row * columns + column + 1]
      ) {
        appendSegment(originalSegments, originalMiddle[row], 'removed')
        row += 1
      } else {
        appendSegment(improvedSegments, improvedMiddle[column], 'added')
        column += 1
      }
    }
    while (row < originalMiddle.length) {
      appendSegment(originalSegments, originalMiddle[row], 'removed')
      row += 1
    }
    while (column < improvedMiddle.length) {
      appendSegment(improvedSegments, improvedMiddle[column], 'added')
      column += 1
    }
  }

  for (let offset = suffix; offset > 0; offset -= 1) {
    const originalToken = original[original.length - offset]
    const improvedToken = improved[improved.length - offset]
    appendSegment(originalSegments, originalToken, 'unchanged')
    appendSegment(improvedSegments, improvedToken, 'unchanged')
  }

  return { original: originalSegments, improved: improvedSegments }
}
