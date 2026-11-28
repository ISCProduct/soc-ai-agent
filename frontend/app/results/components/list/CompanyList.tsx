'use client'

import { Box, Typography } from '@mui/material'
import type { Company } from '../../types'
import { UI } from '@/lib/design/tokens'
import { CompanyRow } from './CompanyRow'
import type { CompanyRowContext } from './rowContext'

/** 罫線で区切った1枚の面。行ごとに箱は作らない。 */
export function CompanyList({ companies, ctx }: { companies: Company[]; ctx: CompanyRowContext }) {
  return (
    <Box
      component="section"
      aria-label="あなたに近い企業"
      sx={{ border: `1px solid ${UI.rule}`, borderRadius: '8px', backgroundColor: UI.field, overflow: 'hidden' }}
    >
      {companies.length === 0 ? (
        <Typography sx={{ p: 3, fontSize: 14, color: UI.inkSoft }}>該当する企業がありません</Typography>
      ) : (
        <Box component="ol" sx={{ listStyle: 'none', m: 0, p: 0 }}>
          {companies.map((company, i) => (
            <CompanyRow key={`${company.id}-${i}`} company={company} rank={i + 1} ctx={ctx} />
          ))}
        </Box>
      )}
    </Box>
  )
}
