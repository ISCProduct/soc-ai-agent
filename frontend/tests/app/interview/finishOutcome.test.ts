import {
  FINISH_FAILED_MESSAGE,
  FORCED_STOP_MESSAGE,
  resolveFinishOutcomeMessage,
} from '@/app/interview/finishOutcome'

describe('resolveFinishOutcomeMessage (#1015)', () => {
  it('finishSessionが失敗した場合、握りつぶさずエラーメッセージを返す', () => {
    expect(resolveFinishOutcomeMessage({ finishFailed: true, forced: false })).toBe(FINISH_FAILED_MESSAGE)
  })

  it('finishFailedはforcedより優先される', () => {
    expect(resolveFinishOutcomeMessage({ finishFailed: true, forced: true })).toBe(FINISH_FAILED_MESSAGE)
  })

  it('強制終了（時間切れ）のみの場合はその旨のメッセージを返す', () => {
    expect(resolveFinishOutcomeMessage({ finishFailed: false, forced: true })).toBe(FORCED_STOP_MESSAGE)
  })

  it('正常終了の場合はnull', () => {
    expect(resolveFinishOutcomeMessage({ finishFailed: false, forced: false })).toBeNull()
  })
})
