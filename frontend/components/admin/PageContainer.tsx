import { Box, type BoxProps } from '@mui/material'

type PageContainerProps = Omit<BoxProps, 'maxWidth'> & {
  maxWidth?: number | string
}

/** 管理画面の標準ページ幅 */
export const ADMIN_PAGE_WIDTH = {
  form: 720,
  standard: 1100,
  wide: 1200,
  /** 相関図など横に広い可視化向け */
  full: 1600,
} as const

export function PageContainer({
  maxWidth = ADMIN_PAGE_WIDTH.standard,
  sx,
  children,
  ...rest
}: PageContainerProps) {
  const sxList = Array.isArray(sx) ? sx : [sx]
  return (
    <Box
      {...rest}
      sx={[
        { p: { xs: 2, sm: 3, md: 4 }, maxWidth, mx: 'auto', width: '100%' },
        ...sxList,
      ]}
    >
      {children}
    </Box>
  )
}
