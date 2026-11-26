'use client'

import { Button, Stack, Typography } from '@mui/material'
import { FAVORITES_PAGE_SIZE } from '@/lib/favorites-page'

/**
 * お気に入りが100社を超えたとき、次の100社と前の100社を出す。
 * 100社以内なら何も出さない。色だけに頼らず、ボタンの文字で移動先を示す。
 */
export function FavoritesPageNav({
  total,
  offset,
  pageCount,
  onOffsetChange,
}: {
  total: number
  offset: number
  pageCount: number
  onOffsetChange: (next: number) => void
}) {
  const hasPrev = offset > 0
  const hasNext = offset + pageCount < total
  if (!hasPrev && !hasNext) return null
  const from = pageCount === 0 ? 0 : offset + 1
  const to = offset + pageCount
  return (
    <Stack
      component="nav"
      aria-label="お気に入りのページ"
      direction="row"
      spacing={1}
      useFlexGap
      flexWrap="wrap"
      alignItems="center"
      sx={{ mt: 2 }}
    >
      <Typography variant="body2" color="text.secondary">
        全{total}社のうち {from}–{to}社目
      </Typography>
      {hasPrev ? (
        <Button
          variant="outlined"
          onClick={() => onOffsetChange(Math.max(0, offset - FAVORITES_PAGE_SIZE))}
        >
          前の100社
        </Button>
      ) : null}
      {hasNext ? (
        <Button variant="outlined" onClick={() => onOffsetChange(offset + FAVORITES_PAGE_SIZE)}>
          次の100社
        </Button>
      ) : null}
    </Stack>
  )
}
