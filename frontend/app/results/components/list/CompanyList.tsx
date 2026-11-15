'use client'

import { Box, Typography } from '@mui/material'
import type { Company } from '../../types'
import { UI } from '@/lib/design/tokens'
import { CompanyRow } from './CompanyRow'
import { TAB_PANEL_ID, tabId } from './companyRules'
import type { ResultsTab } from './companyRules'
import type { CompanyRowContext } from './rowContext'

export interface RankedCompany {
  company: Company
  /** 全体の並びでの順位（絞り込んでも変わらない） */
  rank: number
}

/** 罫線で区切った1枚の面。行ごとに箱は作らない。 */
export function CompanyList({ items, tab, ctx }: { items: RankedCompany[]; tab: ResultsTab; ctx: CompanyRowContext }) {
  return (
    <Box
      role="tabpanel"
      id={TAB_PANEL_ID}
      aria-labelledby={tabId(tab)}
      sx={{ border: `1px solid ${UI.rule}`, borderRadius: '8px', backgroundColor: UI.field, overflow: 'hidden' }}
    >
      {items.length === 0 ? (
        <Typography sx={{ p: 3, fontSize: 14, color: UI.inkSoft }}>該当する企業がありません</Typography>
      ) : (
        <Box component="ol" sx={{ listStyle: 'none', m: 0, p: 0 }}>
          {items.map(({ company, rank }) => (
            <CompanyRow key={`${company.id}-${rank}`} company={company} rank={rank} ctx={ctx} />
          ))}
        </Box>
      )}
    </Box>
  )
}
