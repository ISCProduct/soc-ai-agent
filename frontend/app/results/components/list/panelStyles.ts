import { UI } from '@/lib/design/tokens'

/** サイドの罫線パネル（影なし・角丸8px） */
export const panelSx = {
  border: `1px solid ${UI.rule}`,
  borderRadius: '8px',
  backgroundColor: UI.field,
  p: 2.75,
} as const

/** UI トークンを薄めた背景色（色の直書きを避ける） */
export const tint = (color: string, percent = 12): string =>
  `color-mix(in srgb, ${color} ${percent}%, transparent)`
