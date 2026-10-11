/**
 * 学生ホームの「次にやること」（#1576）。
 *
 * 既存の /api/schedule と /api/applications だけで組み立てる。集約APIは作らない
 * （どちらも学生本人の行だけを返し、件数も少ない）。
 */

export type NextActionSchedule = { company_name: string; stage: string; title: string; scheduled_at: string }
export type NextActionApplication = { company_name: string; status: string }

export type NextAction = {
  key: string
  /** 何があるか（日時・企業名を含む1文）。 */
  text: string
  /** 押すと何が起きるかが分かる操作名。 */
  actionLabel: string
  href: string
}

/** 直近とみなす期間。これより先の予定は「次にやること」に出さない。 */
export const UPCOMING_DAYS = 14

function formatWhen(d: Date): string {
  return d.toLocaleString('ja-JP', { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Tokyo' })
}

/** 優先順: 直近の予定（日時が近い順）→ 内定の返事 → 応募するか未決の企業。 */
export function pickNextActions(
  events: NextActionSchedule[],
  applications: NextActionApplication[],
  now: Date,
): NextAction[] {
  const limit = now.getTime() + UPCOMING_DAYS * 24 * 60 * 60 * 1000
  const upcoming = events
    .map((e) => ({ e, t: new Date(e.scheduled_at).getTime() }))
    .filter(({ t }) => Number.isFinite(t) && t >= now.getTime() && t <= limit)
    .sort((a, b) => a.t - b.t)
    .map(({ e, t }, i): NextAction => ({
      key: `schedule-${i}`,
      text: `${formatWhen(new Date(t))} ${e.company_name} ${e.title || e.stage}`,
      actionLabel: '予定を確認する',
      href: '/schedule',
    }))

  const actions = [...upcoming]
  const offered = applications.filter((a) => a.status === 'offered')
  for (const a of offered) {
    actions.push({
      key: `offered-${a.company_name}`,
      text: `${a.company_name} から内定が出ています。承諾するか決めましょう`,
      actionLabel: '選考状況を開く',
      href: '/applications',
    })
  }
  const undecided = applications.filter((a) => a.status === 'not_applied').length
  if (undecided > 0) {
    actions.push({
      key: 'not-applied',
      text: `応募するか決めていない企業が${undecided}社あります`,
      actionLabel: '選考状況を開く',
      href: '/applications',
    })
  }
  return actions
}
