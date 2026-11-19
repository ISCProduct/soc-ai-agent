'use client'

import { Box, Stack, Typography } from '@mui/material'
import WarningAmberIcon from '@mui/icons-material/WarningAmber'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import { UI } from '@/lib/design/tokens'
import { FINDING_SEVERITY_LABEL, type FindingSeverity } from '../utils'

const SEVERITY_COLOR: Record<FindingSeverity, string> = {
  critical: UI.flag,
  warning: UI.ink,
  info: UI.mark,
}

/** 重要度。色だけに頼らず、字形（▲ ! i）とことばを必ず添える。 */
export function SeverityMark({ severity }: { severity: FindingSeverity }) {
  const Icon = severity === 'critical' ? WarningAmberIcon : severity === 'warning' ? ErrorOutlineIcon : InfoOutlinedIcon
  return (
    <Stack direction="row" alignItems="center" spacing={0.75} sx={{ color: SEVERITY_COLOR[severity] }}>
      <Icon sx={{ fontSize: 16 }} aria-hidden />
      <Typography component="span" sx={{ fontSize: 14, lineHeight: '20px', fontWeight: 700, color: 'inherit' }}>
        {FINDING_SEVERITY_LABEL[severity]}
      </Typography>
    </Stack>
  )
}

/** 書類の強調と指摘の一覧で同じ番号を指す丸。選択中は塗りつぶす。 */
export function NoBadge({ no, selected, size = 24, onFlag = false }: { no: number; selected: boolean; size?: number; onFlag?: boolean }) {
  const border = onFlag ? UI.flag : UI.inkFaint
  return (
    <Box
      aria-hidden
      sx={{
        width: size,
        height: size,
        flexShrink: 0,
        borderRadius: '50%',
        display: 'grid',
        placeItems: 'center',
        fontSize: size <= 22 ? 12 : 13,
        fontWeight: 700,
        bgcolor: selected ? UI.mark : UI.field,
        color: selected ? 'primary.contrastText' : onFlag ? UI.flag : UI.inkSoft,
        border: selected ? 'none' : `1.5px solid ${border}`,
      }}
    >
      {no}
    </Box>
  )
}
