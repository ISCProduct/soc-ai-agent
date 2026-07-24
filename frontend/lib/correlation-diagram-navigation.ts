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
 * React Flow ノードから企業 ID を得る。
 * data.companyId を優先し、なければ node.id を解釈する。
 */
export function getCompanyIdFromNode(
  node: CorrelationDiagramNodeLike
): number | null {
  const fromData = parseCompanyId(node.data?.companyId)
  if (fromData !== null) return fromData
  return parseCompanyId(node.id)
}

/**
 * React Flow ノードから企業詳細パスを得る。
 * 無効な ID の場合は null。
 */
export function getCompanyDetailPathFromNode(
  node: CorrelationDiagramNodeLike
): string | null {
  const companyId = getCompanyIdFromNode(node)
  if (companyId === null) return null
  return buildCompanyDetailPath(companyId)
}
