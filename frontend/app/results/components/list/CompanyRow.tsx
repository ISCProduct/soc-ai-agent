'use client'

import type { MouseEvent } from 'react'
import { Box, Button, IconButton, Tooltip, Typography } from '@mui/material'
import { Favorite, FavoriteBorder } from '@mui/icons-material'
import type { Company } from '../../types'
import { leadSentences, UI } from '@/lib/design/tokens'
import { buildMetaLine, isThinEvidence } from './companyRules'
import { CompanyActions } from './CompanyActions'
import { FitScore } from './FitScore'
import { MatchedAxisLine } from './MatchedAxisLine'
import type { CompanyRowContext } from './rowContext'

/** 企業1行。箱は作らず、罫線で区切る親（CompanyList）の中に置く。 */
export function CompanyRow({ company, rank, ctx }: { company: Company; rank: number; ctx: CompanyRowContext }) {
  const { isDesktop } = ctx
  const thin = isThinEvidence(company, ctx.maxAxisCount, ctx.isProvisional)

  const favorite = (
    <Tooltip title={company.isFavorited ? 'お気に入りから外す' : 'お気に入りに入れる'}>
      <span>
        <IconButton
          onClick={(e: MouseEvent) => ctx.onToggleFavorite(e, company)}
          aria-label={`${company.name}をお気に入りに入れる・外す`}
          aria-pressed={company.isFavorited === true}
          disabled={ctx.favoritingId === company.matchId}
          sx={{ color: company.isFavorited ? UI.flag : UI.inkFaint, width: 44, height: 44 }}
        >
          {company.isFavorited ? <Favorite sx={{ fontSize: 22 }} /> : <FavoriteBorder sx={{ fontSize: 22 }} />}
        </IconButton>
      </span>
    </Tooltip>
  )

  return (
    <Box
      component="li"
      sx={{ px: { xs: 2.25, md: 3.5 }, pt: { xs: 2.5, md: 3 }, pb: { xs: 2.25, md: 2.75 }, borderTop: `1px solid ${UI.rule}`, '&:first-of-type': { borderTop: 'none' } }}
    >
      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 1.5 }}>
        <Box sx={{ display: 'flex', alignItems: 'flex-start', gap: { xs: 1.5, md: 1.75 }, minWidth: 0 }}>
          <Typography aria-hidden sx={{ fontSize: 22, lineHeight: '30px', fontWeight: 700, color: UI.inkFaint, width: 28, flexShrink: 0, fontVariantNumeric: 'tabular-nums' }}>
            {rank}
          </Typography>
          <Box sx={{ minWidth: 0 }}>
            <Typography component="h2" sx={{ fontSize: { xs: 18, md: 20 }, lineHeight: '30px', fontWeight: 700 }}>
              <Box
                component="button"
                type="button"
                onClick={() => ctx.onSelectCompany(company)}
                sx={{ font: 'inherit', color: UI.ink, textAlign: 'left', cursor: 'pointer', p: 0, border: 0, background: 'none', '&:focus-visible': { outline: `3px solid ${UI.mark}`, outlineOffset: 2 } }}
              >
                {company.name}
              </Box>
            </Typography>
            <Typography sx={{ fontSize: 14, lineHeight: '22px', color: UI.inkSoft }}>{buildMetaLine(company)}</Typography>
          </Box>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: { xs: 0, md: 1.75 }, flexShrink: 0, mt: { xs: -0.5, md: 0 } }}>
          {favorite}
          {isDesktop && <FitScore score={company.matchScore} />}
        </Box>
      </Box>

      <Box sx={{ pl: { xs: 5, md: 5.25 }, display: 'grid', gap: 1.25, mt: { xs: 0.5, md: 1.25 } }}>
        {!isDesktop && <FitScore score={company.matchScore} />}
        {company.jobMatched && (
          <Typography sx={{ fontSize: 14, lineHeight: '22px', color: UI.inkSoft }}>選んだ職種の募集があります。</Typography>
        )}
        {/* 参考の印は文字で出す（色だけで伝えない）。理由は文頭の「参考:」から続けて読める */}
        {company.isReference && company.fitNote && (
          <Typography sx={{ fontSize: 14, lineHeight: '22px', color: UI.flag }}>{company.fitNote}</Typography>
        )}
        <Typography sx={{ fontSize: { xs: 15, md: 16 }, lineHeight: { xs: 1.85, md: '28px' }, color: UI.ink }}>
          {leadSentences(company.description)}
        </Typography>
        {thin && (
          <Typography sx={{ fontSize: 14, lineHeight: '22px', fontWeight: 700, color: UI.flag }}>
            △ 根拠にできた項目が{company.matchedAxisCount}つです（他の候補より少なめ）
          </Typography>
        )}
        <MatchedAxisLine categoryScores={company.categoryScores} />
        {ctx.onFeedback && company.matchId && (
          <Box role="group" aria-label={`${company.name}の提案への回答`} sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 1 }}>
            <Typography component="span" sx={{ fontSize: 13, color: UI.inkSoft }}>この提案は？</Typography>
            {(['fit', 'unfit'] as const).map((verdict) => (
              <Button
                key={verdict}
                size="small"
                variant={company.feedback === verdict ? 'contained' : 'outlined'}
                aria-pressed={company.feedback === verdict}
                disabled={ctx.feedbackingId === company.matchId}
                onClick={(e) => ctx.onFeedback?.(e, company, verdict)}
                sx={{ minHeight: 44, minWidth: 64 }}
              >
                {verdict === 'fit' ? '合う' : '合わない'}
              </Button>
            ))}
          </Box>
        )}
        <CompanyActions company={company} ctx={ctx} />
      </Box>
    </Box>
  )
}
