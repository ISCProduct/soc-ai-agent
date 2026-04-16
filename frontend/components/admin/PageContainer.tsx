import { Box, type BoxProps } from '@mui/material'

type PageContainerProps = Omit<BoxProps, 'maxWidth'> & {
  maxWidth?: number | string
}

export function PageContainer({ maxWidth = 1000, sx, children, ...rest }: PageContainerProps) {
  const sxList = Array.isArray(sx) ? sx : [sx]
  return (
    <Box
      {...rest}
      sx={[
        { p: 4, maxWidth, mx: 'auto' },
        ...sxList,
      ]}
    >
      {children}
    </Box>
  )
}

