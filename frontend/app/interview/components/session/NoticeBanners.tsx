import { Box, Typography } from '@mui/material'

import { UTTERANCE_SAVE_FAILED_MESSAGE } from '../../utteranceSave'
import { CALL } from '../../callTokens'

interface Props {
  /** 残り時間の警告文。出さないときは null */
  warningText: string | null
  sessionEnded: boolean
  utteranceSaveFailed: boolean
}

/** タイルの上に積む通知。面接は止めずに知らせるだけ。 */
export default function NoticeBanners({ warningText, sessionEnded, utteranceSaveFailed }: Props) {
  if (!warningText && !utteranceSaveFailed) return null
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 0.75, px: 1.5, pt: 1.5, flexShrink: 0 }}>
      {warningText && (
        <Typography role="status" sx={{ px: 2, py: 0.75, borderRadius: 1, fontSize: 13, fontWeight: 600, color: '#fff', bgcolor: sessionEnded ? CALL.danger : 'warning.dark' }}>
          {warningText}
        </Typography>
      )}
      {/* 発話保存の失敗を面接中に見せる（#1476）。面接は続行できるため止めない */}
      {utteranceSaveFailed && (
        <Typography role="alert" sx={{ px: 2, py: 0.75, borderRadius: 1, fontSize: 13, color: '#fff', bgcolor: CALL.danger }}>
          {UTTERANCE_SAVE_FAILED_MESSAGE}
        </Typography>
      )}
    </Box>
  )
}
