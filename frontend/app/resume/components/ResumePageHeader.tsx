'use client'

import { Box, IconButton, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { useRouter } from 'next/navigation'
import { UI } from '@/lib/design/tokens'

type ResumePageHeaderProps = {
  prefilledCompany: string
  prefilledIndustry: string
}

export function ResumePageHeader({ prefilledCompany, prefilledIndustry }: ResumePageHeaderProps) {
  const router = useRouter()

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        {/* size="small" のままだと34x34でモバイルのタップ領域が足りない（#1481） */}
        <IconButton
          onClick={() => router.back()}
          size="small"
          aria-label="前のページに戻る"
          sx={{ bgcolor: UI.tint, color: UI.inkSoft, width: 44, height: 44 }}
        >
          <ArrowBackIcon />
        </IconButton>
        <Typography variant="h4" component="h1" fontWeight="bold" sx={{ fontSize: { xs: '1.4rem', sm: '2.125rem' } }}>
          履歴書・エントリシート レビュー
        </Typography>
      </Box>
      <Typography variant="body1" color="text.secondary" sx={{ mb: prefilledCompany ? 1.5 : 3 }}>
        PDF・DOCX・Google Docs を出すと、AIが指摘と書き直し案を返します。注釈付きPDFも作れます。
      </Typography>
      {prefilledCompany && (
        <Box sx={{ mb: 3, px: 2, py: 1.5, bgcolor: UI.tint, borderRadius: 1, border: `1px solid ${UI.rule}`, borderLeft: `4px solid ${UI.mark}` }}>
          <Typography sx={{ fontWeight: 700, color: UI.ink }}>
            {prefilledCompany}{prefilledIndustry ? `（${prefilledIndustry}）` : ''}向けのレビューです
          </Typography>
          <Typography variant="caption" sx={{ color: UI.inkSoft }}>
            企業の求める人材像を踏まえたアドバイスが反映されます
          </Typography>
        </Box>
      )}
    </>
  )
}
