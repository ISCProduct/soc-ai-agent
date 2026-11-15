'use client'

import type { KeyboardEvent } from 'react'
import { Box, IconButton, Tooltip, Typography } from '@mui/material'
import { Favorite, FavoriteBorder } from '@mui/icons-material'
import type { Company } from '../../types'
import { leadSentences, UI } from '@/lib/design/tokens'
import { buildMetaLine, isThinEvidence } from './companyRules'
import { CompanyActions } from './CompanyActions'
import { FitScore } from './FitBand'
import { MatchedAxisChips } from './MatchedAxisChips'
import type { CompanyRowContext } from './rowContext'

/** 企業1行。箱は作らず、罫線で区切る親（CompanyList）の中に置く。 */
export function CompanyRow({ company, rank, ctx }: { company: Company; rank: number; ctx: CompanyRowContext }) {
  const { isDesktop } = ctx
  const thin = isThinEvidence(company, ctx.maxAxisCount, ctx.isProvisional)

  const title = (
    <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: { xs: 1.5, md: 2 }, minWidth: 0 }}>
      <Typography aria-hidden sx={{ fontSize: 26, fontWeight: 700, color: UI.inkFaint, width: 30, flexShrink: 0, lineHeight: 1.2, fontVariantNumeric: 'tabular-nums' }}>
        {rank}
      </Typography>
      <Box sx={{ minWidth: 0 }}>
        <Typography component="h2" sx={{ fontSize: { xs: 18, md: 20 }, fontWeight: 700, color: UI.ink, lineHeight: 1.4 }}>
          {company.name}
        </Typography>
        <Typography sx={{ fontSize: 13, color: UI.inkSoft, mt: 0.25 }}>{buildMetaLine(company)}</Typography>
      </Box>
    </Box>
  )
  const favorite = (
    <Tooltip title={company.isFavorited ? 'お気に入りから外す' : 'お気に入りに入れる'}>
      <span>
        <IconButton
          onClick={(e) => ctx.onToggleFavorite(e, company)}
          aria-label="お気に入りの切り替え"
          aria-pressed={company.isFavorited === true}
          disabled={ctx.favoritingId === company.matchId}
          sx={{ color: company.isFavorited ? UI.flag : UI.inkFaint, width: 44, height: 44 }}
        >
          {company.isFavorited ? <Favorite fontSize="small" /> : <FavoriteBorder fontSize="small" />}
        </IconButton>
      </span>
    </Tooltip>
  )
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault()
      ctx.onSelectCompany(company)
    }
  }

  return (
    <Box
      component="li"
      tabIndex={0}
      onClick={() => ctx.onSelectCompany(company)}
      onKeyDown={onKeyDown}
      sx={{
        cursor: 'pointer',
        px: { xs: 2, md: 3 },
        py: { xs: 2, md: 3 },
        borderTop: `1px solid ${UI.ruleSoft}`,
        '&:first-of-type': { borderTop: 'none' },
        '&:hover': { backgroundColor: UI.paper },
        '&:focus-visible': { outline: `3px solid ${UI.mark}`, outlineOffset: -3 },
      }}
    >
      {isDesktop ? (
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 2, mb: 2 }}>
          {title}
          <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: 1, flexShrink: 0 }}>
            {favorite}
            <FitScore score={company.matchScore} band={ctx.band} isDesktop />
          </Box>
        </Box>
      ) : (
        <Box sx={{ mb: 1.5 }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 1 }}>
            {title}
            {favorite}
          </Box>
          <Box sx={{ mt: 1 }}>
            <FitScore score={company.matchScore} band={ctx.band} isDesktop={false} />
          </Box>
        </Box>
      )}
      <Typography sx={{ fontSize: { xs: 14, md: 15 }, lineHeight: { xs: 1.9, md: '28px' }, color: UI.ink, mb: 2 }}>
        {leadSentences(company.description)}
      </Typography>
      <MatchedAxisChips categoryScores={company.categoryScores} thinAxisCount={thin ? (company.matchedAxisCount ?? null) : null} />
      <CompanyActions company={company} ctx={ctx} />
    </Box>
  )
}
