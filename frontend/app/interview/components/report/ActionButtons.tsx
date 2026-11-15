import Link from 'next/link'
import { Box, Button, Stack } from '@mui/material'
import { primaryButtonSx, secondaryButtonSx, textLinkSx } from './reportStyles'

/** 「レポートを見る」の遷移先（レポート本体の見出し id） */
export const REPORT_BODY_ID = 'interview-report-body'
const DISABLED_REASON_ID = 'report-view-disabled-reason'

export type ActionButtonsProps = {
  /** 主ボタン。ready=レポートを見る / retry=もう一度試す / disabled=できるまで無効 */
  primary: { kind: 'view'; enabled: boolean } | { kind: 'retry'; label: string; onClick: () => void } | null
  onPracticeAgain: () => void
}

export default function ActionButtons({ primary, onPracticeAgain }: ActionButtonsProps) {
  return (
    <Stack spacing={1} sx={{ alignItems: 'center' }}>
      <Box sx={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 1.5 }}>
        {primary?.kind === 'view' && (
          <Button
            variant="contained"
            disabled={!primary.enabled}
            href={primary.enabled ? `#${REPORT_BODY_ID}` : undefined}
            aria-describedby={primary.enabled ? undefined : DISABLED_REASON_ID}
            sx={primaryButtonSx}
          >
            レポートを見る
          </Button>
        )}
        {primary?.kind === 'retry' && (
          <Button variant="contained" onClick={primary.onClick} sx={primaryButtonSx}>{primary.label}</Button>
        )}
        <Button variant="outlined" onClick={onPracticeAgain} sx={secondaryButtonSx}>もう一度練習する</Button>
      </Box>
      {/* 無効の理由を支援技術にも伝える（見える説明は進捗カード内にある） */}
      {primary?.kind === 'view' && !primary.enabled && (
        <span id={DISABLED_REASON_ID} style={{ position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)' }}>
          レポートができると押せます
        </span>
      )}
      <Button component={Link} href="/interview/history" variant="text" sx={textLinkSx}>面接履歴へ</Button>
    </Stack>
  )
}
