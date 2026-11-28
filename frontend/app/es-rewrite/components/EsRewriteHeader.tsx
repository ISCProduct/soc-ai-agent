import { Box, IconButton, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import EditNoteIcon from '@mui/icons-material/EditNote'
import { CONTENT_MAX_WIDTH, UI } from '@/lib/design/tokens'

// 画面上部の帯（タイトルとホームへ戻るボタン）
export default function EsRewriteHeader({ onBack }: { onBack: () => void }) {
  return (
    <Box
      component="header"
      data-testid="es-rewrite-header"
      sx={{ bgcolor: UI.field, borderBottom: `1px solid ${UI.ruleSoft}` }}
    >
      <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', width: '100%', px: { xs: 2, md: 4 }, py: 2 }}>
        <Box data-testid="es-rewrite-header-row" sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box sx={{ color: UI.mark }}><EditNoteIcon sx={{ fontSize: 32 }} /></Box>
            <Box>
              <Typography sx={{ fontWeight: 700, fontSize: { xs: 16, sm: 20 }, color: UI.ink }}>ES添削・リライト</Typography>
              {/*
                「AIがあなたのES文章を添削・リライトします」をやめた。
                誰が添削するかより、何が返ってくるかを書く。
              */}
              <Typography sx={{ fontSize: 12, color: UI.inkFaint }}>書いた文章を読み、直したほうがよい点と書き直し例を返します</Typography>
            </Box>
          </Box>
          <IconButton onClick={onBack} sx={{ bgcolor: UI.tint, color: UI.inkSoft }} aria-label="ホームへ戻る">
            <ArrowBackIcon />
          </IconButton>
        </Box>
      </Box>
    </Box>
  )
}
