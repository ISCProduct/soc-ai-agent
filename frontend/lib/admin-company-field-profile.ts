/**
 * 業界ごとに入力・表示する企業フィールドの定義。
 * Backend/internal/companyfields/profile.go と対応を揃えること。
 */

export type IndustryProfileId =
  | 'it'
  | 'manufacturing'
  | 'finance'
  | 'consulting'
  | 'education'
  | 'healthcare'
  | 'general'

export type InfoFieldKey =
  | 'description'
  | 'industry'
  | 'location'
  | 'website_url'
  | 'founded_year'
  | 'employee_count'
  | 'main_business'
  | 'culture'
  | 'work_style'
  | 'welfare_details'
  | 'tech_stack'

export type TechFieldKey = 'tech_stack' | 'infra_stack' | 'cicd_tools' | 'development_style'

export type TechFieldSpec = {
  key: TechFieldKey
  label: string
  placeholder?: string
  /** development_style 用の選択肢。未指定時は IT 向け既定を使う */
  options?: string[]
}

export type IndustryFieldProfile = {
  id: IndustryProfileId
  label: string
  matchKeywords: string[]
  infoFields: InfoFieldKey[]
  techAspectEnabled: boolean
  /** タブ表示名（技術情報 / 設備・技術 など） */
  techAspectLabel: string
  techFields: TechFieldSpec[]
  requireTechForPublish: boolean
  /** 技術タブが無いときの案内文 */
  techDisabledMessage?: string
}

const COMMON_INFO_FIELDS: InfoFieldKey[] = [
  'description',
  'industry',
  'location',
  'website_url',
  'founded_year',
  'employee_count',
  'main_business',
  'culture',
  'work_style',
  'welfare_details',
]

const IT_DEV_STYLES = ['スクラム', 'ウォーターフォール', 'カンバン', 'アジャイル', 'その他']
const MFG_DEV_STYLES = ['セル生産', 'ライン生産', '受注生産', '見込み生産', 'その他']

/**
 * マッチ優先順。キーワードが industry に含まれたらそのプロファイルを採用。
 * 末尾の general がフォールバック。
 */
export const INDUSTRY_FIELD_PROFILES: IndustryFieldProfile[] = [
  {
    id: 'it',
    label: 'IT・ソフトウェア',
    matchKeywords: [
      'it',
      'ｉｔ',
      '情報',
      'ソフト',
      'software',
      'web',
      'ウェブ',
      'saas',
      'システム開発',
      '通信',
      'インターネット',
      'ゲーム',
    ],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: true,
    techAspectLabel: '技術情報',
    techFields: [
      {
        key: 'tech_stack',
        label: '言語・フレームワーク',
        placeholder: '例: Go, React, TypeScript',
      },
      {
        key: 'infra_stack',
        label: 'インフラ',
        placeholder: '例: AWS, GCP, Azure, オンプレ',
      },
      {
        key: 'cicd_tools',
        label: 'CI/CDツール',
        placeholder: '例: GitHub Actions, Jenkins, CircleCI',
      },
      {
        key: 'development_style',
        label: '開発手法',
        options: IT_DEV_STYLES,
      },
    ],
    requireTechForPublish: true,
  },
  {
    id: 'manufacturing',
    label: '製造業',
    matchKeywords: ['製造', 'メーカー', '自動車', '機械', '電機', '電子', 'ものづくり', '工場'],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: true,
    techAspectLabel: '設備・技術',
    techFields: [
      {
        key: 'tech_stack',
        label: '主要技術・製品技術',
        placeholder: '例: 精密加工, 車載センサー, CAD/CAM',
      },
      {
        key: 'infra_stack',
        label: '生産設備・拠点',
        placeholder: '例: 国内工場, SMTライン, クリーンルーム',
      },
      {
        key: 'development_style',
        label: '生産・開発の進め方',
        options: MFG_DEV_STYLES,
      },
    ],
    requireTechForPublish: false,
  },
  {
    id: 'finance',
    label: '金融・保険',
    matchKeywords: ['金融', '銀行', '保険', '証券', 'クレジット'],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: false,
    techAspectLabel: '技術情報',
    techFields: [],
    requireTechForPublish: false,
    techDisabledMessage: '金融・保険業では、プログラミング技術スタックの登録は不要です。会社概要と関連企業を確認してください。',
  },
  {
    id: 'consulting',
    label: 'コンサルティング',
    matchKeywords: ['コンサル'],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: false,
    techAspectLabel: '技術情報',
    techFields: [],
    requireTechForPublish: false,
    techDisabledMessage: 'コンサルティング業では技術スタックの登録は必須ではありません。会社概要と関連企業を確認してください。',
  },
  {
    id: 'education',
    label: '教育',
    matchKeywords: ['教育', '学校', '学習', '大学', '塾'],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: false,
    techAspectLabel: '技術情報',
    techFields: [],
    requireTechForPublish: false,
    techDisabledMessage: '教育業界では技術スタックの登録は不要です。会社概要と関連企業を確認してください。',
  },
  {
    id: 'healthcare',
    label: '医療・福祉',
    matchKeywords: ['医療', '福祉', '病院', 'ヘルスケア', '介護'],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: false,
    techAspectLabel: '技術情報',
    techFields: [],
    requireTechForPublish: false,
    techDisabledMessage: '医療・福祉では技術スタックの登録は不要です。会社概要と関連企業を確認してください。',
  },
  {
    id: 'general',
    label: 'その他',
    matchKeywords: [],
    infoFields: COMMON_INFO_FIELDS,
    techAspectEnabled: false,
    techAspectLabel: '技術情報',
    techFields: [],
    requireTechForPublish: false,
    techDisabledMessage:
      'この業界では技術スタックの登録は必須ではありません。必要な場合は業種を IT・ソフトウェアや製造業に近い名称へ変更してください。',
  },
]

export function resolveIndustryFieldProfile(industry?: string | null): IndustryFieldProfile {
  const normalized = (industry || '').trim().toLowerCase()
  if (!normalized) {
    return INDUSTRY_FIELD_PROFILES[INDUSTRY_FIELD_PROFILES.length - 1]
  }
  for (const profile of INDUSTRY_FIELD_PROFILES) {
    if (profile.id === 'general') continue
    if (profile.matchKeywords.some((kw) => normalized.includes(kw.toLowerCase()))) {
      return profile
    }
  }
  return INDUSTRY_FIELD_PROFILES[INDUSTRY_FIELD_PROFILES.length - 1]
}

export function infoFieldEnabled(profile: IndustryFieldProfile, key: InfoFieldKey): boolean {
  return profile.infoFields.includes(key)
}

export function techFieldSpec(profile: IndustryFieldProfile, key: TechFieldKey): TechFieldSpec | undefined {
  return profile.techFields.find((f) => f.key === key)
}
