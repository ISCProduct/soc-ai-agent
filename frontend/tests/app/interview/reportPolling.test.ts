/**
 * @jest-environment jsdom
 */
import {
  evaluateReportPollTick,
  REPORT_POLL_INTERVAL_MS,
  REPORT_POLL_MAX_CONSECUTIVE_FAILURES,
  REPORT_POLL_TIMEOUT_MS,
} from '@/app/interview/reportPolling'

describe('reportPolling', () => {
  it('定数は 3 秒間隔・3 分タイムアウト・連続3回失敗で打ち切りである', () => {
    expect(REPORT_POLL_INTERVAL_MS).toBe(3000)
    expect(REPORT_POLL_TIMEOUT_MS).toBe(3 * 60 * 1000)
    expect(REPORT_POLL_MAX_CONSECUTIVE_FAILURES).toBe(3)
  })

  it('report がある場合は ready を返す（タイムアウト前でも）', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS + 1,
        hasReport: true,
        consecutiveFailures: 0,
      }),
    ).toBe('ready')
  })

  it('未到着かつ制限時間未満なら continue', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS - 1,
        hasReport: false,
        consecutiveFailures: 0,
      }),
    ).toBe('continue')
  })

  it('未到着かつ制限時間以上なら timeout', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS,
        hasReport: false,
        consecutiveFailures: 0,
      }),
    ).toBe('timeout')
  })
})

// #1057: 瞬断1回でポーリングを止めないこと
describe('reportPolling 連続失敗の扱い (#1057)', () => {
  it.each([1, 2])('連続失敗が閾値未満(%i回)なら continue でポーリングを継続する', failures => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 1000,
        hasReport: false,
        consecutiveFailures: failures,
      }),
    ).toBe('continue')
  })

  it('連続失敗が閾値に達したら error にする', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 1000,
        hasReport: false,
        consecutiveFailures: REPORT_POLL_MAX_CONSECUTIVE_FAILURES,
      }),
    ).toBe('error')
  })

  it('閾値未満の失敗中でもタイムアウトには到達する', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: REPORT_POLL_TIMEOUT_MS,
        hasReport: false,
        consecutiveFailures: 1,
      }),
    ).toBe('timeout')
  })

  it('閾値は maxConsecutiveFailures で上書きできる', () => {
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 1000,
        hasReport: false,
        consecutiveFailures: 1,
        maxConsecutiveFailures: 1,
      }),
    ).toBe('error')
  })

  it('過去に失敗していても report が取得できたら ready を優先する', () => {
    // 呼び出し側は成功時にカウンタを0へ戻すが、順序に依存せず結果を捨てないことを保証する
    expect(
      evaluateReportPollTick({
        startedAtMs: 0,
        nowMs: 1000,
        hasReport: true,
        consecutiveFailures: REPORT_POLL_MAX_CONSECUTIVE_FAILURES,
      }),
    ).toBe('ready')
  })
})
