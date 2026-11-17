'use client'

import type { KeyboardEvent } from 'react'
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
  const move = (e: KeyboardEvent) => {
    const i = RESULTS_TABS.findIndex((t) => t.key === tab)
    const n = RESULTS_TABS.length
    const next = { ArrowRight: (i + 1) % n, ArrowLeft: (i + n - 1) % n, Home: 0, End: n - 1 }[e.key]
    if (next === undefined) return
    e.preventDefault()
    const key = RESULTS_TABS[next].key
    onChange(key)
    document.getElementById(tabId(key))?.focus()
  }
  return (
    <Box sx={{ display: 'flex', flexDirection: { xs: 'column', md: 'row' }, alignItems: { xs: 'flex-start', md: 'center' }, justifyContent: 'space-between', gap: 1.5, mb: 2 }}>
      <Box role="tablist" aria-label="企業の絞り込み" onKeyDown={move} sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
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
              tabIndex={selected ? 0 : -1}
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
