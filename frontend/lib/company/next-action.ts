export type PortalActionLink = {
  label: string
  href: string
}

export type NextCompanyAction = {
  title: string
  body: string
  primary: PortalActionLink
  secondary?: PortalActionLink
}

/**
 * ポータルトップで最初に出す行動。
 * 未対応の応募があるなら確認、無ければ求人が無いうちは求人作成、
 * どちらも片付いていれば学生を探す。
 */
export function nextCompanyAction(input: {
  publishedJobs: number
  pendingApplications: number
}): NextCompanyAction {
  if (input.pendingApplications > 0) {
    const count = input.pendingApplications
    return {
      title: `未対応の応募が${count}件あります`,
      body: '選考を止めたままにすると、学生の応募が次の段階へ進みません。',
      primary: { label: '応募を確認する', href: '/company-portal/applications' },
      secondary: { label: '求人を確認する', href: '/company-portal/jobs' },
    }
  }
  if (input.publishedJobs === 0) {
    return {
      title: '求人を公開すると、学生からの応募が届きます',
      body: '公開中の求人が0件のため、応募は発生しません。スカウトを送っても、返信した学生が応募できる求人がない状態になります。',
      primary: { label: '求人を作成する', href: '/company-portal/jobs' },
      secondary: { label: '先に学生を探す', href: '/company-portal/students' },
    }
  }
  return {
    title: '未対応の応募はありません',
    body: `公開中の求人は${input.publishedJobs}件です。新しい候補者を探すか、求人の内容を見直せます。`,
    primary: { label: '学生を探す', href: '/company-portal/students' },
    secondary: { label: '求人を確認する', href: '/company-portal/jobs' },
  }
}
