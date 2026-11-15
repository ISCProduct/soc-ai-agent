import { Box, Button, LinearProgress, Typography } from '@mui/material'
import { alpha } from '@mui/material/styles'
import RefreshIcon from '@mui/icons-material/Refresh'

import { CALL } from '../../callTokens'

interface Props {
  status: string
  errorMessage: string | null
  onReconnect: () => void
}

/** 接続中・エラー時にタイル全体を覆う。通常時は何も出さない。 */
export default function StatusOverlay({ status, errorMessage, onReconnect }: Props) {
  if (status !== 'connecting' && status !== 'error') return null
  return (
    <Box
      sx={{
        position: 'absolute', inset: 0, zIndex: 3, bgcolor: alpha(CALL.stage, 0.88),
        display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 2, px: 3, textAlign: 'center',
      }}
    >
      {status === 'connecting' ? (
        <>
          <Typography role="status" sx={{ color: CALL.ink, fontWeight: 600 }}>接続中...</Typography>
          <LinearProgress sx={{ width: 160 }} />
        </>
      ) : (
        <>
          <Typography role="alert" sx={{ color: CALL.ink }}>{errorMessage}</Typography>
          <Button variant="contained" startIcon={<RefreshIcon />} onClick={onReconnect} sx={{ minHeight: 44, borderRadius: 999, bgcolor: CALL.accent }}>
            再接続する
          </Button>
        </>
      )}
    </Box>
  )
}
