'use client'

import type { ReactNode } from 'react'
import { Box, Stack, Typography, type BoxProps } from '@mui/material'

type AdminPanelProps = Omit<BoxProps, 'title'> & {
  title?: string
  headerRight?: ReactNode
  /** ヘッダー下にボーダーを出す（デフォルト true） */
  divided?: boolean
  children: ReactNode
}

/**
 * 管理画面のコンテンツ枠（角丸ボーダー）。一覧・テーブル・ツールで共通利用。
 */
export function AdminPanel({
  title,
  headerRight,
  divided = true,
  children,
  sx,
  ...rest
}: AdminPanelProps) {
  const sxList = Array.isArray(sx) ? sx : [sx]
  const hasHeader = Boolean(title || headerRight)

  return (
    <Box
      {...rest}
      sx={[
        {
          border: '1px solid',
          borderColor: 'divider',
          borderRadius: '10px',
          overflow: 'hidden',
          bgcolor: 'background.paper',
        },
        ...sxList,
      ]}
    >
      {hasHeader ? (
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          alignItems={{ sm: 'center' }}
          justifyContent="space-between"
          spacing={1.5}
          sx={{
            px: 2.5,
            py: 1.75,
            ...(divided ? { borderBottom: '1px solid', borderColor: 'divider' } : null),
          }}
        >
          {title ? (
            <Typography variant="subtitle1" fontWeight={700}>
              {title}
            </Typography>
          ) : (
            <Box />
          )}
          {headerRight}
        </Stack>
      ) : null}
      {children}
    </Box>
  )
}

type AdminPanelBodyProps = {
  children: ReactNode
  /** デフォルト true。テーブル直置き時は false */
  padded?: boolean
}

export function AdminPanelBody({ children, padded = true }: AdminPanelBodyProps) {
  return <Box sx={padded ? { px: 2.5, py: 2 } : undefined}>{children}</Box>
}
