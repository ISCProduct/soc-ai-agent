'use client'

import type { MouseEvent } from 'react'
import { Box, IconButton, Tooltip, Typography } from '@mui/material'
import { Favorite } from '@mui/icons-material'
import type { CategoryScores, Company } from '../types'
import { CATEGORY_SCORE_LABELS } from '../utils'
import { UI } from '@/lib/design/tokens'

const AXIS_KEYS = Object.keys(CATEGORY_SCORE_LABELS) as (keyof CategoryScores)[]

/** 軸の値が無いときは、0と決めつけない。 */
export function axisScoreLabel(company: Company, key: keyof CategoryScores): string {
  const value = company.categoryScores?.[key]
  if (typeof value !== 'number' || !Number.isFinite(value)) return '未設定'
  return String(Math.round(value))
}

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
 * 診断をまたいだお気に入りを、業界・職種・総合の適合度と10軸で見比べる。
 * 並びは呼び出し側が渡した順のまま。課金などの別基準では並べ替えない。
 */
export default function FavoritesCompareList({
  companies,
  favoritingId,
  onSelectCompany,
  onToggleFavorite,
}: FavoritesCompareListProps) {
  return (
    <Box sx={{ overflowX: 'auto' }}>
    <Box
      component="table"
      sx={{
        width: '100%',
        minWidth: { sm: 1080 },
        borderCollapse: 'collapse',
        '& th:first-of-type, & td:first-of-type': {
          position: { sm: 'sticky' },
          left: 0,
          backgroundColor: UI.paper,
          zIndex: 1,
        },
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
          // display:none だと見出しが読み上げから消える。見た目だけ隠し、名前は残す。
          '& thead': {
            position: 'absolute',
            width: '1px',
            height: '1px',
            padding: 0,
            margin: '-1px',
            overflow: 'hidden',
            clip: 'rect(0, 0, 0, 0)',
            whiteSpace: 'nowrap',
            border: 0,
          },
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
            minWidth: '9em',
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
          業界・職種・総合の適合度と、10軸の適合度を揃えています。企業名を押すと詳細を開けます。
        </Typography>
      </caption>
      <Box component="thead">
        <Box component="tr">
          <Box component="th" scope="col">企業</Box>
          <Box component="th" scope="col">業界</Box>
          <Box component="th" scope="col">職種</Box>
          <Box component="th" scope="col">適合度</Box>
          {AXIS_KEYS.map((key) => (
            <Box component="th" scope="col" key={key}>{CATEGORY_SCORE_LABELS[key]}</Box>
          ))}
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
            {AXIS_KEYS.map((key) => (
              <Box component="td" data-label={CATEGORY_SCORE_LABELS[key]} key={key}>
                <Typography sx={{ fontSize: 14, color: UI.ink, fontVariantNumeric: 'tabular-nums' }}>
                  {axisScoreLabel(company, key)}
                </Typography>
              </Box>
            ))}
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
    </Box>
  )
}
