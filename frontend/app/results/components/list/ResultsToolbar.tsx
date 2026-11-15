'use client'

import { Box, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { RESULTS_TABS, TAB_PANEL_ID, tabId } from './companyRules'
import type { ResultsTab } from './companyRules'

interface Props {
  tab: ResultsTab
  counts: Record<ResultsTab, number>
  onChange: (tab: ResultsTab) => void
}

/** 絞り込みタブ（ピル）と並び順の表示。並び順は固定なので選択肢は出さない。 */
export function ResultsToolbar({ tab, counts, onChange }: Props) {
  return (
    <Box sx={{ display: 'flex', flexDirection: { xs: 'column', md: 'row' }, alignItems: { xs: 'flex-start', md: 'center' }, justifyContent: 'space-between', gap: 1.5, mb: 2 }}>
      <Box role="tablist" aria-label="企業の絞り込み" sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
        {RESULTS_TABS.map(({ key, label }) => {
          const selected = key === tab
          return (
            <Box
              key={key}
              component="button"
              type="button"
              role="tab"
              id={tabId(key)}
              aria-selected={selected}
              aria-controls={TAB_PANEL_ID}
              onClick={() => onChange(key)}
              sx={{
                font: 'inherit', fontSize: 14, cursor: 'pointer', px: 2, minHeight: { xs: 44, md: 36 },
                borderRadius: 999, border: `1px solid ${selected ? UI.mark : UI.rule}`,
                backgroundColor: selected ? UI.mark : UI.field, color: selected ? UI.field : UI.ink,
                fontWeight: selected ? 700 : 400,
                '&:focus-visible': { outline: `3px solid ${UI.mark}`, outlineOffset: 2 },
              }}
            >
              {label} {counts[key]}
            </Box>
          )
        })}
      </Box>
      <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>適合度の高い順</Typography>
    </Box>
  )
}
