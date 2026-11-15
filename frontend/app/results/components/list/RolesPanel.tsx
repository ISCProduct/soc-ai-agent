'use client'

import { Box, Typography } from '@mui/material'
import type { SuggestedRole } from '../../types'
import { UI } from '@/lib/design/tokens'
import { panelSx } from './panelStyles'

interface Props {
  jobSuitabilityComment: string
  suggestedRoles: SuggestedRole[]
  /** true なら外枠なし（SP の開閉カード内で使う） */
  bare?: boolean
}

/** 「向いていそうな職種」。コメントも職種も無ければ出さない。 */
export function RolesPanel({ jobSuitabilityComment, suggestedRoles, bare = false }: Props) {
  if (!jobSuitabilityComment && suggestedRoles.length === 0) return null
  return (
    <Box sx={bare ? { mt: 2 } : panelSx}>
      <Typography component="h2" sx={{ fontSize: 17, fontWeight: 700, color: UI.ink, mb: 1 }}>向いていそうな職種</Typography>
      {jobSuitabilityComment && (
        <Typography sx={{ fontSize: 14, lineHeight: 1.85, color: UI.inkSoft, mb: 1 }}>{jobSuitabilityComment}</Typography>
      )}
      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {suggestedRoles.map((role, i) => (
          <Box component="li" key={i} sx={{ py: 1.5, borderTop: i === 0 ? 'none' : `1px solid ${UI.ruleSoft}` }}>
            <Typography sx={{ fontSize: 15, fontWeight: 700, color: UI.ink }}>{role.title}</Typography>
            {role.reason && <Typography sx={{ fontSize: 13, lineHeight: 1.8, color: UI.inkSoft, mt: 0.25 }}>{role.reason}</Typography>}
          </Box>
        ))}
      </Box>
    </Box>
  )
}
