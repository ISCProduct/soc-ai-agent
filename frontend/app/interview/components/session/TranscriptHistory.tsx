import { Box, Typography } from '@mui/material'

import { CALL } from '../../callTokens'
import type { Utterance } from '../../types'

const ROLE_LABEL: Record<Utterance['role'], string> = { ai: '面接官', user: 'あなた' }

/** 「記録」パネルの中身。面接官とあなたの発話を時系列で並べる。 */
export default function TranscriptHistory({ utterances }: { utterances: readonly Utterance[] }) {
  if (utterances.length === 0) {
    return <Typography sx={{ fontSize: 13, color: CALL.muted }}>面接が始まると会話がここに記録されます</Typography>
  }
  return (
    <Box component="ul" sx={{ listStyle: 'none', p: 0, m: 0, display: 'flex', flexDirection: 'column', gap: 1.5 }}>
      {utterances.map((u, i) => (
        <Box component="li" key={i}>
          <Typography sx={{ fontSize: 11, color: CALL.muted, fontWeight: u.role === 'user' ? 700 : 400 }}>{ROLE_LABEL[u.role]}</Typography>
          <Typography sx={{ fontSize: 13, color: CALL.ink, lineHeight: 1.6 }}>{u.text}</Typography>
        </Box>
      ))}
    </Box>
  )
}
