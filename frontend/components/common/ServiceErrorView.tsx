'use client'

import { Box, Button, Stack, Typography } from '@mui/material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { useRouter } from 'next/navigation'

/**
 * - connection: サーバーに接続できなかった（/error）
 * - unexpected: 画面の表示中に例外が出た（app/error.tsx のエラー境界）
 */
export type ServiceErrorKind = 'connection' | 'unexpected'

type Props = {
  kind?: ServiceErrorKind
  /** 「再試行」で呼ぶ処理（エラー境界の reset など）。無ければ retryHref へ進む。 */
  onRetry?: () => void
  /** onRetry が無いときに「再試行」で進むページ。無ければトップ（/）。 */
  retryHref?: string
}

const COPY: Record<ServiceErrorKind, { label: string; title: string; body: string }> = {
  connection: {
    label: '接続エラー',
    title: '接続できません',
    body: 'サーバーに接続できませんでした。しばらくしてから、もう一度お試しください。',
  },
  unexpected: {
    label: '表示エラー',
    title: 'うまく表示できません',
    body: '画面の表示でうまくいきませんでした。やり直してもだめなときは先生に連絡してください。',
  },
}

/**
 * エラー画面（#2213）。ステータスコードや内部のエラー名は出さない。
 * 状態は色だけでなく字形（アイコン）と文字でも示し、本文は role="alert" で読み上げに届ける。
 */
export function ServiceErrorView({ kind = 'connection', onRetry, retryHref }: Props) {
  const router = useRouter()
  const copy = COPY[kind]

  return (
    <Box
      sx={{
        minHeight: '100dvh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'background.default',
        px: 2,
      }}
    >
      <Stack spacing={2} alignItems="center" sx={{ maxWidth: 420, textAlign: 'center' }}>
        <Typography variant="h5" component="h1" fontWeight={700}>
          {copy.title}
        </Typography>
        <Box role="alert" sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, color: 'error.main' }}>
            <ErrorOutlineIcon fontSize="small" aria-hidden />
            <Typography component="span" sx={{ fontSize: 13, fontWeight: 700 }}>
              {copy.label}
            </Typography>
          </Box>
          <Typography color="text.secondary">{copy.body}</Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          <Button variant="contained" onClick={() => (onRetry ? onRetry() : router.push(retryHref ?? '/'))}>
            再試行
          </Button>
          <Button variant="outlined" onClick={() => router.push('/')}>
            トップへ
          </Button>
        </Stack>
      </Stack>
    </Box>
  )
}
