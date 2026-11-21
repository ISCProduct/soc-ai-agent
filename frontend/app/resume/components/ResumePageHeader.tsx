'use client'

import { Box, ButtonBase, Stack, Typography } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import DescriptionIcon from '@mui/icons-material/Description'
import { useRouter } from 'next/navigation'
import { UI } from '@/lib/design/tokens'
import { ResumeStepper } from './ResumeStepper'

type ResumePageHeaderProps = {
  phase: 'input' | 'running' | 'result'
  /** ステッパーの今の位置 */
  step: 1 | 2 | 3
  prefilledCompany: string
  prefilledIndustry: string
  /** 結果画面の見出し下に出す「ファイル名・応募先・区分」 */
  fileName: string
  targetLabel: string
  candidateLabel: string
  /** 結果画面の「別の書類をレビューする」 */
  onReset: () => void
}

export function ResumePageHeader({
  phase,
  step,
  prefilledCompany,
  prefilledIndustry,
  fileName,
  targetLabel,
  candidateLabel,
  onReset,
}: ResumePageHeaderProps) {
  const router = useRouter()
  const result = phase === 'result'

  return (
    <Stack spacing={1.75} sx={{ pt: { xs: 2, md: 3.5 }, pb: { xs: 2, md: 3 } }}>
      {/* 戻るリンクもタップ領域を44px確保する（#1481） */}
      <ButtonBase
        onClick={result ? onReset : () => router.back()}
        sx={{ alignSelf: 'flex-start', gap: 1, minHeight: 44, color: UI.inkSoft, fontSize: 15, borderRadius: 1 }}
      >
        <ArrowBackIcon sx={{ fontSize: 18 }} />
        {result ? '別の書類をレビューする' : '前のページに戻る'}
      </ButtonBase>
      <Typography
        variant="h4"
        component="h1"
        sx={{ fontSize: { xs: '1.5rem', md: '1.75rem' }, lineHeight: 1.35, fontWeight: 700, color: UI.ink }}
      >
        {result ? 'レビュー結果' : '履歴書・ES のレビュー'}
      </Typography>
      {result ? (
        <Stack direction="row" alignItems="flex-start" spacing={1} sx={{ color: UI.inkSoft, fontSize: 15 }}>
          <DescriptionIcon sx={{ fontSize: 18, mt: '3px' }} aria-hidden />
          <Box component="span">{[fileName, targetLabel ? `${targetLabel}向け` : '', candidateLabel].filter(Boolean).join(' ・ ')}</Box>
        </Stack>
      ) : (
        <>
          {phase === 'input' && (
            <Typography sx={{ fontSize: 16, lineHeight: 1.75, color: UI.inkSoft, maxWidth: 760 }}>
              書類を読んで、直したほうがよい点を、書き換え案つきでお伝えします。結果は注釈つきPDFでも受け取れます。
            </Typography>
          )}
          {phase === 'input' && prefilledCompany && (
            <Box sx={{ px: 2, py: 1.5, bgcolor: UI.markTint, borderRadius: '8px', border: `1px solid ${UI.rule}`, maxWidth: 760 }}>
              <Typography sx={{ fontWeight: 700, color: UI.ink }}>
                {prefilledCompany}
                {prefilledIndustry ? `（${prefilledIndustry}）` : ''}向けのレビューです
              </Typography>
              <Typography variant="caption" sx={{ color: UI.inkSoft }}>
                企業の求める人材像を踏まえたアドバイスが反映されます
              </Typography>
            </Box>
          )}
          <ResumeStepper current={step} />
        </>
      )}
    </Stack>
  )
}
