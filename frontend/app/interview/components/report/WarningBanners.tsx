import { Box, Button, Typography } from '@mui/material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { UTTERANCE_SAVE_FAILED_MESSAGE } from '../../utteranceSave'
import { MIN_TAP_SIZE, noticeSx, primaryButtonSx } from './reportStyles'

export type WarningBannersProps = {
  /** finishSession 失敗などの画面全体のエラー（#1015） */
  errorMessage: string | null
  finishFailed?: boolean
  onRetryFinish?: () => void
  /** 発話が保存できていない。レポートが欠ける理由として必ず出す（#1476） */
  utteranceSaveFailed?: boolean
  videoSizeWarning: string | null
}

export default function WarningBanners({ errorMessage, finishFailed, onRetryFinish, utteranceSaveFailed, videoSizeWarning }: WarningBannersProps) {
  return (
    <>
      {errorMessage && (
        <Box sx={noticeSx}>
          <ErrorOutlineIcon fontSize="small" />
          <Box sx={{ flex: 1 }}>
            <Typography variant="body2" sx={{ color: 'inherit' }}>{errorMessage}</Typography>
            {finishFailed && onRetryFinish && (
              <Button variant="contained" onClick={onRetryFinish} sx={{ ...primaryButtonSx, mt: 1.5, minHeight: MIN_TAP_SIZE }}>
                再試行
              </Button>
            )}
          </Box>
        </Box>
      )}
      {utteranceSaveFailed && (
        <Box role="alert" sx={noticeSx}>
          <WarningAmberIcon fontSize="small" />
          <Typography variant="body2" sx={{ color: 'inherit' }}>{UTTERANCE_SAVE_FAILED_MESSAGE}</Typography>
        </Box>
      )}
      {videoSizeWarning && (
        <Box sx={noticeSx}>
          <WarningAmberIcon fontSize="small" />
          <Typography variant="body2" sx={{ color: 'inherit' }}>{videoSizeWarning}</Typography>
        </Box>
      )}
    </>
  )
}
