import Link from 'next/link'
import { Box, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { MIN_TAP_SIZE } from './lobbyStyles'

/**
 * リンク風の見た目だけで href も onClick も無い項目を置いていた（#1058）。
 * 押しても何も起きず、Typography なので Tab フォーカスも当たらない。
 * LandingFooter と同じ方針で、実在するページだけを本物のリンクで出す。
 * 利用規約・ヘルプはページが無いので載せない。作ったら足すこと。
 */
export default function LobbyFooter() {
  return (
    <Box component="footer" sx={{ display: 'flex', justifyContent: 'center' }}>
      <Typography
        component={Link}
        href="/privacy"
        variant="body2"
        sx={{ color: UI.inkSoft, minHeight: MIN_TAP_SIZE, display: 'inline-flex', alignItems: 'center', '&:hover': { textDecoration: 'underline' } }}
      >
        プライバシーポリシー
      </Typography>
    </Box>
  )
}
