'use client'

import type { MouseEvent } from 'react'
import { Box, IconButton, Tooltip, Typography } from '@mui/material'
import { Favorite } from '@mui/icons-material'
import type { Company } from '../types'
import { UI } from '@/lib/design/tokens'

export interface FavoritesCompareListProps {
  companies: Company[]
  favoritingId: number | null
  onSelectCompany: (company: Company) => void
  onToggleFavorite: (e: MouseEvent, company: Company) => void
}

/** 求人が紐づいていないマッチは、職種を空のまま断定しない。 */
export function jobTitleLabel(company: Company): string {
  const title = company.jobTitle?.trim()
  return title ? title : '未設定'
}

/**
 * 診断をまたいだお気に入りを、業界・職種・適合度の列で見比べる。
 * 並びは呼び出し側が渡した順のまま。課金などの別基準では並べ替えない。
 */
export default function FavoritesCompareList({
  companies,
  favoritingId,
  onSelectCompany,
  onToggleFavorite,
}: FavoritesCompareListProps) {
  return (
    <Box
      component="table"
      sx={{
        width: '100%',
        borderCollapse: 'collapse',
        '& th, & td': {
          textAlign: 'left',
          verticalAlign: 'baseline',
          borderBottom: `1px solid ${UI.rule}`,
          py: 1.5,
          px: { xs: 0, sm: 1 },
          fontWeight: 400,
        },
        '& th': {
          fontSize: 13,
          color: UI.inkSoft,
          py: 1,
        },
        // 狭い画面では行を縦に積む。横スクロールの表は指で比較しにくい。
        '@media (max-width:599.95px)': {
          '& thead': { display: 'none' },
          '& tr': {
            display: 'block',
            borderBottom: `1px solid ${UI.rule}`,
            py: 1.5,
          },
          '& td': {
            display: 'block',
            borderBottom: 'none',
            py: 0.25,
            px: 0,
          },
          '& td::before': {
            content: 'attr(data-label)',
            display: 'inline-block',
            minWidth: '4.5em',
            marginRight: '0.75em',
            color: UI.inkSoft,
            fontSize: 13,
          },
          '& td[data-label=""]::before': { display: 'none' },
        },
      }}
    >
      <caption style={{ captionSide: 'top', textAlign: 'left', paddingBottom: 8 }}>
        <Typography component="span" sx={{ fontSize: 14, color: UI.inkSoft }}>
          業界・職種・適合度を揃えています。企業名を押すと詳細を開けます。
        </Typography>
      </caption>
      <Box component="thead">
        <Box component="tr">
          <Box component="th" scope="col">企業</Box>
          <Box component="th" scope="col">業界</Box>
          <Box component="th" scope="col">職種</Box>
          <Box component="th" scope="col">適合度</Box>
          <Box component="th" scope="col">
            <Box component="span" sx={{ position: 'absolute', width: 1, height: 1, overflow: 'hidden', clip: 'rect(0 0 0 0)' }}>
              お気に入り
            </Box>
          </Box>
        </Box>
      </Box>
      <Box component="tbody">
        {companies.map((company) => (
          <Box component="tr" key={company.matchId ?? company.id}>
            <Box component="td" data-label="">
              <Typography
                component="button"
                type="button"
                onClick={() => onSelectCompany(company)}
                sx={{
                  p: 0,
                  border: 0,
                  background: 'none',
                  cursor: 'pointer',
                  font: 'inherit',
                  fontSize: { xs: 17, sm: 16 },
                  fontWeight: 700,
                  color: UI.ink,
                  textAlign: 'left',
                  '&:focus-visible': { outline: `3px solid ${UI.mark}`, outlineOffset: 2 },
                }}
              >
                {company.name}
              </Typography>
            </Box>
            <Box component="td" data-label="業界">
              <Typography sx={{ fontSize: 14, color: UI.ink }}>{company.industry}</Typography>
            </Box>
            <Box component="td" data-label="職種">
              <Typography sx={{ fontSize: 14, color: UI.ink }}>{jobTitleLabel(company)}</Typography>
            </Box>
            <Box component="td" data-label="適合度">
              <Typography sx={{ fontSize: 18, fontWeight: 700, color: UI.ink, fontVariantNumeric: 'tabular-nums' }}>
                {company.matchScore}
              </Typography>
            </Box>
            <Box component="td" data-label="">
              <Tooltip title="お気に入りから外す">
                <IconButton
                  size="small"
                  onClick={(e) => onToggleFavorite(e, company)}
                  aria-label={`${company.name}をお気に入りから外す`}
                  disabled={favoritingId === company.matchId}
                  sx={{ color: UI.flag }}
                >
                  <Favorite fontSize="small" />
                </IconButton>
              </Tooltip>
            </Box>
          </Box>
        ))}
      </Box>
    </Box>
  )
}
