'use client'

import { Box, Button, CircularProgress, LinearProgress, Stack, Typography } from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import { batchProgressPercent, emptyBatchProgress, formatBatchProgressLabel, type BatchProgress } from '@/lib/admin/company-batch-progress'

type Props = {
  busy: boolean
  missingBatchLoading: boolean
  batchProgress: BatchProgress | null
  handleFetchMissingBatch: (dryRun: boolean) => void
}

/** 足りない情報のまとめて取得（システム管理者向け）。進捗バーを出す。 */
export function MissingBatchPanel({ busy, missingBatchLoading, batchProgress, handleFetchMissingBatch }: Props) {
  return (
    <Box
      sx={{
        mb: 2,
        p: 2.5,
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 1,
        bgcolor: 'background.paper',
      }}
    >
      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        spacing={2}
        alignItems={{ sm: 'center' }}
        justifyContent="space-between"
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography fontWeight={700} sx={{ mb: 0.5 }}>
            足りない情報をまとめて取得
          </Typography>
          <Typography variant="body2" color="text.secondary">
            まだそろっていない会社概要・技術情報・関連企業を自動で集めます。
            処理した社数と内訳が進捗バーに出ます。足りなければ続きから再度実行できます。
            取得後は必ず内容を確認してから公開してください。
          </Typography>
          {missingBatchLoading && batchProgress && (
            <Box sx={{ mt: 1.5 }}>
              <LinearProgress
                variant="determinate"
                value={batchProgressPercent(batchProgress)}
                sx={{ height: 8, borderRadius: 4 }}
              />
              <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.75 }}>
                {formatBatchProgressLabel(batchProgress, true)}
              </Typography>
              <Typography variant="caption" color="warning.main" sx={{ display: 'block', mt: 0.25 }}>
                取得中はページを更新・移動しないでください。途中結果の表示が消えます。
              </Typography>
            </Box>
          )}
        </Box>
        <Stack direction="row" spacing={1} flexShrink={0}>
          <Button variant="outlined" disabled={busy} onClick={() => handleFetchMissingBatch(true)}>
            対象を確認
          </Button>
          <Button
            variant="contained"
            color="secondary"
            disabled={busy}
            onClick={() => handleFetchMissingBatch(false)}
            startIcon={missingBatchLoading ? <CircularProgress size={16} color="inherit" /> : <RefreshIcon />}
            disableElevation
          >
            {missingBatchLoading ? `取得中 ${batchProgressPercent(batchProgress ?? emptyBatchProgress())}%` : 'まとめて取得'}
          </Button>
        </Stack>
      </Stack>
    </Box>
  )
}
