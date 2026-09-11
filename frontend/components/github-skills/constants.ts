export const CATEGORY_COLORS: Record<string, string> = {
  Frontend:       '#4FC3F7',
  Backend:        '#81C784',
  Infrastructure: '#FFB74D',
  Database:       '#F48FB1',
  Other:          '#CE93D8',
}

export const CATEGORY_LABELS: Record<string, string> = {
  Frontend:       'フロントエンド',
  Backend:        'バックエンド',
  Infrastructure: 'インフラ',
  Database:       'DB',
  Other:          'その他',
}

export const AXES_ORDER = ['Frontend', 'Backend', 'Infrastructure', 'Database', 'Other'] as const
