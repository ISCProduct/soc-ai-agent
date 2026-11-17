/**
 * @jest-environment jsdom
 */
import {
  advanceProgress,
  INITIAL_PROGRESS_TRACKER,
  parseReportProgress,
  type ReportProgress,
} from '@/app/interview/reportProgress'

describe('parseReportProgress', () => {
  it.each([
    ['report_stage が無い（旧バックエンド）', { id: 1 }],
    ['未知の段階', { report_stage: 'unknown' }],
    ['session が null', null],
  ])('%s は null', (_name, session) => {
    expect(parseReportProgress(session)).toBeNull()
  })

  it('段階・%・失敗理由を取り出し、%は 0〜100 に丸める', () => {
    expect(parseReportProgress({ report_stage: 'failed', report_percent: 150, report_failed_reason: 'llm_timeout' }))
      .toEqual({ stage: 'failed', percent: 100, failedReason: 'llm_timeout' })
  })
})

describe('advanceProgress', () => {
  const gen: ReportProgress = { stage: 'generating', percent: 10 }

  it('generating は経過時間で 10→85 に寄せ、120秒で 85', () => {
    const t0 = advanceProgress(INITIAL_PROGRESS_TRACKER, gen, 0)
    expect(t0.view.percent).toBe(10)
    const mid = advanceProgress(t0.tracker, gen, 60_000)
    expect(mid.view.percent).toBe(47)
    const end = advanceProgress(mid.tracker, gen, 600_000)
    expect(end.view.percent).toBe(85)
  })

  it('サーバー値が小さく戻っても表示は後退しない', () => {
    const a = advanceProgress(INITIAL_PROGRESS_TRACKER, { stage: 'saving', percent: 90 }, 0)
    const b = advanceProgress(a.tracker, { stage: 'loading', percent: 5 }, 1000)
    expect(b.view.percent).toBe(90)
  })

  it('done 以外は 99% を超えない。done は 100%', () => {
    expect(advanceProgress(INITIAL_PROGRESS_TRACKER, { stage: 'saving', percent: 100 }, 0).view.percent).toBe(99)
    expect(advanceProgress(INITIAL_PROGRESS_TRACKER, { stage: 'done', percent: 100 }, 0).view.percent).toBe(100)
  })

  it('同じ段階が 60 秒続くと停滞（generating は 10 分まで待つ）', () => {
    const q: ReportProgress = { stage: 'queued', percent: 0 }
    const q0 = advanceProgress(INITIAL_PROGRESS_TRACKER, q, 0)
    expect(advanceProgress(q0.tracker, q, 59_999).view.stalled).toBe(false)
    expect(advanceProgress(q0.tracker, q, 60_000).view.stalled).toBe(true)

    const g0 = advanceProgress(INITIAL_PROGRESS_TRACKER, gen, 0)
    expect(advanceProgress(g0.tracker, gen, 9 * 60_000).view.stalled).toBe(false)
    expect(advanceProgress(g0.tracker, gen, 10 * 60_000).view.stalled).toBe(true)
  })

  it('段階が進むと停滞の計測がやり直しになる', () => {
    const q0 = advanceProgress(INITIAL_PROGRESS_TRACKER, { stage: 'queued', percent: 0 }, 0)
    const l = advanceProgress(q0.tracker, { stage: 'loading', percent: 5 }, 100_000)
    expect(l.view.stalled).toBe(false)
  })
})
