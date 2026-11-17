'use client'

import { useState } from 'react'
import type { MouseEvent } from 'react'
import { Box, Button, Menu, MenuItem, Tooltip } from '@mui/material'
import { ExpandMore } from '@mui/icons-material'
import type { Company } from '../../types'
import { buildEsRewriteQuery, buildInterviewQuery } from '../../utils'
import { getGuestApplicationsButtonProps } from '@/lib/guest-limits'
import { APPLY_CLOSED_LABEL, getApplyClosedButtonProps } from '@/lib/apply-availability'
import { UI } from '@/lib/design/tokens'
import type { CompanyRowContext } from './rowContext'

/** 応募ボタンの文言。応募済み > 受付停止 > 応募中 > 通常 の優先順。 */
export function applyLabel(company: Pick<Company, 'isApplied'>, closed: boolean, applying: boolean): string {
  if (company.isApplied) return '応募済み'
  if (closed) return APPLY_CLOSED_LABEL
  return applying ? '応募中...' : '応募する'
}

const baseSx = { fontSize: 14, fontWeight: 700, borderRadius: '6px', minHeight: { xs: 44, md: 40 }, px: 2.5 } as const

export function CompanyActions({ company, ctx }: { company: Company; ctx: CompanyRowContext }) {
  const [anchor, setAnchor] = useState<HTMLElement | null>(null)
  const guest = getGuestApplicationsButtonProps(ctx.isGuestUser)
  const closed = getApplyClosedButtonProps(ctx.applyEnabled)
  const applying = ctx.applyingId === company.matchId
  const full = !ctx.isDesktop
  const stop = (fn: () => void) => (e: MouseEvent) => { e.stopPropagation(); fn() }

  const practice = (
    <Button variant="contained" onClick={stop(() => ctx.onNavigate(`/interview?${buildInterviewQuery(company)}`))}
      sx={{ ...baseSx, backgroundColor: UI.mark, boxShadow: 'none', width: full ? '100%' : 'auto' }}>
      面接を練習する
    </Button>
  )
  const apply = (
    <Tooltip title={guest.title || (company.isApplied ? '' : closed.title)}
      disableHoverListener={!guest.disabled && (company.isApplied || !closed.disabled)}>
      <Box component="span" onClick={(e: MouseEvent) => e.stopPropagation()} sx={{ display: 'block' }}>
        <Button variant="outlined" fullWidth={full}
          disabled={company.isApplied || applying || guest.disabled || closed.disabled}
          onClick={(e) => ctx.onApply(e, company)}
          sx={{ ...baseSx, color: UI.ink, borderColor: UI.rule, backgroundColor: UI.field }}>
          {applyLabel(company, closed.disabled, applying)}
        </Button>
      </Box>
    </Tooltip>
  )
  const menuId = `company-menu-${company.id}`
  const more = (
    <Button variant={full ? 'outlined' : 'text'} fullWidth={full}
      aria-label={`${company.name}のその他の操作`} aria-haspopup="menu"
      aria-controls={anchor ? menuId : undefined} aria-expanded={anchor ? true : undefined}
      onClick={(e) => { e.stopPropagation(); setAnchor(e.currentTarget) }}
      endIcon={<ExpandMore aria-hidden />}
      sx={{ ...baseSx, color: UI.inkSoft, borderColor: UI.rule, backgroundColor: full ? UI.field : undefined }}>
      その他
    </Button>
  )
  const go = (path: string) => () => { setAnchor(null); ctx.onNavigate(path) }

  return (
    <>
      {full ? (
        <Box sx={{ display: 'grid', gap: 1 }}>
          {practice}
          <Box sx={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 1 }}>{apply}{more}</Box>
        </Box>
      ) : (
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>{practice}{apply}{more}</Box>
      )}
      {/* Menu は Portal だが React のイベントは行へ伝播するため、詳細が開かないよう止める */}
      <Menu id={menuId} anchorEl={anchor} open={Boolean(anchor)} onClose={() => setAnchor(null)} onClick={(e) => e.stopPropagation()}>
        <MenuItem onClick={go(`/correlation-diagram?company_id=${company.id}`)}>関連企業を見る</MenuItem>
        <MenuItem onClick={go(`/es-rewrite?${buildEsRewriteQuery(company)}`)}>ES添削・リライト</MenuItem>
      </Menu>
    </>
  )
}
