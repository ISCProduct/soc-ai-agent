'use client'

import { Alert, AlertTitle, Box, Button, Stack, Typography } from '@mui/material'

type ErrorStateProps = {
  /** 何が起きたか。対象を名指しする。「エラーが発生しました」では使えない。 */
  title: string
  /** 利用者が何をすればよいか。 */
  description?: string
  /** 再試行。渡したときだけボタンを出す。 */
  onRetry?: () => void
  retryLabel?: string
}

/**
 * 取得や保存に失敗したときの表示。
 *
 * プロジェクトルール §25 に対応する。既存の `ErrorAlert` はメッセージを
 * そのまま MUI Alert に流すだけで、「次に何をすればよいか」が無い。
 * 一行のエラー文字列を出すだけなら `ErrorAlert` のままでよく、
 * 画面の主要部分が描けなかったときはこちらを使う。
 *
 * 文面の規約:
 * - 謝らない。何が起きたかと次の操作だけを書く
 * - 内部用語を出さない（ステータスコード・例外名・テーブル名）
 * - 再試行で直らない種類のときは `onRetry` を渡さない
 */
export function ErrorState({ title, description, onRetry, retryLabel = '再読み込み' }: ErrorStateProps) {
  return (
    <Box sx={{ py: { xs: 3, sm: 4 } }}>
      <Alert
        severity="error"
        variant="outlined"
        // role="alert" は読み上げを即座に割り込ませる。描画時点で既に起きた失敗なので
        // status ではなく alert が適切。
        role="alert"
        sx={{ maxWidth: '60ch' }}
      >
        <AlertTitle sx={{ fontWeight: 700 }}>{title}</AlertTitle>
        <Stack spacing={1.5} alignItems="flex-start">
          {description ? <Typography variant="body2">{description}</Typography> : null}
          {onRetry ? (
            <Button onClick={onRetry} variant="outlined" color="inherit" size="small">
              {retryLabel}
            </Button>
          ) : null}
        </Stack>
      </Alert>
    </Box>
  )
}
