/**
 * Shared constants for the interview feature.
 */
import type { Position } from './types'

export const PRIMARY = '#ec5b13'
export const BG_LIGHT = '#f8f6f6'
export const BG_DARK = '#221610'

export const POSITIONS: Position[] = [
  { id: 'engineer', title: 'ソフトウェアエンジニア', department: 'Engineering', icon: '💻', questions: 8, category: 'general' },
  { id: 'designer', title: 'プロダクトデザイナー', department: 'Design', icon: '🎨', questions: 7, category: 'general' },
  { id: 'sales', title: '営業職', department: 'Sales', icon: '📈', questions: 7, category: 'general' },
  { id: 'marketing', title: 'マーケティング', department: 'Growth', icon: '📣', questions: 6, category: 'general' },
  { id: 'pm', title: 'プロダクトマネージャー', department: 'Product', icon: '🧭', questions: 9, category: 'general' },
  { id: 'data', title: 'データアナリスト', department: 'Data', icon: '📊', questions: 7, category: 'general' },
  { id: 'se', title: 'システムエンジニア（SE）', department: 'SIer / Development', icon: '🖥️', questions: 8, category: 'sier' },
  { id: 'infra', title: 'インフラエンジニア', department: 'SIer / Infrastructure', icon: '🔧', questions: 7, category: 'sier' },
  { id: 'it_consultant', title: 'ITコンサルタント', department: 'SIer / Consulting', icon: '📋', questions: 8, category: 'sier' },
  { id: 'pmo', title: 'PMO（プロジェクト管理）', department: 'SIer / PMO', icon: '📅', questions: 7, category: 'sier' },
  { id: 'network', title: 'ネットワークエンジニア', department: 'SIer / Network', icon: '🌐', questions: 7, category: 'sier' },
  { id: 'qa', title: 'テスト・品質保証（QA）', department: 'SIer / QA', icon: '✅', questions: 6, category: 'sier' },
]
