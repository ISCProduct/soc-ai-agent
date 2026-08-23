/**
 * オンボーディングの各ステップ完了判定。
 * UI から分離し、判定ロジックを単体テスト可能にする。
 */

export type OnboardingStepFlags = [boolean, boolean, boolean]

/**
 * 完了フラグから、次にアクティブにすべきステップの index を返す。
 * 全て完了していれば最後のステップを返す。
 */
export function resolveActiveStep(completed: OnboardingStepFlags): number {
  const nextStep = completed.findIndex((c) => !c)
  return nextStep === -1 ? completed.length - 1 : nextStep
}

/**
 * オンボーディングの完了状態を判定する。
 * - ステップ1（自己分析チャット）: チャットセッションが存在する
 * - ステップ2（企業マッチング）: マッチング結果（recommendations）が1件以上あり閲覧可能
 *   ※ チャットセッションが存在するだけでは完了にしない（#1015）
 * - ステップ3（面接練習）: 面接セッションが存在する
 */
export function resolveOnboardingSteps(args: {
  hasChatSession: boolean
  hasMatchingResults: boolean
  hasInterview: boolean
}): OnboardingStepFlags {
  return [args.hasChatSession, args.hasMatchingResults, args.hasInterview]
}
