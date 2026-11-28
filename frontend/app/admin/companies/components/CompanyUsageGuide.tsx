'use client'

import { Box, Stack, Typography } from '@mui/material'

/** 画面上部の「使い方（3ステップ）」。権限によって手順の文言を変える。 */
export function CompanyUsageGuide({ isPlatform }: { isPlatform: boolean }) {
  return (
    <Box
      sx={{
        mb: 2,
        p: 2,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        bgcolor: 'grey.50',
      }}
    >
      <Typography fontWeight={700} sx={{ mb: 1 }}>
        使い方（3ステップ）
      </Typography>
      <Stack spacing={0.5}>
        <Typography variant="body2" color="text.secondary">
          {isPlatform
            ? '1. 企業名で探し、情報が足りなければ「情報を取得」します'
            : '1. 企業名で探し、「会社概要」「技術情報」「関連企業」で内容を確認します'}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {isPlatform
            ? '2. 「会社概要」「技術情報」「関連企業」で内容を確認・修正します'
            : '2. 必要なら内容を修正し、担当校向けの掲載承認を設定します'}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          {isPlatform
            ? '3. 情報がそろった企業を選んで「学生に公開」します（足りない企業は選べません）'
            : '3. 学生への公開はシステム管理者が行います'}
        </Typography>
      </Stack>
    </Box>
  )
}
