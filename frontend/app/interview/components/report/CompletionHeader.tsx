import { Box, Typography } from '@mui/material'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import { UI } from '@/lib/design/tokens'

/** Figma のチェック丸アイコンの大きさ（px） */
const ICON_SIZE = 56

/** 全状態で共通の見出し。「質問8問・14分32秒　企業名／職種」は取れた項目だけ出す */
export default function CompletionHeader({ metaLine }: { metaLine: string | null }) {
  return (
    <Box sx={{ textAlign: 'center' }}>
      <CheckCircleOutlineIcon aria-hidden sx={{ fontSize: ICON_SIZE, color: 'success.main' }} />
      <Typography component="h1" sx={{ fontSize: { xs: 24, sm: 28 }, fontWeight: 700, mt: 1 }}>
        面接おつかれさまでした
      </Typography>
      {metaLine && (
        <Typography variant="body2" sx={{ color: UI.inkSoft, mt: 0.5 }}>{metaLine}</Typography>
      )}
    </Box>
  )
}
