'use client'

import {
  Box,
  Typography,
  Button,
  Stack,
} from '@mui/material'
import { Refresh } from '@mui/icons-material'
import { ListSkeleton } from '@/components/common/ListSkeleton'

type NavigateHandler = () => void

/** 推薦データ読み込み中。確定後と同じ一覧の形を先に出す（§23）。 */
export function ResultsLoadingView() {
  return (
    <Box sx={{ maxWidth: 960, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="body1" color="text.secondary" sx={{ mb: 2 }}>
        希望条件との一致度を計算しています
      </Typography>
      <ListSkeleton rows={4} label="おすすめ企業を読み込み中" />
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
      <Typography variant="h6" component="p" sx={{ textAlign: 'center', fontWeight: 700 }}>
        まだおすすめ企業を計算できていません
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center', maxWidth: '44ch' }}>
        診断チャットを最後まで進めると、希望条件との一致度が高い企業がここに並びます。
        診断を終えた直後は数秒かかるので、ページを更新してください。
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
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
      py: 6,
      px: 3,
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
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: 2,
      py: 6,
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
