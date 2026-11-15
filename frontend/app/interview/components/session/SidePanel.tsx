import { Box, IconButton, Typography } from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'

import { CALL } from '../../callTokens'
import type { Utterance } from '../../types'
import type { SidePanelKind } from './ControlBar'
import { SIDE_PANEL_WIDTH, TAP_TARGET, WIDE_BREAKPOINT } from './sessionLayout'
import StarGuide from './StarGuide'
import TranscriptHistory from './TranscriptHistory'

const TITLE: Record<SidePanelKind, string> = { hint: 'ヒント', log: '会話の記録' }

interface Props {
  kind: SidePanelKind
  utterances: readonly Utterance[]
  onClose: () => void
}

/** 右サイドパネル。広い画面ではタイルを縮めて横に並び、狭い画面ではタイルに重ねて全面に出す。 */
export default function SidePanel({ kind, utterances, onClose }: Props) {
  return (
    <Box
      component="aside"
      aria-label={TITLE[kind]}
      sx={{
        bgcolor: CALL.panel, borderRadius: 2, p: 2.5, overflowY: 'auto',
        position: { xs: 'absolute', [WIDE_BREAKPOINT]: 'static' },
        inset: { xs: 0, [WIDE_BREAKPOINT]: 'auto' },
        zIndex: 4,
        flex: { [WIDE_BREAKPOINT]: `0 0 ${SIDE_PANEL_WIDTH}px` },
      }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Typography component="h2" sx={{ fontSize: 16, fontWeight: 700, color: CALL.ink }}>{TITLE[kind]}</Typography>
        <IconButton onClick={onClose} aria-label="閉じる" sx={{ width: TAP_TARGET, height: TAP_TARGET, color: CALL.soft }}>
          <CloseIcon />
        </IconButton>
      </Box>
      {kind === 'hint' ? <StarGuide /> : <TranscriptHistory utterances={utterances} />}
    </Box>
  )
}
