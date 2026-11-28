/** 診断品質レポート。バックエンドのモデルは json タグが無く、フィールド名のまま返る。 */
export interface DiagnosisQualityReport {
  ID: number
  UserID: number
  SessionID: string
  Confidence: number
  FlagsJSON: string
  Summary: string
  UpdatedAt: string
}

/** これ未満、またはフラグがある診断を「確認したい」に上げる。 */
export const LOW_CONFIDENCE = 60

export function parseFlags(raw: string): string[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is string => typeof item === 'string' && item !== '')
  } catch {
    return []
  }
}

export function needsReview(report: DiagnosisQualityReport): boolean {
  return report.Confidence < LOW_CONFIDENCE || parseFlags(report.FlagsJSON).length > 0
}

export function confidenceBand(confidence: number): '低い' | 'ふつう' | '高い' {
  if (confidence < LOW_CONFIDENCE) return '低い'
  if (confidence < 80) return 'ふつう'
  return '高い'
}
