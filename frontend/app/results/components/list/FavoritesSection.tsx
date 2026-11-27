'use client'

import type { MouseEvent } from 'react'
import { Box, Button, Typography } from '@mui/material'
import type { Company } from '../../types'
import { UI } from '@/lib/design/tokens'
import FavoritesCompareList from '../FavoritesCompareList'
import { FavoritesPageNav } from '@/components/favorites/FavoritesPageNav'

interface Props {
  companies: Company[]
  loading: boolean
  error: string | null
  /** ページを切る前の社数 */
  total: number
  /** 今のページの先頭が何社目か。0 が先頭 */
  offset: number
  /** 省略するとページ送りを出さない */
  onOffsetChange?: (offset: number) => void
  favoritingId: number | null
  onRetry: () => void
  onShowAll: () => void
  onSelectCompany: (company: Company) => void
  onUnfavorite: (e: MouseEvent, company: Company) => void
}

/** 「お気に入りだけ」のときの一覧（#2301）。読み込み中・失敗・0件を出し分ける。 */
export function FavoritesSection({ companies, loading, error, total, offset, onOffsetChange, favoritingId, onRetry, onShowAll, onSelectCompany, onUnfavorite }: Props) {
  if (companies.length === 0) {
    if (loading) {
      return <Typography role="status" sx={{ fontSize: 15, color: UI.inkSoft, py: 3 }}>お気に入りを読み込んでいます</Typography>
    }
    if (error) {
      return (
        <Box sx={{ py: 3 }}>
          <Typography sx={{ fontSize: 16, fontWeight: 700, color: UI.ink, mb: 1 }}>{error}</Typography>
          <Button variant="outlined" onClick={onRetry} sx={{ minHeight: 44 }}>再読み込み</Button>
        </Box>
      )
    }
    // 最後のページの企業を外して空になったときは、前のページを読み直している途中
    if (offset > 0) {
      return <Typography role="status" sx={{ fontSize: 15, color: UI.inkSoft, py: 3 }}>お気に入りを読み込んでいます</Typography>
    }
    return (
      <Box sx={{ py: 3 }}>
        <Typography sx={{ fontSize: 16, fontWeight: 700, color: UI.ink, mb: 1 }}>お気に入りにした企業はまだありません</Typography>
        <Typography sx={{ fontSize: 15, color: UI.inkSoft, maxWidth: '62ch', lineHeight: 1.85, mb: 2 }}>
          すべての企業に戻って、残しておきたい企業のハートを押してください。ここに残ります。
        </Typography>
        <Button variant="outlined" onClick={onShowAll} sx={{ minHeight: 44 }}>すべての企業を見る</Button>
      </Box>
    )
  }
  return (
    <>
      <FavoritesCompareList companies={companies} favoritingId={favoritingId} onSelectCompany={onSelectCompany} onToggleFavorite={onUnfavorite} />
      {onOffsetChange ? (
        <FavoritesPageNav total={total} offset={offset} pageCount={companies.length} onOffsetChange={onOffsetChange} />
      ) : null}
    </>
  )
}
