import { Box, Button, Typography } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { UI } from '@/lib/design/tokens'
import { MIN_TAP_SIZE } from './lobbyStyles'

/**
 * カメラ／マイクの許可エラー。プレビュー枠の上に重ねて出す。
 * 原因ごとの直し方（ブラウザ設定・OS 設定）を一緒に見せ、再試行で取り直せるようにする。
 * 赤などの色に頼らず、アイコンと文言でエラーだと分かるようにしている。
 */
export default function PermissionError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <Box
      role="alert"
      sx={{ position: 'absolute', inset: 0, p: 3, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', textAlign: 'center' }}
    >
      <ErrorOutlineIcon sx={{ color: UI.nightInk, mb: 0.5 }} />
      <Typography sx={{ color: UI.nightInk, mb: 1.5, fontSize: 14, fontWeight: 600 }}>{message}</Typography>
      <Box sx={{ mb: 1.5, textAlign: 'left', bgcolor: UI.nightPanel, borderRadius: 1, p: 1.5, width: '100%', maxWidth: 360 }}>
        <Typography sx={{ color: UI.nightInk, fontSize: 12, mb: 0.5 }}>【ブラウザの許可】</Typography>
        <Typography sx={{ color: UI.nightSoft, fontSize: 12, lineHeight: 1.8, mb: 1 }}>
          アドレスバー左端の鍵アイコン → カメラ・マイクを「許可」
        </Typography>
        <Typography sx={{ color: UI.nightInk, fontSize: 12, mb: 0.5 }}>【Mac のシステム設定】</Typography>
        <Typography sx={{ color: UI.nightSoft, fontSize: 12, lineHeight: 1.8 }}>
          システム設定 → プライバシーとセキュリティ
          <br />→ カメラ / マイク → 使用中のブラウザをオン
        </Typography>
      </Box>
      <Button startIcon={<RefreshIcon />} onClick={onRetry} sx={{ minHeight: MIN_TAP_SIZE, color: UI.nightInk, textDecoration: 'underline' }}>
        再試行
      </Button>
    </Box>
  )
}
