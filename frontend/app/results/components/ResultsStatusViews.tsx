'use client'

import {
  Box,
  Typography,
  Button,
  Stack,
  CircularProgress,
} from '@mui/material'
import { Refresh } from '@mui/icons-material'

type NavigateHandler = () => void

/** 推薦データ読み込み中 */
export function ResultsLoadingView() {
  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
    }}>
      <CircularProgress size={60} />
      <Typography variant="h6" color="text.secondary">
        AIが企業を分析中...
      </Typography>
    </Box>
  )
}

/** empty フラグ時（既存どおり稀にしか立たないが UI を維持） */
export function ResultsEmptyView({ onBackToChat }: { onBackToChat: NavigateHandler }) {
  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
      p: 3,
    }}>
      <Typography variant="h6" color="text.secondary" sx={{ textAlign: 'center' }}>
        データがありません。診断を完了してから数秒待ち、ページを更新してください。
      </Typography>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ width: { xs: '100%', sm: 'auto' } }}>
        <Button
          variant="contained"
          startIcon={<Refresh />}
          onClick={() => window.location.reload()}
        >
          ページを更新
        </Button>
        <Button
          variant="outlined"
          onClick={onBackToChat}
        >
          チャットに戻る
        </Button>
      </Stack>
    </Box>
  )
}

/** API エラー / 推薦空メッセージ表示 */
export function ResultsErrorView({
  error,
  onBack,
  onReset,
}: {
  error: string
  onBack: NavigateHandler
  onReset: NavigateHandler
}) {
  return (
    <Box sx={{
      flex: 1,
      minHeight: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
      p: 3,
    }}>
      <Typography variant="h6" color="error" sx={{ whiteSpace: 'pre-line', textAlign: 'center' }}>
        {error}
      </Typography>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ width: { xs: '100%', sm: 'auto' } }}>
        <Button
          variant="contained"
          startIcon={<Refresh />}
          onClick={() => window.location.reload()}
        >
          ページを更新
        </Button>
        <Button
          variant="outlined"
          onClick={onBack}
        >
          チャットに戻る
        </Button>
        <Button
          variant="outlined"
          color="error"
          onClick={onReset}
        >
          最初からやり直す
        </Button>
      </Stack>
    </Box>
  )
}

/** 企業リストが空（エラーではない） */
export function ResultsNoMatchView({ onReset }: { onReset: NavigateHandler }) {
  return (
    <Box sx={{
      flex: 1,
      minHeight: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
    }}>
      <Typography variant="h6" color="text.secondary">
        適合する企業が見つかりませんでした
      </Typography>
      <Button variant="outlined" onClick={onReset}>
        最初からやり直す
      </Button>
    </Box>
  )
}
