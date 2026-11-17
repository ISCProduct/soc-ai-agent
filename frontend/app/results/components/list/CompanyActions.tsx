'use client'

import { useState } from 'react'
import { Box, Button, Menu, MenuItem, Tooltip } from '@mui/material'
import { ArrowForward, ExpandMore } from '@mui/icons-material'
import type { Company } from '../../types'
import { buildEsRewriteQuery, buildInterviewQuery } from '../../utils'
import { getGuestApplicationsButtonProps } from '@/lib/guest-limits'
import { APPLY_CLOSED_LABEL, getApplyClosedButtonProps } from '@/lib/apply-availability'
import { UI } from '@/lib/design/tokens'
import type { CompanyRowContext } from './rowContext'

/** 応募の文言。応募済み > 受付停止 > 応募中 > 通常 の優先順。 */
export function applyLabel(company: Pick<Company, 'isApplied'>, closed: boolean, applying: boolean): string {
  if (company.isApplied) return '応募済み'
  if (closed) return APPLY_CLOSED_LABEL
  return applying ? '応募中...' : '応募する'
}

/** 主操作以外は下線つきのリンクにする（Figma: 診断結果 改善案 v4）。タップ領域は44px確保する。 */
const linkSx = {
  color: UI.ink,
  fontSize: 15,
  fontWeight: 400,
  textDecoration: 'underline',
  textUnderlinePosition: 'from-font',
  minHeight: 44,
  minWidth: 44,
  px: 0.5,
  borderRadius: '4px',
  '&:hover': { textDecoration: 'underline', backgroundColor: 'transparent' },
  '&.Mui-disabled': { color: UI.inkFaint },
} as const

export function CompanyActions({ company, ctx }: { company: Company; ctx: CompanyRowContext }) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const guest = getGuestApplicationsButtonProps(ctx.isGuestUser)
  const closed = getApplyClosedButtonProps(ctx.applyEnabled)
  const applying = ctx.applyingId === company.matchId
  const menuId = `company-menu-${company.id}`
  const go = (path: string) => () => { setAnchor(null); ctx.onNavigate(path) }

  return (
    <Box sx={{ display: 'flex', flexDirection: { xs: 'column', md: 'row' }, alignItems: { xs: 'stretch', md: 'center' }, columnGap: 3, rowGap: 0.5, pt: 0.75 }}>
      <Button
        variant="contained"
        endIcon={<ArrowForward aria-hidden sx={{ fontSize: 16 }} />}
        onClick={() => ctx.onSelectCompany(company)}
        aria-label={`${company.name}の詳細を見る`}
        sx={{ backgroundColor: UI.mark, boxShadow: 'none', fontSize: 15, fontWeight: 700, minHeight: 44, px: 2.25, borderRadius: '6px', whiteSpace: 'nowrap' }}
      >
        詳しく見る
      </Button>
      <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', columnGap: 3, justifyContent: { xs: 'flex-start', md: 'flex-start' } }}>
        <Button variant="text" onClick={() => ctx.onNavigate(`/interview?${buildInterviewQuery(company)}`)} sx={linkSx}>
          面接を練習する
        </Button>
        <Tooltip title={guest.title || (company.isApplied ? '' : closed.title)}
          disableHoverListener={!guest.disabled && (company.isApplied || !closed.disabled)}>
          <span>
            <Button variant="text" disabled={company.isApplied || applying || guest.disabled || closed.disabled}
              onClick={(e) => ctx.onApply(e, company)} sx={linkSx}>
              {applyLabel(company, closed.disabled, applying)}
            </Button>
          </span>
        </Tooltip>
        <Button variant="text" aria-label={`${company.name}のその他の操作`} aria-haspopup="menu"
          aria-controls={anchor ? menuId : undefined} aria-expanded={anchor ? true : undefined}
          onClick={(e) => setAnchor(e.currentTarget)} endIcon={<ExpandMore aria-hidden />} sx={linkSx}>
          その他
        </Button>
      </Box>
      <Menu id={menuId} anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)}>
        <MenuItem onClick={go(`/correlation-diagram?company_id=${company.id}`)}>関連企業を見る</MenuItem>
        <MenuItem onClick={go(`/es-rewrite?${buildEsRewriteQuery(company)}`)}>ES添削・リライト</MenuItem>
      </Menu>
    </Box>
  )
}
