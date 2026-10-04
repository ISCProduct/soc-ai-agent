import { resolveFreshness, TTL_DAYS, type Freshness } from '@/lib/design-tokens'

/** 鮮度判定に必要な取得日時だけを受け取る。画面の Company 型に依存させない。 */
export type CompanyFetchTimes = {
  info_fetched_at?: string | null
  tech_fetched_at?: string | null
  relations_fetched_at?: string | null
  jobs_fetched_at?: string | null
}

export type CompanyFreshness = {
  freshness: Freshness
  /** 「会社概要が期限切れ」のように、どの系統かを添える。 */
  detail: string
}

/**
 * 系統ごとの TTL。バックエンド `companyfetch.TTL*`（text.go）と対応させている。
 * ずれると画面の「期限切れ」とバッチの再取得対象が食い違う。
 */
const ASPECTS = [
  { label: '会社概要', key: 'info_fetched_at', ttl: TTL_DAYS.info },
  { label: '関連企業', key: 'relations_fetched_at', ttl: TTL_DAYS.relations },
  { label: '技術情報', key: 'tech_fetched_at', ttl: TTL_DAYS.tech },
  { label: '求人', key: 'jobs_fetched_at', ttl: TTL_DAYS.jobs },
] as const

/**
 * 対応の緊急度。未取得が最優先。
 *
 * 未取得は学生に見せる情報がまだ無い状態で、期限切れは古い情報が出ている状態。
 * 前者のほうが先に埋める必要がある。
 */
const URGENCY: Record<Freshness, number> = {
  unfetched: 3,
  expired: 2,
  due: 1,
  fresh: 0,
}

/**
 * 企業1社の鮮度を、最も緊急な系統で代表させる。
 *
 * 一覧の既存表示は「欠損している系統」は出すが、取得済みデータが TTL を超えて
 * 古くなっていることは出していなかった。バッチの再取得対象には入るのに画面からは
 * 見えないため、同じ企業が毎回取得され続けていても気づけない。
 *
 * 一覧に出すのは1つだけにする。4系統を並べると行が長くなり、
 * 「この行に対応が必要か」という判断が遅くなる。内訳は詳細画面で見る。
 */
export function companyFreshness(c: CompanyFetchTimes, now: Date = new Date()): CompanyFreshness {
  let worst: Freshness = 'fresh'
  let label = ''

  for (const aspect of ASPECTS) {
    const f = resolveFreshness(c[aspect.key], aspect.ttl, now)
    if (URGENCY[f] > URGENCY[worst]) {
      worst = f
      label = aspect.label
    }
  }

  if (worst === 'fresh') return { freshness: 'fresh', detail: '' }
  return { freshness: worst, detail: label }
}
