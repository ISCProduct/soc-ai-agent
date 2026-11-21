'use client'

import { Box, Typography } from '@mui/material'
import type { SuggestedRole } from '../../types'
import { UI } from '@/lib/design/tokens'
import { withStableKeys } from '@/lib/list-keys'
import { panelSx } from './panelStyles'

interface Props {
  jobSuitabilityComment: string
  suggestedRoles: SuggestedRole[]
}

/** 「向いていそうな職種」。職種が無いときだけ、全体のコメントを代わりに出す。どちらも無ければ出さない。 */
export function RolesPanel({ jobSuitabilityComment, suggestedRoles }: Props) {
  if (suggestedRoles.length === 0 && !jobSuitabilityComment) return null
  return (
    <Box component="section" aria-labelledby="roles-title" sx={{ ...panelSx, p: { xs: 2.5, md: 2.75 }, pt: { md: 2.5 } }}>
      <Typography id="roles-title" component="h2" sx={{ fontSize: 17, lineHeight: '24px', fontWeight: 700, color: UI.ink, mb: 1.75 }}>
        向いていそうな職種
      </Typography>
      {suggestedRoles.length === 0 ? (
        <Typography sx={{ fontSize: 14, lineHeight: 1.85, color: UI.inkSoft }}>{jobSuitabilityComment}</Typography>
      ) : (
        <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
          {withStableKeys(suggestedRoles, (r) => r.title).map(({ key, item: role, index: i }) => (
            <Box component="li" key={key} sx={{ pt: i === 0 ? 0 : 1.75, pb: 1.75, '&:last-of-type': { pb: 0 }, borderTop: i === 0 ? 'none' : `1px solid ${UI.ruleSoft}` }}>
              <Typography sx={{ fontSize: 16, lineHeight: '24px', fontWeight: 700, color: UI.ink }}>{role.title}</Typography>
              {role.reason && <Typography sx={{ fontSize: 14, lineHeight: '23px', color: UI.inkSoft, mt: 0.5 }}>{role.reason}</Typography>}
            </Box>
          ))}
        </Box>
      )}
    </Box>
  )
}
