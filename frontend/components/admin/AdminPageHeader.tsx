'use client'

import Link from 'next/link'
import type { ReactNode } from 'react'
import { Box, IconButton, Stack, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'

type AdminPageHeaderProps = {
  title: string
  description?: string
  /** 省略時は戻るボタンなし。通常は `/admin` または一覧パス */
  backHref?: string
  backAriaLabel?: string
  actions?: ReactNode
}

/**
 * 管理画面共通ヘッダー（戻る + タイトル + 説明 + 右アクション）。
 */
export function AdminPageHeader({
  title,
  description,
  backHref,
  backAriaLabel = '戻る',
  actions,
}: AdminPageHeaderProps) {
  return (
    <Stack
      direction={{ xs: 'column', sm: 'row' }}
      alignItems={{ sm: 'flex-start' }}
      justifyContent="space-between"
      spacing={2}
      sx={{ mb: 3 }}
    >
      <Box sx={{ minWidth: 0 }}>
        <Stack direction="row" alignItems="center" spacing={0.5} sx={{ mb: description ? 0.5 : 0 }}>
          {backHref ? (
            // size="small" のままだと34x34でモバイルのタップ領域が足りない（#1481）
            <IconButton
              component={Link}
              href={backHref}
              size="small"
              aria-label={backAriaLabel}
              sx={{ width: 44, height: 44 }}
            >
              <ArrowBackIcon />
            </IconButton>
          ) : null}
          {/* 管理画面のページ見出しは h1（#1479）。見た目は variant="h4" のまま */}
          <Typography variant="h4" component="h1" fontWeight={700} sx={{ letterSpacing: '-0.02em' }}>
            {title}
          </Typography>
        </Stack>
        {description ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ pl: backHref ? { sm: 5 } : 0 }}
          >
            {description}
          </Typography>
        ) : null}
      </Box>
      {actions ? (
        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          flexWrap="wrap"
          useFlexGap
          sx={{ flexShrink: 0 }}
        >
          {actions}
        </Stack>
      ) : null}
    </Stack>
  )
}
