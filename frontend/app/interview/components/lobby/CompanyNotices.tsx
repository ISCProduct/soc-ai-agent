import { Box, Stack, Typography } from '@mui/material'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import { UI } from '@/lib/design/tokens'
import type { InterviewCompany } from '../../types'
import { cardSx } from './lobbyStyles'

/** マッチング結果から来たときの案内バナー。 */
export function MatchingBanner({ company }: { company: InterviewCompany | null }) {
  return (
    <Box sx={cardSx}>
      <Typography sx={{ fontSize: 13, color: UI.ink, fontWeight: 600 }}>
        {company?.name}（{company?.industry || '業種未設定'}）向けの面接練習を始めます
      </Typography>
    </Box>
  )
}

/**
 * 企業管理に未登録の企業を選んだときの警告。
 * カスタム質問・深掘りが効かないことを、アイコンと文言で（色に頼らず）伝える。
 */
export function UnregisteredWarning() {
  return (
    <Box role="note" sx={{ ...cardSx, display: 'flex', gap: 1, borderColor: UI.flag }}>
      <WarningAmberIcon sx={{ color: UI.flag, fontSize: 18, mt: 0.2, flexShrink: 0 }} />
      <Box>
        <Typography sx={{ fontSize: 13, color: UI.flag, fontWeight: 700 }}>カスタム質問・深掘りは無効です</Typography>
        <Typography sx={{ fontSize: 12, color: UI.inkSoft, mt: 0.5, lineHeight: 1.5 }}>
          企業管理に未登録のため、一般的な面接練習のみ行えます。登録企業を選ぶとカスタム質問が有効になります。
        </Typography>
      </Box>
    </Box>
  )
}

/** 企業情報（文化・働き方・福利厚生）。値がある項目だけ出す。 */
export function CompanyInfo({ company }: { company: InterviewCompany }) {
  const items = [
    { label: '企業文化', text: company.culture },
    { label: '働き方', text: company.work_style },
    { label: '福利厚生', text: company.welfare_details },
  ].filter((item) => item.text)
  if (items.length === 0) return null
  return (
    <Box>
      <Typography sx={{ fontSize: 13, fontWeight: 700, color: UI.inkSoft, mb: 1 }}>企業情報</Typography>
      <Stack spacing={1}>
        {items.map((item) => (
          <Box key={item.label} sx={cardSx}>
            <Typography sx={{ fontSize: 11, fontWeight: 700, color: UI.mark, mb: 0.3 }}>{item.label}</Typography>
            <Typography sx={{ fontSize: 13, color: UI.ink, lineHeight: 1.5 }}>{item.text}</Typography>
          </Box>
        ))}
      </Stack>
    </Box>
  )
}
