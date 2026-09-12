import { classifyProvenance } from '@/lib/company-provenance'

// 企業情報の出どころ表示（#1125 フェーズ1）。
// AI 推定を公的情報と同じ見た目で出さないことが目的なので、分類の境界を固定する。
describe('classifyProvenance', () => {
  it('gBizinfo と同期済みなら公式情報', () => {
    const got = classifyProvenance({
      source_type: 'web_search',
      gbiz_last_synced_at: '2026-09-01T00:00:00Z',
      last_fetch_confidence: 'low',
    })
    // AI 由来の source_type や低い確信度が残っていても、公的DBと同期済みなら公式扱い
    expect(got?.kind).toBe('official')
    expect(got?.label).toBe('公式情報')
    expect(got?.detail).toContain('gBizinfo')
  })

  it('公式サイト由来は公式情報', () => {
    const got = classifyProvenance({ source_type: 'official', source_fetched_at: null } as never)
    expect(got?.kind).toBe('official')
  })

  it('運営の手入力は運営入力', () => {
    expect(classifyProvenance({ source_type: 'manual' })?.kind).toBe('manual')
  })

  it.each([
    ['web_search', 'AI推定'],
    ['scrape', 'AI推定'],
    ['job_site', 'AI推定'],
  ])('source_type=%s は %s', (sourceType, label) => {
    const got = classifyProvenance({ source_type: sourceType })
    expect(got?.kind).toBe('ai')
    expect(got?.label).toBe(label)
  })

  it('確信度だけでも AI 推定として扱う（source_type が空のケース）', () => {
    const got = classifyProvenance({ last_fetch_confidence: 'medium' })
    expect(got?.kind).toBe('ai')
    expect(got?.confidenceLabel).toBe('確信度: 中程度')
  })

  it.each([
    ['high', '確信度: 高い', 'default'],
    ['medium', '確信度: 中程度', 'default'],
    ['low', '確信度: 低い', 'warning'],
  ])('確信度 %s のラベルと強調', (level, expectedLabel, expectedTone) => {
    const got = classifyProvenance({ source_type: 'web_search', last_fetch_confidence: level })
    expect(got?.confidenceLabel).toBe(expectedLabel)
    // 低い確信度だけは警告色にして目に入るようにする
    expect(got?.tone).toBe(expectedTone)
  })

  it('AI 推定には根拠URLを添える', () => {
    const got = classifyProvenance({
      source_type: 'web_search',
      source_url: 'https://example.test/ir',
      last_model_used: 'gpt-4o-mini',
      fetched_at: '2026-08-22T10:51:50.003Z',
    })
    expect(got?.evidenceUrl).toBe('https://example.test/ir')
    expect(got?.detail).toContain('正確性は保証されません')
    expect(got?.detail).toContain('2026/08/22')
    expect(got?.detail).toContain('gpt-4o-mini')
  })

  it('公式情報には根拠リンクを付けない（出典が自明なため）', () => {
    expect(classifyProvenance({ gbiz_last_synced_at: '2026-09-01T00:00:00Z' })?.evidenceUrl).toBeUndefined()
  })

  it.each([
    ['null', null],
    ['undefined', undefined],
    ['空オブジェクト', {}],
    ['出どころ情報なし', { source_type: '', last_fetch_confidence: '' }],
  ])('%s のときは表示しない', (_label, input) => {
    expect(classifyProvenance(input as never)).toBeNull()
  })

  it('不正な確信度は無視する', () => {
    const got = classifyProvenance({ source_type: 'web_search', last_fetch_confidence: 'unknown' })
    expect(got?.kind).toBe('ai')
    expect(got?.confidenceLabel).toBeUndefined()
  })

  it('不正な日時は表示に混ぜない', () => {
    const got = classifyProvenance({ source_type: 'web_search', fetched_at: 'not-a-date' })
    expect(got?.detail).not.toContain('NaN')
    expect(got?.detail).not.toContain('Invalid')
  })
})
