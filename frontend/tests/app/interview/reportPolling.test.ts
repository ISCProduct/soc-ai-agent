/**
 * @jest-environment jsdom
 */
import {
  evaluateReportPollTick,
  REPORT_POLL_INTERVAL_MS,
  REPORT_POLL_TIMEOUT_MS,
} from '@/app/interview/reportPolling'

describe('reportPolling', () => {
  it('定数は 3 秒間隔・3 分タイムアウトである', () => {
    expect(REPORT_POLL_INTERVAL_MS).toBe(3000)
    expect(REPORT_POLL_TIMEOUT_MS).toBe(3 * 60 * 1000)
  })

  it('fetch 失敗時は error を返す', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 1000,
        hasReport: false,
        fetchFailed: true,
      }),
    ).toBe('error')
  })

  it('report がある場合は ready を返す（タイムアウト前でも）', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS + 1,
        hasReport: true,
        fetchFailed: false,
      }),
    ).toBe('ready')
  })

  it('未到着かつ制限時間未満なら continue', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS - 1,
        hasReport: false,
        fetchFailed: false,
      }),
    ).toBe('continue')
  })

  it('未到着かつ制限時間以上なら timeout', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS,
        hasReport: false,
        fetchFailed: false,
      }),
    ).toBe('timeout')
  })

  it('fetchFailed は hasReport より優先される', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 0,
        hasReport: true,
        fetchFailed: true,
      }),
    ).toBe('error')
  })
})
