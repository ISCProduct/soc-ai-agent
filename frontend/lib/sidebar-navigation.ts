/** 分析進捗に応じた「今日やること」文言 */
export function getTodayActionLabel(overallProgress: number): string {
  if (overallProgress <= 0) {
    return '① 自己分析チャットを始めましょう'
  }
  if (overallProgress < 80) {
    return '① チャットを続けて分析を完成させましょう'
  }
  return '② マッチング結果を確認しましょう'
}

/** 「今日やること」内のマッチング結果 CTA を出すか（80%以上かつ未完了） */
export function shouldShowResultsCta(overallProgress: number): boolean {
  return overallProgress >= 80 && overallProgress < 100
}

/** 選考管理ナビの遷移先（ゲストはログインへ） */
export function getApplicationsNavTarget(isGuest: boolean): string {
  return isGuest ? '/login' : '/applications'
}
