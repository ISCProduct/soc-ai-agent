import type { ElementType, ReactNode } from 'react'
import { Box, Stack, Typography } from '@mui/material'

type EmptyStateProps = {
  /** 何が無いのかを一文で。「データがありません」ではなく対象を名指しする。 */
  title: string
  /** 次に何をすればよいか。省略可だが、原則として書く。 */
  description?: string
  /** 次の操作。具体的な操作名のボタンを渡す（「OK」「実行」は使わない）。 */
  action?: ReactNode
  /**
   * title を見出し要素として描画する際のタグ（例: "h2"）。
   * 省略時は見出しにしない（既存呼び出し元の見た目・DOM構造を変えないため）。
   * ページの見出し階層に合わせて呼び出し元が指定する。
   */
  titleComponent?: ElementType
  /**
   * 余白を詰める。
   *
   * 既定の余白はページ全体が空のときを想定している。チャットのログのように
   * 既に余白を持つ領域の中へ置くと、本文（挨拶や直前の質問）を画面外へ
   * 押し出してしまう。省略時は従来どおり。
   */
  dense?: boolean
}

/**
 * データが無いときの表示。
 *
 * プロジェクトルール §24 に対応する。空表示を「データがありません」で終わらせると、
 * 利用者は自分の操作が失敗したのか元々無いのか判断できない。
 * 理由と次の操作を必ず示す。
 *
 * 現状この表示は8ファイル以上で各ページに手書きされていて文言も粒度も揃っていない。
 * 新規・改修時はここを使う。
 *
 * 装飾目的のイラストやアイコンは置かない（§27）。読む順序は
 * 「何が無いか」→「どうすればよいか」→「操作」に固定する。
 */
export function EmptyState({
  title,
  description,
  action,
  titleComponent,
  dense = false,
}: EmptyStateProps) {
  return (
    <Box
      sx={{
        py: dense ? { xs: 2, sm: 2.5 } : { xs: 5, sm: 7 },
        px: 2,
        textAlign: 'center',
        // 読み幅を抑える。中央寄せの文は長いと視線の戻りが大きくなる。
        maxWidth: '44ch',
        mx: 'auto',
      }}
    >
      <Stack spacing={1.5} alignItems="center">
        <Typography
          variant="body1"
          fontWeight={700}
          color="text.primary"
          {...(titleComponent ? { component: titleComponent } : {})}
        >
          {title}
        </Typography>
        {description ? (
          <Typography variant="body2" color="text.secondary">
            {description}
          </Typography>
        ) : null}
        {action ? <Box sx={{ pt: 1 }}>{action}</Box> : null}
      </Stack>
    </Box>
  )
}
