import { Box, type BoxProps } from '@mui/material'

export const ADMIN_CARD_BORDER = '1px solid #e0e0e0'

type AdminListCardProps = BoxProps

export function AdminListCard({ sx, children, ...rest }: AdminListCardProps) {
  const sxList = Array.isArray(sx) ? sx : [sx]
  return (
    <Box
      {...rest}
      sx={[
        { border: ADMIN_CARD_BORDER, borderRadius: 1, p: 2 },
        ...sxList,
      ]}
    >
      {children}
    </Box>
  )
}

