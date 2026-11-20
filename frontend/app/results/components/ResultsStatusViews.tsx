'use client'

import {
  Box,
  Typography,
  Button,
  Stack,
} from '@mui/material'
import { ErrorOutline, Refresh } from '@mui/icons-material'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { buildEmptyRecommendationsMessage, emptyRecommendationsAction } from '../utils'

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

/** ページを読み込み直す（テストで差し替えられるように引数で受ける） */
const reloadPage = () => window.location.reload()

/**
 * おすすめ企業が0件（エラーではない。#2220）。何が無いか・なぜか・次にすること（主な操作は1つ）を出す。
 * 診断を終えていないだけの人に「失敗した」ように見せないため、エラーの見た目（赤字・alert）にしない。
 */
export function ResultsEmptyView({
  reason,
  onBackToChat,
  onReload = reloadPage,
}: {
  reason?: string | null
  onBackToChat: NavigateHandler
  onReload?: NavigateHandler
}) {
  const action = emptyRecommendationsAction(reason ?? undefined)
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
      <Typography variant="h6" component="p" sx={{ textAlign: 'center', fontWeight: 700 }}>
        おすすめ企業がまだありません
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center', maxWidth: '44ch' }}>
        {buildEmptyRecommendationsMessage(reason ?? undefined)}
      </Typography>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ width: { xs: '100%', sm: 'auto' } }}>
        {action === 'chat' ? (
          <Button variant="contained" onClick={onBackToChat}>
            診断チャットを続ける
          </Button>
        ) : (
          <>
            <Button variant="contained" startIcon={<Refresh />} onClick={onReload}>
              もう一度読み込む
            </Button>
            <Button variant="text" onClick={onBackToChat}>
              診断チャットに戻る
            </Button>
          </>
        )}
      </Stack>
    </Box>
  )
}

/**
 * おすすめ企業の取得に失敗した（本当のエラーだけ。0件は ResultsEmptyView）。
 * 本文は role="alert" で読み上げに届け、状態は色だけでなく字形（アイコン）でも示す。主な操作は「もう一度読み込む」。
 */
export function ResultsErrorView({
  error,
  onBack,
  onReload = reloadPage,
}: {
  error: string
  onBack: NavigateHandler
  onReload?: NavigateHandler
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
      <Box role="alert" sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'error.main' }}>
        <ErrorOutline aria-hidden />
        <Typography variant="h6" component="p" sx={{ whiteSpace: 'pre-line', textAlign: 'center' }}>
          {error}
        </Typography>
      </Box>
      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} sx={{ width: { xs: '100%', sm: 'auto' } }}>
        <Button variant="contained" startIcon={<Refresh />} onClick={onReload}>
          もう一度読み込む
        </Button>
        <Button variant="text" onClick={onBack}>
          診断チャットに戻る
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
