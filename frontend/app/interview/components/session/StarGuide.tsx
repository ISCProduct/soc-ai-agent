import { Box, Typography } from '@mui/material'

import { CALL } from '../../callTokens'

/** 回答に迷ったときの思考の補助線。チケット固有の採点基準ではなく、どの質問にも共通する一般的な枠組み。 */
export const STAR_GUIDE = [
  { key: 'S', label: '状況', description: 'どんな状況・背景だったか' },
  { key: 'T', label: '課題', description: '達成すべき課題は何だったか' },
  { key: 'A', label: '行動', description: '自分が具体的に取った行動' },
  { key: 'R', label: '結果', description: '行動の結果どうなったか' },
] as const

/** 「ヒント」パネルの中身。STAR（状況・課題・行動・結果）の順に答えるための4項目。 */
export default function StarGuide() {
  return (
    <Box>
      <Typography sx={{ fontSize: 13, color: CALL.soft, mb: 1.5 }}>答え方の順番（STAR）</Typography>
      <Box component="ol" sx={{ listStyle: 'none', p: 0, m: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
        {STAR_GUIDE.map((s) => (
          <Box component="li" key={s.key} sx={{ display: 'flex', gap: 1.5, alignItems: 'center' }}>
            <Box aria-hidden sx={{ width: 28, height: 28, borderRadius: '50%', bgcolor: CALL.control, color: CALL.ink, display: 'grid', placeItems: 'center', fontSize: 12, fontWeight: 700, flexShrink: 0 }}>
              {s.key}
            </Box>
            <Box>
              <Typography sx={{ fontSize: 14, fontWeight: 700, color: CALL.ink }}>{s.label}</Typography>
              <Typography sx={{ fontSize: 12, color: CALL.muted }}>{s.description}</Typography>
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  )
}
