'use client'

import { Box, IconButton, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import type { AppRouterInstance } from 'next/dist/shared/lib/app-router-context.shared-runtime'

type ResumePageHeaderProps = {
  router: AppRouterInstance
  prefilledCompany: string
  prefilledIndustry: string
}

export function ResumePageHeader({ router, prefilledCompany, prefilledIndustry }: ResumePageHeaderProps) {
  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <IconButton
          onClick={() => router.back()}
          size="small"
          aria-label="前のページに戻る"
          sx={{ bgcolor: '#f1f5f9', color: '#475569' }}
        >
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h4" fontWeight="bold" sx={{ fontSize: { xs: '1.4rem', sm: '2.125rem' } }}>
          履歴書・エントリシート レビュー
        </Typography>
      </Box>
      <Typography variant="body1" color="text.secondary" sx={{ mb: prefilledCompany ? 1.5 : 3 }}>
        PDF/DOCX/Google Docsをアップロードして、注釈付きPDFを生成します。
      </Typography>
      {prefilledCompany && (
        <Box sx={{ mb: 3, px: 2, py: 1.5, bgcolor: '#e8f5e9', borderRadius: 1, border: '1px solid #a5d6a7' }}>
          <Typography sx={{ fontSize: 14, color: '#2e7d32', fontWeight: 600 }}>
            {prefilledCompany}{prefilledIndustry ? `（${prefilledIndustry}）` : ''}向けに最適化されたフィードバックを提供します
          </Typography>
          <Typography sx={{ fontSize: 12, color: '#388e3c', mt: 0.5 }}>
            企業の求める人材像を踏まえたアドバイスが反映されます
          </Typography>
        </Box>
      )}
    </>
  )
}
