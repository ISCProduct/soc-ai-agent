'use client'

import Link from 'next/link'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Button,
  CircularProgress,
  LinearProgress,
  Stack,
  Typography,
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { batchProgressPercent, type BatchProgress } from '@/lib/admin/company-batch-progress'
import { pct, type L1Coverage } from '../company-list-model'

type Props = {
  isPlatform: boolean
  busy: boolean
  coverage: L1Coverage | null
  warming: boolean
  warmMessage: string
  batchProgress: BatchProgress | null
  handleSeedL1: () => void
  handleWarmL1: (dryRun: boolean) => void
}

/** 「高度な設定」。マッチング用データの準備・更新と、求人・就職情報への移動。 */
export function AdvancedSettings({
  isPlatform,
  busy,
  coverage,
  warming,
  warmMessage,
  batchProgress,
  handleSeedL1,
  handleWarmL1,
}: Props) {
  return (
    <Accordion
      disableGutters
      elevation={0}
      sx={{
        mt: 2,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        '&:before': { display: 'none' },
        overflow: 'hidden',
      }}
    >
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box>
          <Typography fontWeight={700}>高度な設定（通常は使いません）</Typography>
          <Typography variant="body2" color="text.secondary">
            システム担当向け。マッチング用データの更新や、求人・就職情報への移動
          </Typography>
        </Box>
      </AccordionSummary>
      <AccordionDetails sx={{ pt: 0 }}>
        <Stack spacing={2}>
          {isPlatform && (
          <Box>
            <Typography variant="subtitle2" sx={{ mb: 0.5 }}>
              マッチング用データの更新
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
              公開中の企業について、学生とのマッチングに使う情報を最新にします。
              {coverage
                ? `（公開 ${coverage.published_total} 社 / 会社情報 ${pct(coverage.info_rate)} / マッチング情報 ${pct(coverage.profile_rate)}）`
                : ''}
            </Typography>
            {warmMessage && (
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
                {warmMessage}
              </Typography>
            )}
            {warming && batchProgress && (
              <Box sx={{ mb: 1 }}>
                <LinearProgress
                  variant="determinate"
                  value={batchProgressPercent(batchProgress)}
                  sx={{ height: 6, borderRadius: 3 }}
                />
                <Typography variant="caption" color="warning.main" sx={{ display: 'block', mt: 0.5 }}>
                  更新中はページを更新・移動しないでください。途中結果の表示が消えます。
                </Typography>
              </Box>
            )}
            <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
              <Button variant="text" size="small" disabled={busy} onClick={() => handleSeedL1()}>
                初期データを用意
              </Button>
              <Button variant="outlined" size="small" disabled={busy} onClick={() => handleWarmL1(true)}>
                対象を確認
              </Button>
              <Button
                variant="contained"
                size="small"
                disabled={busy}
                onClick={() => handleWarmL1(false)}
                startIcon={warming ? <CircularProgress size={14} color="inherit" /> : undefined}
                disableElevation
              >
                マッチング情報を更新
              </Button>
            </Stack>
          </Box>
          )}

          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button component={Link} href="/admin/job-positions" size="small" variant="outlined">
              求人一覧へ
            </Button>
            <Button component={Link} href="/admin/graduate-employments" size="small" variant="outlined">
              就職情報へ
            </Button>
          </Stack>
        </Stack>
      </AccordionDetails>
    </Accordion>
  )
}
