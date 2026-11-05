import { Box, Skeleton, Stack, TableCell, TableRow } from '@mui/material'
import { visuallyHidden } from '@mui/utils'

/**
 * 一覧・カード群の読み込み中表示（§23）。
 *
 * 中央にスピナーを1つ置くと、読み込み後にレイアウトが跳ねる。
 * 確定後と同じ形の枠を先に出す。読み上げ環境には「読み込み中」を1回だけ伝える。
 */
export function ListSkeleton({ rows = 3, label = '読み込み中' }: { rows?: number; label?: string }) {
  return (
    <Stack spacing={1.5} role="status" aria-label={label} aria-busy="true">
      {Array.from({ length: rows }, (_, i) => (
        <Box key={i} sx={{ p: 2, border: 1, borderColor: 'divider', borderRadius: 2 }}>
          <Skeleton variant="text" width="40%" height={28} />
          <Skeleton variant="text" width="70%" />
          <Skeleton variant="text" width="55%" />
        </Box>
      ))}
    </Stack>
  )
}

/** 表の読み込み中表示。<TableBody> の中に置く。 */
export function TableSkeletonRows({ rows = 5, columns, label = '読み込み中' }: { rows?: number; columns: number; label?: string }) {
  return (
    <>
      {Array.from({ length: rows }, (_, r) => (
        <TableRow key={r} aria-busy="true">
          {Array.from({ length: columns }, (_, c) => (
            <TableCell key={c}>
              {r === 0 && c === 0 ? <Box component="span" role="status" sx={visuallyHidden}>{label}</Box> : null}
              <Skeleton variant="text" />
            </TableCell>
          ))}
        </TableRow>
      ))}
    </>
  )
}
