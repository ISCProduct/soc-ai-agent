/**
 * Shared utility functions for the interview feature.
 * Pure functions with no side-effects — safe to import anywhere.
 */

/** Formats elapsed seconds as "m:ss" (e.g. 125 → "2:05") */
export function formatSeconds(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = String(totalSeconds % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
}

/** Safely parses a JSON string; returns null on any error instead of throwing */
export function parseJsonSafe(value?: string): unknown {
  try {
    return value ? JSON.parse(value) : null
  } catch {
    return null
  }
}

/**
 * Extracts the human-readable message from a backend error response body.
 * Backend errors are JSON `{ error, code, detail? }` (see error_handler.go)
 * with `error` already sanitized to a Japanese message server-side.
 * Falls back to the raw body text if it isn't in that shape, so callers
 * never lose information — they just avoid showing raw JSON to the user
 * when the expected shape is present (#910).
 */
export function extractApiErrorMessage(bodyText: string): string {
  const parsed = parseJsonSafe(bodyText)
  if (parsed && typeof parsed === 'object' && typeof (parsed as { error?: unknown }).error === 'string') {
    return (parsed as { error: string }).error
  }
  return bodyText
}

/**
 * Converts a media-device / API error into a user-friendly Japanese message.
 * Keeps error-message logic out of UI components.
 */
export function parseMediaError(error: unknown): string {
  const msg: string = (error as { message?: string })?.message || ''
  const lower = msg.toLowerCase()
  if (msg.includes('NotAllowedError') || lower.includes('denied'))
    return 'マイクとカメラへのアクセスが拒否されました。ブラウザのアドレスバー横から権限を許可してください。'
  if (msg.includes('NotFoundError'))
    return 'マイクまたはカメラが見つかりません。デバイスが正しく接続されているか確認してください。'
  // OpenAI 側の 401（chat/tts/whisper error 401 など）のみ API キー案内にする。
  // セッション JWT の Unauthorized を誤って API キー不足と表示しない。
  if (
    /(?:chat|tts|whisper|openai).*error\s*401/i.test(msg) ||
    (lower.includes('openai') && (lower.includes('unauthorized') || msg.includes('401')))
  ) {
    return 'AIサービスへの接続に失敗しました。（OpenAI APIキーを確認してください）'
  }
  if (lower.includes('unauthorized') || /\b401\b/.test(msg))
    return 'ログインの有効期限が切れました。再ログインしてから面接を開始してください。'
  return msg || '接続に失敗しました。ネットワークを確認して再試行してください。'
}

/**
 * Parses a multipart/mixed response that contains a JSON metadata part
 * followed by an audio binary part.
 *
 * Why custom parsing instead of a library: the Fetch API does not expose
 * multipart body parsing, and we need to handle the binary audio part
 * without converting it to a string.
 */
export async function parseMultipartResponse(
  res: Response
): Promise<{ meta: Record<string, string>; audio: Blob }> {
  const contentType = res.headers.get('content-type') || ''
  const boundaryMatch = contentType.match(/boundary=([^\s;]+)/)
  if (!boundaryMatch) throw new Error('No boundary in multipart response')

  const boundary = '--' + boundaryMatch[1]
  const buf = await res.arrayBuffer()
  const bytes = new Uint8Array(buf)
  const enc = new TextEncoder()

  /** Returns the index of the first occurrence of `needle` at or after `from`, or -1 */
  const findPattern = (needle: Uint8Array, from: number): number => {
    outer: for (let i = from; i <= bytes.length - needle.length; i++) {
      for (let j = 0; j < needle.length; j++) {
        if (bytes[i + j] !== needle[j]) continue outer
      }
      return i
    }
    return -1
  }

  const boundaryBytes = enc.encode(boundary)
  const headerSeparator = enc.encode('\r\n\r\n')

  // Part 1: JSON metadata
  const part1Start = findPattern(boundaryBytes, 0)
  const part1HeaderEnd = findPattern(headerSeparator, part1Start + boundaryBytes.length)
  const part2Start = findPattern(boundaryBytes, part1HeaderEnd + 4)
  const jsonBytes = bytes.slice(part1HeaderEnd + 4, part2Start - 2)
  const meta = JSON.parse(new TextDecoder().decode(jsonBytes).trim())

  // Part 2: Audio binary
  const part2HeaderEnd = findPattern(headerSeparator, part2Start + boundaryBytes.length)
  const closingBoundary = enc.encode(boundary + '--')
  const closingBoundaryPos = findPattern(closingBoundary, part2HeaderEnd + 4)
  const audioEnd = closingBoundaryPos !== -1 ? closingBoundaryPos - 2 : bytes.length
  const audio = new Blob([bytes.slice(part2HeaderEnd + 4, audioEnd)], { type: 'audio/mpeg' })

  return { meta, audio }
}
