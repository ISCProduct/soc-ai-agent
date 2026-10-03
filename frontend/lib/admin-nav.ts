/**
 * 管理画面のナビゲーション定義。
 *
 * これまで `/admin` のハブ画面（page-content.tsx）が唯一の入口で、
 * 分類と遷移先をその画面のローカル配列として持っていた。27画面に共通シェルが無く、
 * 下位画面へ入ると「戻る」以外の移動手段が無かったため、ここへ切り出して
 * ハブ画面とサイドバーの両方が同じ定義を読むようにする。
 *
 * 分類は既にハブ画面で使われていたものをそのまま使う（独自の分類を作らない）。
 */

export type AdminNavItem = {
  /** サイドバーとハブカードの見出しに使う。 */
  title: string
  /** ハブカードの説明文。サイドバーでは使わない。 */
  description: string
  href: string
  /** ハブカードのボタン文言。具体的な操作名にする（§13）。 */
  cta: string
}

export type AdminNavGroup = {
  heading: string
  /** true のときはプラットフォーム管理者だけに見せる。 */
  platformOnly?: boolean
  items: AdminNavItem[]
}

export const ADMIN_NAV: AdminNavGroup[] = [
  {
    heading: 'システム管理',
    platformOnly: true,
    items: [
      {
        title: '学園(組織)管理',
        description: '学園サブドメインの登録・契約プラン・契約期間を管理します。',
        href: '/admin/organizations',
        cta: '学園管理へ',
      },
      {
        title: '監査ログ',
        description: '管理者操作の履歴を確認できます。',
        href: '/admin/audit-logs',
        cta: '監査ログへ',
      },
      {
        title: 'APIコストモニタリング',
        description: 'OpenAI APIの日次・月次コストとモデル別内訳を可視化します。',
        href: '/admin/costs',
        cta: 'コスト管理へ',
      },
      {
        title: 'ベクトルDB / RAG運用',
        description: 'Chroma のインデックス状況確認と企業単位の再埋め込みを行います。',
        href: '/admin/vector-db',
        cta: 'ベクトルDB管理へ',
      },
      {
        title: 'スコア精度検証',
        description: '相関分析・フェーズ別メトリクス・A/Bテスト・キャリブレーションを管理します。',
        href: '/admin/score-validation',
        cta: 'スコア精度検証へ',
      },
      {
        title: 'プロファイル再計算',
        description: '企業マッチングプロファイルを一括または個別に再計算します。',
        href: '/admin/profile-recalculation',
        cta: 'プロファイル再計算へ',
      },
      {
        title: '集合知サマリー再構築',
        description: '全企業の行動サマリーをバッチ再集計します。',
        href: '/admin/collective-insights',
        cta: '集合知管理へ',
      },
      {
        title: 'スクレイパーセッション',
        description: 'クローリングに使用するサイトごとのセッション（Cookie）を管理します。',
        href: '/admin/scraper-sessions',
        cta: 'セッション管理へ',
      },
    ],
  },
  {
    heading: '学校運営',
    items: [
      {
        title: '企業情報',
        description: '学生に見せる企業の登録・確認・公開を行います。',
        href: '/admin/companies',
        cta: '企業情報の管理へ',
      },
      {
        title: '求人管理',
        description: '企業に紐づく求人ポジションの登録・確認を行います。',
        href: '/admin/job-positions',
        cta: '求人管理へ',
      },
      {
        title: '応募・選考管理',
        description: '学生の応募一覧を確認し、選考ステータスを更新します。',
        href: '/admin/applications',
        cta: '応募管理へ',
      },
      {
        title: '卒業生の就職情報',
        description: '卒業生の就職先情報の登録・確認を行います。',
        href: '/admin/graduate-employments',
        cta: '就職情報管理へ',
      },
      {
        title: 'ユーザー管理',
        description: '担当校のユーザー情報を確認します。',
        href: '/admin/users',
        cta: 'ユーザー管理へ',
      },
    ],
  },
  {
    heading: '教員業務',
    items: [
      {
        title: '面接管理',
        description: '担当校の面接セッションと録画動画を確認できます。',
        href: '/admin/interviews',
        cta: '面接管理へ',
      },
      {
        title: 'スコアダッシュボード',
        description: 'ユーザー別の練習回数・平均スコア・スコア推移を一覧比較します。',
        href: '/admin/dashboard',
        cta: 'ダッシュボードへ',
      },
      {
        title: '生徒の傾向分析',
        description: '担当する生徒のタイプと向いている業界を一覧比較します（参考情報）。',
        href: '/admin/student-insights',
        cta: '傾向分析へ',
      },
    ],
  },
]

/**
 * 権限に応じて見せるグループを返す。
 *
 * `isPlatform` が未確定（null）のあいだは platformOnly を出さない。
 * 読み込み中に出して直後に消すと、押そうとした項目が消える。
 */
export function visibleAdminNav(isPlatform: boolean | null): AdminNavGroup[] {
  return ADMIN_NAV.filter((g) => !g.platformOnly || isPlatform === true)
}

/**
 * 現在のパスに対応する項目を返す。無ければ null。
 *
 * 前方一致で見るのは、`/admin/companies/12/edit` のような下位パスでも
 * 「企業情報」を現在地として示すため。最長一致を取る。
 * 単純に最初の一致を返すと、`/admin/companies` が
 * `/admin/companies/[id]/relations` より先に当たって粒度が落ちる。
 */
export function activeAdminNavItem(pathname: string): AdminNavItem | null {
  let best: AdminNavItem | null = null
  for (const group of ADMIN_NAV) {
    for (const item of group.items) {
      if (pathname !== item.href && !pathname.startsWith(item.href + '/')) continue
      if (!best || item.href.length > best.href.length) best = item
    }
  }
  return best
}
