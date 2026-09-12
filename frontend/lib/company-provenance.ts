/**
 * 企業情報の出どころ（provenance）を学生向けの表示ラベルへ変換する（#1125 フェーズ1）。
 *
 * 企業情報は gBizinfo（公的DB）由来と AI 補完が混在しているが、学生向けの企業ページでは
 * 両者が区別できず、AI 推定の情報が公的情報と同じ見た目で表示されていた。
 * 「どこから来た情報か」と「確信度」を出し、AI 推定には根拠リンクを添える。
 */

export type ProvenanceKind = 'official' | 'ai' | 'manual' | 'unknown'

export type ConfidenceLevel = 'high' | 'medium' | 'low'

/** API（/api/companies/:id）が返す出どころ関連フィールド */
export interface CompanyProvenanceInput {
  source_type?: string | null
  source_url?: string | null
  last_fetch_confidence?: string | null
  last_model_used?: string | null
  gbiz_last_synced_at?: string | null
  /** セクション別の取得時刻。無ければ source_fetched_at で代替する */
  fetched_at?: string | null
}

export interface ProvenanceLabel {
  kind: ProvenanceKind
  /** チップに出す文言 */
  label: string
  /** AI 推定のときの確信度表示（高い/中程度/低い） */
  confidenceLabel?: string
  /** MUI Chip の color に渡す意味づけ */
  tone: 'success' | 'warning' | 'default'
  /** 根拠として開けるURL（AI 推定のときだけ） */
  evidenceUrl?: string
  /** ツールチップに出す補足（取得時刻・モデル名など） */
  detail: string
}

const CONFIDENCE_LABELS: Record<ConfidenceLevel, string> = {
  high: '確信度: 高い',
  medium: '確信度: 中程度',
  low: '確信度: 低い',
}

function normalizeConfidence(value?: string | null): ConfidenceLevel | undefined {
  if (value === 'high' || value === 'medium' || value === 'low') return value
  return undefined
}

function formatDate(value?: string | null): string {
  if (!value) return ''
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}/${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')}`
}

function joinDetail(parts: Array<string | undefined>): string {
  return parts.filter((p) => p && p.length > 0).join(' / ')
}

/**
 * 出どころを分類する。表示するものが無い場合は null を返す（バッジを出さない）。
 *
 * 判定順:
 *   1. gBizinfo と同期済み → 公的DB由来（公式情報）
 *   2. source_type が official / manual → 公式サイト / 手入力
 *   3. source_type が web_search、または確信度がある → AI 推定
 *   4. それ以外 → 表示しない
 */
export function classifyProvenance(input: CompanyProvenanceInput | null | undefined): ProvenanceLabel | null {
  if (!input) return null

  const confidence = normalizeConfidence(input.last_fetch_confidence)
  const fetchedAt = formatDate(input.fetched_at)
  const sourceType = (input.source_type || '').trim()

  if (input.gbiz_last_synced_at) {
    return {
      kind: 'official',
      label: '公式情報',
      tone: 'success',
      detail: joinDetail(['出典: gBizinfo（経済産業省の法人情報DB）', `同期: ${formatDate(input.gbiz_last_synced_at)}`]),
    }
  }

  if (sourceType === 'official') {
    return {
      kind: 'official',
      label: '公式情報',
      tone: 'success',
      detail: joinDetail(['出典: 企業の公式サイト', fetchedAt && `取得: ${fetchedAt}`]),
    }
  }

  if (sourceType === 'manual') {
    return {
      kind: 'manual',
      label: '運営入力',
      tone: 'default',
      detail: joinDetail(['運営が手入力した情報', fetchedAt && `更新: ${fetchedAt}`]),
    }
  }

  if (sourceType === 'web_search' || sourceType === 'scrape' || sourceType === 'job_site' || confidence) {
    return {
      kind: 'ai',
      label: 'AI推定',
      confidenceLabel: confidence ? CONFIDENCE_LABELS[confidence] : undefined,
      tone: confidence === 'low' ? 'warning' : 'default',
      evidenceUrl: input.source_url || undefined,
      detail: joinDetail([
        'AIがWeb上の情報から推定した内容です。正確性は保証されません',
        confidence && CONFIDENCE_LABELS[confidence],
        fetchedAt && `取得: ${fetchedAt}`,
        input.last_model_used ? `モデル: ${input.last_model_used}` : undefined,
      ]),
    }
  }

  return null
}
