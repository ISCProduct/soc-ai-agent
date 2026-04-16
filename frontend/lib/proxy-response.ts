export type ProxyResponseData = Record<string, unknown>

export function parseProxyResponse(raw: string, isOk: boolean): ProxyResponseData {
  if (!raw) {
    return {}
  }

  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as ProxyResponseData
    }
    return { data: parsed }
  } catch {
    return isOk ? { message: raw } : { error: raw }
  }
}

