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
  /**
   * このセクションがAI取得パイプラインで埋められたか。
   *
   * companies テーブルの source_type は行に1つしか無く、技術スタック取得
   * (tech_stack_fetcher) や関連企業取得 (company_relations_fetcher) が
   * 実行されるたびに上書きされる。そのため source_type だけでは
   * 「基本情報は公的DB、技術スタックはAI推定」の混在を区別できない。
   * tech_fetched_at / relations_fetched_at はAI取得側だけが打刻するので、
   * その有無をセクション単位の判定材料として使う。
   */
  section_ai_fetched?: boolean
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

// source_type の表記ゆれと複合値を正規化する。
//
// 実データには "gbizinfo+web_search" のような複合値があり（本番相当DBで34社）、
// 単純な等値比較ではどの分岐にも当たらずバッジが出ないまま
// AI推定の情報が無警告で表示されていた。
function sourceTokens(sourceType: string): string[] {
  return sourceType
    .toLowerCase()
    .split(/[+,/\s]+/)
    .map((t) => t.trim())
    .filter((t) => t.length > 0)
}

/** 公的DB・企業公式サイト由来を示すトークン */
const OFFICIAL_TOKENS = new Set(['official', 'gbizinfo', 'gbiz', 'public_registry'])
/** 運営の手入力 */
const MANUAL_TOKENS = new Set(['manual'])
/**
 * AI・クローラー由来を示すトークン。
 * Backend の companyfetch 定数（scrape / web_search / llm_extract）に加え、
 * 求人サイトやグラフ生成側の表記ゆれも含める。
 */
const AI_TOKENS = new Set([
  'web_search',
  'llm_web_search',
  'scrape',
  'scraping',
  'job_site',
  'llm_extract',
  'db',
])

/**
 * 出どころを分類する。
 *
 * 判定順:
 *   1. セクションがAI取得パイプラインで埋まっている → AI 推定
 *      （行の source_type が公的DBでも、そのセクションはAIが埋めている）
 *   2. AI 由来のトークンが1つでも含まれる → AI 推定
 *      混在（gbizinfo+web_search）は安全側に倒す。一部でもAIなら公式と名乗らせない
 *   3. すべて公的DB・公式サイト由来 → 公式情報
 *   4. 手入力 → 運営入力
 *   5. 判定できない → 出典不明
 *
 * null は返さない。バッジを出さないと「出どころが確かな情報」と同じ見た目になり、
 * この機能の目的（AI推定を公的情報と誤認させない）が達成できないため。
 */
export function classifyProvenance(input: CompanyProvenanceInput | null | undefined): ProvenanceLabel | null {
  if (!input) return null

  const confidence = normalizeConfidence(input.last_fetch_confidence)
  const fetchedAt = formatDate(input.fetched_at)
  const tokens = sourceTokens(input.source_type || '')

  const hasAI = input.section_ai_fetched === true || tokens.some((t) => AI_TOKENS.has(t))
  const hasOfficial = tokens.some((t) => OFFICIAL_TOKENS.has(t)) || Boolean(input.gbiz_last_synced_at)
  const hasManual = tokens.some((t) => MANUAL_TOKENS.has(t))

  if (hasAI) {
    return {
      kind: 'ai',
      label: 'AI推定',
      confidenceLabel: confidence ? CONFIDENCE_LABELS[confidence] : undefined,
      tone: confidence === 'low' ? 'warning' : 'default',
      evidenceUrl: input.source_url || undefined,
      detail: joinDetail([
        hasOfficial
          ? '公的DBの情報にAIがWeb上から補完した内容が混ざっています。正確性は保証されません'
          : 'AIがWeb上の情報から推定した内容です。正確性は保証されません',
        confidence && CONFIDENCE_LABELS[confidence],
        fetchedAt && `取得: ${fetchedAt}`,
        input.last_model_used ? `モデル: ${input.last_model_used}` : undefined,
      ]),
    }
  }

  if (hasOfficial) {
    const syncedAt = formatDate(input.gbiz_last_synced_at)
    return {
      kind: 'official',
      label: '公式情報',
      tone: 'success',
      detail: joinDetail([
        tokens.includes('official') && !tokens.some((t) => t.startsWith('gbiz'))
          ? '出典: 企業の公式サイト'
          : '出典: gBizinfo（経済産業省の法人情報DB）',
        syncedAt ? `同期: ${syncedAt}` : fetchedAt && `取得: ${fetchedAt}`,
      ]),
    }
  }

  if (hasManual) {
    return {
      kind: 'manual',
      label: '運営入力',
      tone: 'default',
      detail: joinDetail(['運営が手入力した情報', fetchedAt && `更新: ${fetchedAt}`]),
    }
  }

  // 確信度だけがあるケース（source_type 未設定）もAI取得の痕跡なのでAI扱い
  if (confidence) {
    return {
      kind: 'ai',
      label: 'AI推定',
      confidenceLabel: CONFIDENCE_LABELS[confidence],
      tone: confidence === 'low' ? 'warning' : 'default',
      evidenceUrl: input.source_url || undefined,
      detail: joinDetail([
        'AIがWeb上の情報から推定した内容です。正確性は保証されません',
        CONFIDENCE_LABELS[confidence],
        fetchedAt && `取得: ${fetchedAt}`,
        input.last_model_used ? `モデル: ${input.last_model_used}` : undefined,
      ]),
    }
  }

  return {
    kind: 'unknown',
    label: '出典不明',
    tone: 'warning',
    detail: joinDetail(['出どころを確認できていない情報です', fetchedAt && `取得: ${fetchedAt}`]),
  }
}
