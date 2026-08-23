import { resolveActiveStep, resolveOnboardingSteps } from '@/app/onboarding/steps'

describe('resolveOnboardingSteps (#1015)', () => {
  it('チャット開始のみ（マッチング結果は未取得）の場合、ステップ2は未完了', () => {
    const completed = resolveOnboardingSteps({
      hasChatSession: true,
      hasMatchingResults: false,
      hasInterview: false,
    })
    expect(completed).toEqual([true, false, false])
  })

  it('マッチング結果が閲覧可能な場合、ステップ2は完了', () => {
    const completed = resolveOnboardingSteps({
      hasChatSession: true,
      hasMatchingResults: true,
      hasInterview: false,
    })
    expect(completed).toEqual([true, true, false])
  })

  it('何も進んでいない場合、全て未完了', () => {
    const completed = resolveOnboardingSteps({
      hasChatSession: false,
      hasMatchingResults: false,
      hasInterview: false,
    })
    expect(completed).toEqual([false, false, false])
  })
})

describe('resolveActiveStep', () => {
  it('最初の未完了ステップを返す', () => {
    expect(resolveActiveStep([true, false, false])).toBe(1)
    expect(resolveActiveStep([false, false, false])).toBe(0)
  })

  it('全て完了していれば最後のステップを返す', () => {
    expect(resolveActiveStep([true, true, true])).toBe(2)
  })
})
