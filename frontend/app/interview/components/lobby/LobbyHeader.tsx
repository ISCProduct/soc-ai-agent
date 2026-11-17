import { Box, Button, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { UI } from '@/lib/design/tokens'
import { MIN_TAP_SIZE } from './lobbyStyles'

/** 上の細い帯。左に「戻る」、右にサービス名。 */
export default function LobbyHeader({ onBack }: { onBack: () => void }) {
  return (
    <Box
      component="header"
      sx={{
        px: { xs: 1, md: 3 },
        display: 'flex',
        justifyContent: 'space-between',
        alignItems: 'center',
        bgcolor: UI.field,
        borderBottom: `1px solid ${UI.ruleSoft}`,
      }}
    >
      <Button
        onClick={onBack}
        aria-label="前の画面へ戻る"
        startIcon={<ArrowBackIcon />}
        sx={{ minHeight: MIN_TAP_SIZE, color: UI.mark, fontWeight: 600 }}
      >
        戻る
      </Button>
      <Typography sx={{ fontSize: 14, fontWeight: 600, color: UI.inkSoft, pr: 1 }}>AI面接練習</Typography>
    </Box>
  )
}
