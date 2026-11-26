'use client'

import { Box, Button, Stack, Typography } from '@mui/material'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { useRouter } from 'next/navigation'

/**
 * - connection: サーバーに接続できなかった（/error）
 * - unexpected: 画面の表示中に例外が出た（app/error.tsx のエラー境界）
 */
export type ServiceErrorKind = 'connection' | 'unexpected'

/**
 * 誰が見るか。app/error.tsx はアプリで唯一のエラー境界で、管理画面・企業ポータルの例外でも出る。
 * 学生以外に「先生に連絡」と案内しても復旧の手段にならないので、文言を分ける（#2213 のレビュー）。
 */
export type ServiceErrorAudience = 'student' | 'staff'

type Props = {
  kind?: ServiceErrorKind
  audience?: ServiceErrorAudience
  /** 「再試行」で呼ぶ処理（エラー境界の retry など）。あれば主のボタンは「再試行」。 */
  onRetry?: () => void
  /**
   * onRetry が無いときに、主のボタン「元の画面に戻る」で戻るページ（/error の from）。
   * 戻るだけで元の処理はやり直さないので、「再試行」とは呼ばない（#2213 のレビュー）。無ければ「トップへ」だけを出す。
   */
  retryHref?: string
}

const COPY: Record<ServiceErrorKind, { label: string; title: string; body: Record<ServiceErrorAudience, string> }> = {
  connection: {
    label: '接続エラー',
    title: '接続できません',
    body: {
      student: 'サーバーに接続できませんでした。しばらくしてから、もう一度お試しください。',
      staff: 'サーバーに接続できませんでした。しばらくしてから、もう一度お試しください。',
    },
  },
  unexpected: {
    label: '表示エラー',
    title: 'うまく表示できません',
    body: {
      student: '画面の表示でうまくいきませんでした。やり直してもだめなときは先生に連絡してください。',
      staff: '画面の表示でうまくいきませんでした。やり直してもだめなときは、時間をおいてからもう一度お試しください。',
    },
  },
}

/**
 * エラー画面（#2213）。ステータスコードや内部のエラー名は出さない。
 * 状態は色だけでなく字形（アイコン）と文字でも示し、本文は role="alert" で読み上げに届ける。
 */
export function ServiceErrorView({ kind = 'connection', audience = 'student', onRetry, retryHref }: Props) {
  const router = useRouter()
  const copy = COPY[kind]
  const backHref = retryHref && retryHref !== '/' ? retryHref : null

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
          <Typography color="text.secondary">{copy.body[audience]}</Typography>
        </Box>
        <Stack direction="row" spacing={1}>
          {onRetry ? (
            <Button variant="contained" onClick={onRetry}>
              再試行
            </Button>
          ) : backHref ? (
            // 一時的なエラー画面を履歴に残さない（戻るでエラー画面に戻らないように。#2213 のレビュー）
            <Button variant="contained" onClick={() => router.replace(backHref)}>
              元の画面に戻る
            </Button>
          ) : null}
          <Button variant={onRetry || backHref ? 'outlined' : 'contained'} onClick={() => router.replace('/')}>
            トップへ
          </Button>
        </Stack>
      </Stack>
    </Box>
  )
}
