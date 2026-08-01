export type ProxyResponseData = Record<string, unknown>

export type ParsedProxyResponse = ProxyResponseData | unknown[] | null

export function parseProxyResponse(raw: string, isOk: boolean): ParsedProxyResponse {
  if (!raw) {
    return {}
  }

  try {
    const parsed: unknown = JSON.parse(raw)
    // バックエンドが配列を返す API（relations / market-info 等）はそのまま透過する
    if (Array.isArray(parsed)) {
      return parsed
    }
    if (parsed && typeof parsed === 'object') {
      return parsed as ProxyResponseData
    }
    return { data: parsed }
  } catch {
    return isOk ? { message: raw } : { error: raw }
  }
}

