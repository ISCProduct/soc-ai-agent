/** 相関図ノードから企業詳細ページへの遷移を組み立てる */

export type CorrelationDiagramNodeLike = {
  id: string
  data?: {
    companyId?: unknown
  }
}

/** 正の整数の企業 ID のみを許可する */
export function parseCompanyId(value: unknown): number | null {
  if (typeof value === 'number') {
    if (!Number.isInteger(value) || value <= 0) return null
    return value
  }
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    if (!Number.isInteger(parsed) || parsed <= 0) return null
    return parsed
  }
  return null
}

/** 企業詳細ページのパスを生成する */
export function buildCompanyDetailPath(companyId: number): string {
  return `/company/${companyId}`
}

/**
 * React Flow ノードから企業詳細パスを得る。
 * data.companyId を優先し、なければ node.id を解釈する。
 * 無効な ID の場合は null（遷移しない）。
 */
export function getCompanyDetailPathFromNode(
  node: CorrelationDiagramNodeLike
): string | null {
  const fromData = parseCompanyId(node.data?.companyId)
  if (fromData !== null) {
    return buildCompanyDetailPath(fromData)
  }
  const fromId = parseCompanyId(node.id)
  if (fromId !== null) {
    return buildCompanyDetailPath(fromId)
  }
  return null
}
