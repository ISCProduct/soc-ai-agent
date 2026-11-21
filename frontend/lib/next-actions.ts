/**
 * 学生ホームの「次にやること」（#1576）。
 *
 * 既存の /api/schedule と /api/applications だけで組み立てる。集約APIは作らない
 * （どちらも学生本人の行だけを返し、件数も少ない）。
 *
 * 診断スコアに応じた改善の提案（SOCAIAGENT-614）は、既存の /api/portfolio（診断と面接）から組み立てる。
 * 診断・面接がまだなら始めることを、診断があれば低めの傾向を面接・ES の練習へつなぐ。
 */

import { formatDateTime } from '@/lib/i18n/format'

export type NextActionSchedule = { company_name: string; stage: string; title: string; scheduled_at: string }
export type NextActionApplication = { company_name: string; status: string }

export type NextAction = {
  key: string
  /** 何があるか（日時・企業名を含む1文）。 */
  text: string
  /** 押すと何が起きるかが分かる操作名。href が無い（この画面で完結する）ときは空 */
  actionLabel: string
  /** 無ければボタンを出さない（例: 診断はホームのチャットで進めるので移動しない） */
  href?: string
}

/** 診断と面接の状況。取れなかったときは null（改善の提案を出さないだけで、予定・応募は出す）。 */
export type GrowthStatus = {
  /** 最新の診断の10カテゴリのスコア。診断がまだなら空 */
  diagnosis: { category: string; score: number }[]
  /** AI 面接のレポートの件数（最新3件まで）。0 なら未着手 */
  interviewCount: number
}

type Practice = { kind: 'interview' | 'es'; hint: string }

/**
 * カテゴリごとの練習の提案。安定志向・ワークライフバランスは働き方の好みで、低いことが弱みではないので含めない。
 * 話して伝える力に関わるものは面接、経験を言葉にして書くものは ES の練習へつなぐ。
 */
export const CATEGORY_PRACTICE: Record<string, Practice> = {
  コミュニケーション力: { kind: 'interview', hint: '面接で、結論から短く話す練習をしてみましょう' },
  リーダーシップ志向: { kind: 'interview', hint: '周りを動かした経験を、面接で具体的に話す練習をしてみましょう' },
  チームワーク志向: { kind: 'interview', hint: 'チームで取り組んだ経験と自分の役割を、面接で話す練習をしてみましょう' },
  チャレンジ志向: { kind: 'interview', hint: '新しいことに挑戦した経験を、面接で話す練習をしてみましょう' },
  技術志向: { kind: 'es', hint: '学んだ技術や作ったものを、ES に具体的に書く練習をしてみましょう' },
  創造性志向: { kind: 'es', hint: '工夫して形にした経験を、ES に書いて整理してみましょう' },
  細部志向: { kind: 'es', hint: '丁寧に確認して防いだミスなどの経験を、ES に書いてみましょう' },
  成長志向: { kind: 'es', hint: '目標を立てて伸ばした経験を、ES に書いて整理してみましょう' },
}

/** 低めとみなすスコア（これ以上なら提案しない） */
export const WEAK_SCORE_BELOW = 50
/** 低めの判定に必要な、測れたカテゴリ（練習の対象）の数。少ないと偏りで決まってしまう */
export const MIN_MEASURED_FOR_WEAKEST = 4

/**
 * 練習の対象のカテゴリのうち、いちばん低いもの。測れた数が少ない・低めでないときは null。
 * 同じ点なら CATEGORY_PRACTICE の並び順で先のもの。
 */
export function findWeakestCategory(
  diagnosis: GrowthStatus['diagnosis'],
): { category: string; score: number; tied: boolean } | null {
  const order = Object.keys(CATEGORY_PRACTICE)
  const measured = diagnosis
    .filter((d) => d.category in CATEGORY_PRACTICE && Number.isFinite(d.score))
    .sort((a, b) => a.score - b.score || order.indexOf(a.category) - order.indexOf(b.category))
  if (measured.length < MIN_MEASURED_FOR_WEAKEST) return null
  const weakest = measured[0]
  if (weakest.score >= WEAK_SCORE_BELOW) return null
  // 2番目も同じ点なら「ほかより低め」とは言えない
  const tied = measured[1] !== undefined && measured[1].score === weakest.score
  return { category: weakest.category, score: weakest.score, tied }
}

/** 診断・面接の状況から、改善の提案を作る（並びは 診断の開始 → 低めの傾向の練習 → 面接の開始）。 */
export function pickGrowthActions(growth: GrowthStatus): NextAction[] {
  const actions: NextAction[] = []
  if (growth.diagnosis.length === 0) {
    actions.push({
      key: 'diagnosis-start',
      text: 'AI 診断がまだです。下のチャットで質問に答えると、あなたの傾向が分かります',
      actionLabel: '',
    })
  } else {
    const weakest = findWeakestCategory(growth.diagnosis)
    if (weakest) {
      const practice = CATEGORY_PRACTICE[weakest.category]
      actions.push({
        key: `weakest-${weakest.category}`,
        // 同じ点の傾向がほかにもあるときは、ほかより劣っているようには書かない
        text: weakest.tied
          ? `「${weakest.category}」が ${Math.round(weakest.score)} と低めです。${practice.hint}`
          : `「${weakest.category}」が ${Math.round(weakest.score)} と、ほかより低めです。${practice.hint}`,
        actionLabel: practice.kind === 'interview' ? '面接で練習する' : 'ES で練習する',
        href: practice.kind === 'interview' ? '/interview' : '/es-rewrite',
      })
    }
  }
  if (growth.interviewCount === 0) {
    actions.push({
      key: 'interview-start',
      // 件数はレポートのある終了済みの面接（生成待ち・失敗は数えない）なので、「練習していない」とは言い切らない
      text: 'AI 面接の練習レポートがまだありません。本番の前に一度試してみましょう',
      actionLabel: '面接を練習する',
      href: '/interview',
    })
  }
  return actions
}

/** 直近とみなす期間。これより先の予定は「次にやること」に出さない。 */
export const UPCOMING_DAYS = 14

function formatWhen(d: Date): string {
  return formatDateTime(d, { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', timeZone: 'Asia/Tokyo' })
}

/**
 * 優先順: 直近の予定（日時が近い順）→ 内定の返事 → 診断・面接の改善の提案 → 応募するか未決の企業。
 * 期限のあるものを先に出し、改善の提案はその後ろに置く。growth が null なら改善の提案は出さない。
 */
export function pickNextActions(
  events: NextActionSchedule[],
  applications: NextActionApplication[],
  now: Date,
  growth: GrowthStatus | null = null,
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
  if (growth) actions.push(...pickGrowthActions(growth))
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
