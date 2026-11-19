'use client'

import { Box, ButtonBase, CircularProgress, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'

type ResumeRunningCardProps = {
  hasReport: boolean
  fileName: string
  targetLabel: string
  candidateLabel: string
  onCancel: () => void
}

/** レビュー中。待ち時間の目安と、何を出したかの要約、中止の手段を出す。 */
export function ResumeRunningCard({ hasReport, fileName, targetLabel, candidateLabel, onCancel }: ResumeRunningCardProps) {
  const rows = [
    ['書類', fileName],
    ['応募先', targetLabel],
    ['候補者区分', candidateLabel],
  ]
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', pt: 2, pb: { xs: 6, md: 15 } }}>
      <Stack
        role="status"
        spacing={2}
        alignItems="center"
        sx={{
          width: '100%',
          maxWidth: 640,
          pt: { xs: 4, md: 5.5 },
          pb: { xs: 4, md: 5 },
          px: { xs: 2.5, md: 6 },
          bgcolor: UI.field,
          border: `1px solid ${UI.rule}`,
          borderRadius: '8px',
          textAlign: 'center',
        }}
      >
        <CircularProgress size={48} aria-label="読み込み中" sx={{ color: UI.mark }} />
        <Typography sx={{ fontSize: { xs: 20, md: 22 }, lineHeight: '32px', fontWeight: 700, color: UI.ink }}>
          {hasReport ? 'レポートを書いています' : '書類を読み取っています'}
        </Typography>
        {/*
          「通常30〜60秒」は計測した値ではなかったので、目安の数字は出さない（#2217）。
          画面を離れると、useResumePage の後始末（アンマウント時の reviewAbortRef.abort()）で通信を止めるので、
          添削は中断される。そのとおりに書く。
        */}
        <Typography sx={{ fontSize: 16, lineHeight: '26px', color: UI.inkSoft }}>
          しばらくかかります。画面を離れると中断されるので、この画面を開いたままお待ちください。
        </Typography>
        <Stack component="dl" spacing={0.75} sx={{ width: '100%', m: 0, px: 2.5, py: 2, bgcolor: UI.tint, borderRadius: '8px', textAlign: 'left' }}>
          {rows.map(([label, value]) => (
            <Stack key={label} direction="row" spacing={2}>
              <Typography component="dt" sx={{ width: 90, flexShrink: 0, fontSize: 14, color: UI.inkSoft }}>
                {label}
              </Typography>
              <Typography component="dd" sx={{ m: 0, fontSize: 15, fontWeight: 700, color: UI.ink, overflowWrap: 'anywhere' }}>
                {value}
              </Typography>
            </Stack>
          ))}
        </Stack>
        <ButtonBase
          onClick={onCancel}
          sx={{ minHeight: 44, fontSize: 14, color: UI.ink, textDecoration: 'underline', textUnderlinePosition: 'from-font', borderRadius: '2px' }}
        >
          中止して入力に戻る
        </ButtonBase>
      </Stack>
    </Box>
  )
}
