/** 企業バッチ取得の進捗集計。画面は波ごとの HTTP 結果を足し込むだけ。 */

function num(data: Record<string, unknown>, key: string): number {
  const v = data[key]
  return typeof v === 'number' && Number.isFinite(v) ? v : 0
}

export function batchWaveFromResponse(data: unknown): BatchWave {
  const d = typeof data === 'object' && data !== null && !Array.isArray(data)
    ? (data as Record<string, unknown>)
    : {}
  return {
    processed: num(d, 'processed'),
    info_ok: num(d, 'info_ok'),
    tech_ok: num(d, 'tech_ok'),
    relations_ok: num(d, 'relations_ok'),
    persona_ok: num(d, 'persona_ok'),
    skipped: num(d, 'skipped'),
    errors: num(d, 'errors'),
  }
}

export function candidateCountFromResponse(data: unknown): number {
  const d = typeof data === 'object' && data !== null && !Array.isArray(data)
    ? (data as Record<string, unknown>)
    : {}
  const n = d.candidate_n ?? d.candidate_count
  return typeof n === 'number' && Number.isFinite(n) ? n : 0
}

export type BatchWave = {
  processed: number
  info_ok: number
  tech_ok: number
  relations_ok: number
  persona_ok?: number
  skipped: number
  errors: number
}

export type BatchProgress = {
  processed: number
  infoOk: number
  techOk: number
  relOk: number
  personaOk: number
  skipped: number
  errors: number
  estimated: number
  rounds: number
}

export function emptyBatchProgress(estimated = 0): BatchProgress {
  return {
    processed: 0,
    infoOk: 0,
    techOk: 0,
    relOk: 0,
    personaOk: 0,
    skipped: 0,
    errors: 0,
    estimated: Math.max(0, estimated),
    rounds: 0,
  }
}

export function applyBatchWave(prev: BatchProgress, wave: BatchWave, waveLimit: number): BatchProgress {
  const processed = prev.processed + Math.max(0, wave.processed)
  let estimated = Math.max(prev.estimated, processed)
  // 満杯の波なら続きがある可能性があるのでバーを100%にしない
  if (waveLimit > 0 && wave.processed >= waveLimit) {
    estimated = Math.max(estimated, processed + waveLimit)
  }
  return {
    processed,
    infoOk: prev.infoOk + Math.max(0, wave.info_ok),
    techOk: prev.techOk + Math.max(0, wave.tech_ok),
    relOk: prev.relOk + Math.max(0, wave.relations_ok),
    personaOk: prev.personaOk + Math.max(0, wave.persona_ok ?? 0),
    skipped: prev.skipped + Math.max(0, wave.skipped),
    errors: prev.errors + Math.max(0, wave.errors),
    estimated,
    rounds: prev.rounds + 1,
  }
}

export function batchProgressPercent(progress: BatchProgress): number {
  if (progress.estimated <= 0) return progress.processed > 0 ? 100 : 0
  return Math.min(100, Math.round((progress.processed / progress.estimated) * 100))
}

/** 同じ失敗企業を無限に拾い直さない。満杯かつ新規保存ゼロなら打ち切る。 */
export function shouldContinueBatch(
  wave: BatchWave,
  waveLimit: number,
  rounds: number,
  maxRounds: number,
): boolean {
  if (rounds >= maxRounds) return false
  if (wave.processed <= 0) return false
  if (wave.processed < waveLimit) return false
  const filled = wave.info_ok + wave.tech_ok + wave.relations_ok + (wave.persona_ok ?? 0)
  if (filled <= 0) return false
  return true
}

export function formatBatchProgressLabel(progress: BatchProgress, running: boolean): string {
  const denom =
    progress.estimated > progress.processed
      ? `約 ${progress.estimated} 社`
      : `${progress.processed} 社`
  const head = running
    ? `取得中: ${progress.processed} / ${denom}`
    : `取得完了: ${progress.processed} 社`
  const parts = [
    `会社概要 ${progress.infoOk}`,
    `技術 ${progress.techOk}`,
    `関連企業 ${progress.relOk}`,
    progress.personaOk > 0 ? `マッチング情報 ${progress.personaOk}` : '',
    progress.skipped > 0 ? `スキップ ${progress.skipped}` : '',
    progress.errors > 0 ? `失敗 ${progress.errors}` : '',
  ].filter(Boolean)
  return parts.length > 0 ? `${head}（${parts.join(' / ')}）` : head
}

export type BatchItemFailure = { name: string; error: string; companyId?: number }

/** Backend の items[].error を拾う。画面が件数だけ見て原因を捨てないため。 */
export function batchItemFailuresFromResponse(data: unknown): BatchItemFailure[] {
  const d = typeof data === 'object' && data !== null && !Array.isArray(data)
    ? (data as Record<string, unknown>)
    : {}
  if (!Array.isArray(d.items)) return []
  const out: BatchItemFailure[] = []
  for (const raw of d.items) {
    if (typeof raw !== 'object' || raw === null) continue
    const item = raw as Record<string, unknown>
    const error = typeof item.error === 'string' ? item.error.trim() : ''
    if (!error) continue
    out.push({
      name: typeof item.name === 'string' ? item.name : '',
      error,
      companyId: typeof item.company_id === 'number' ? item.company_id : undefined,
    })
  }
  return out
}

export function explainBatchItemError(raw: string): string {
  const s = raw.toLowerCase()
  if (s.includes('budget')) return '月次の情報取得上限に達しています'
  if (s.includes('openai client is nil')) return 'OpenAI APIキーが未設定です'
  if (s.includes('does not exist') || s.includes('model_not_found')) {
    return '検索用AIモデルが廃止または未対応です'
  }
  if (s.includes('429') || s.includes('rate limit') || s.includes('rate_limit')) {
    return 'OpenAI のレート制限です'
  }
  if (s.includes('web search failed') || s.includes('web search returned empty')) {
    return 'Web検索で企業情報を取得できませんでした'
  }
  if (s.includes('timeout') || s.includes('deadline exceeded')) return '取得がタイムアウトしました'
  if (s.includes('gbiz')) return 'gBizINFO からの取得に失敗しました'
  return raw
}

export function formatBatchFailureDetail(failures: BatchItemFailure[]): string {
  if (failures.length === 0) return ''
  const explained = failures.map((f) => ({
    name: f.name,
    reason: explainBatchItemError(f.error),
  }))
  const uniq = [...new Set(explained.map((e) => e.reason))]
  if (uniq.length === 1) {
    const hint = uniq[0].includes('上限')
      ? 'コスト画面を確認してください。'
      : '時間をおいて再度お試しください。'
    return `${failures.length} 社とも ${uniq[0]}。${hint}`
  }
  return explained
    .slice(0, 6)
    .map((e) => `${e.name || '不明'}: ${e.reason}`)
    .join(' / ')
}

/** Next/Backend のログを同じキーで grep するための1行。dry_run は出さない。 */
export function formatMissingBatchLogLine(data: unknown): string | null {
  const d = typeof data === 'object' && data !== null && !Array.isArray(data)
    ? (data as Record<string, unknown>)
    : {}
  if (d.dry_run === true) return null
  if (typeof d.error === 'string' && d.error && d.processed == null) {
    return `[fetch_missing_batch] stop_reason=http_error error=${d.error}`
  }
  if (typeof d.processed !== 'number' && !Array.isArray(d.items)) return null
  const failures = batchItemFailuresFromResponse(data)
  const parts = failures.map((f) => {
    const id = f.companyId != null ? `id=${f.companyId} ` : ''
    return `${id}${f.name || '不明'}: ${f.error}`
  })
  const stop = typeof d.stop_reason === 'string' && d.stop_reason ? d.stop_reason : 'unknown'
  return (
    `[fetch_missing_batch] stop_reason=${stop}` +
    ` processed=${num(d, 'processed')}` +
    ` info_ok=${num(d, 'info_ok')}` +
    ` tech_ok=${num(d, 'tech_ok')}` +
    ` relations_ok=${num(d, 'relations_ok')}` +
    ` errors=${num(d, 'errors')}` +
    ` failures=${parts.join(' | ')}`
  )
}
