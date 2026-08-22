import {
  applyBatchWave,
  batchItemFailuresFromResponse,
  batchProgressPercent,
  batchWaveFromResponse,
  candidateCountFromResponse,
  emptyBatchProgress,
  explainBatchItemError,
  formatBatchFailureDetail,
  formatBatchProgressLabel,
  formatMissingBatchLogLine,
  shouldContinueBatch,
  type BatchWave,
} from '@/lib/admin-company-batch-progress'

const wave = (over: Partial<BatchWave> = {}): BatchWave => ({
  processed: 6,
  info_ok: 4,
  tech_ok: 3,
  relations_ok: 2,
  skipped: 0,
  errors: 0,
  ...over,
})

describe('admin-company-batch-progress', () => {
  it('accumulates waves and keeps the bar below 100% while a full wave remains', () => {
    const first = applyBatchWave(emptyBatchProgress(20), wave(), 6)
    expect(first.processed).toBe(6)
    expect(first.infoOk).toBe(4)
    expect(first.estimated).toBeGreaterThanOrEqual(12)
    expect(batchProgressPercent(first)).toBeLessThan(100)

    const last = applyBatchWave(first, wave({ processed: 2, info_ok: 1, tech_ok: 0, relations_ok: 0 }), 6)
    expect(last.processed).toBe(8)
    expect(last.rounds).toBe(2)
    expect(batchProgressPercent(last)).toBeGreaterThan(0)
  })

  it('stops on empty, partial, all-skip, or max rounds', () => {
    expect(shouldContinueBatch(wave({ processed: 0 }), 6, 1, 20)).toBe(false)
    expect(shouldContinueBatch(wave({ processed: 3 }), 6, 1, 20)).toBe(false)
    expect(shouldContinueBatch(wave({ info_ok: 0, tech_ok: 0, relations_ok: 0 }), 6, 1, 20)).toBe(false)
    expect(shouldContinueBatch(wave(), 6, 20, 20)).toBe(false)
    expect(shouldContinueBatch(wave(), 6, 1, 20)).toBe(true)
  })

  it('formats running and done labels with counts', () => {
    const p = applyBatchWave(emptyBatchProgress(18), wave(), 6)
    expect(formatBatchProgressLabel(p, true)).toContain('取得中')
    expect(formatBatchProgressLabel(p, true)).toContain('6')
    expect(formatBatchProgressLabel(p, false)).toContain('取得完了')
    expect(formatBatchProgressLabel(p, false)).toContain('会社概要 4')
  })

  it('parses backend JSON fields', () => {
    const wave = batchWaveFromResponse({
      processed: 6,
      info_ok: 2,
      tech_ok: 1,
      relations_ok: 3,
      skipped: 1,
      errors: 0,
    })
    expect(wave).toEqual({
      processed: 6,
      info_ok: 2,
      tech_ok: 1,
      relations_ok: 3,
      persona_ok: 0,
      skipped: 1,
      errors: 0,
    })
    expect(candidateCountFromResponse({ candidate_n: 50 })).toBe(50)
    expect(candidateCountFromResponse({ candidate_count: 12 })).toBe(12)
  })

  it('summarizes identical item errors as one cause', () => {
    const data = {
      items: [
        { name: 'A社', error: 'info: budget' },
        { name: 'B社', error: 'tech: budget' },
      ],
    }
    const failures = batchItemFailuresFromResponse(data)
    expect(failures).toHaveLength(2)
    expect(explainBatchItemError('info: budget')).toContain('月次')
    expect(formatBatchFailureDetail(failures)).toContain('2 社とも')
    expect(formatBatchFailureDetail(failures)).toContain('コスト画面')
  })

  it('formats a grep-able log line with stop_reason and item errors', () => {
    const line = formatMissingBatchLogLine({
      stop_reason: 'all_failed',
      processed: 6,
      info_ok: 0,
      tech_ok: 0,
      relations_ok: 0,
      errors: 6,
      items: [{ company_id: 12, name: 'A社', error: 'info: budget' }],
    })
    expect(line).toContain('[fetch_missing_batch]')
    expect(line).toContain('stop_reason=all_failed')
    expect(line).toContain('id=12')
    expect(line).toContain('info: budget')
    expect(formatMissingBatchLogLine({ dry_run: true, processed: 0 })).toBeNull()
  })
})
