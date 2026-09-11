import { Box, type BoxProps } from '@mui/material'

export const ADMIN_CARD_BORDER = '1px solid'
export const ADMIN_CARD_BORDER_COLOR = 'divider'

type AdminListCardProps = BoxProps

/** 入れ子リスト行用の軽い枠（パネル内で使う） */
export function AdminListCard({ sx, children, ...rest }: AdminListCardProps) {
  const sxList = Array.isArray(sx) ? sx : [sx]
  return (
    <Box
      {...rest}
      sx={[
        {
          border: ADMIN_CARD_BORDER,
          borderColor: ADMIN_CARD_BORDER_COLOR,
          borderRadius: '8px',
          p: 2,
        },
        ...sxList,
      ]}
    >
      {children}
    </Box>
  )
}
