'use client'

import { useEffect, useRef } from 'react'
import { Box, Button, ButtonBase, Stack, Typography } from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import VisibilityOutlinedIcon from '@mui/icons-material/VisibilityOutlined'
import { UI } from '@/lib/design/tokens'
import { FINDING_SEVERITY_LABEL, type Finding, type FindingSeverity } from '../utils'
import { NoBadge, SeverityMark } from './ResumeFindingParts'

export type FindingFilter = 'all' | FindingSeverity

type ResumeFindingsPaneProps = {
  /** 絞り込み後の指摘 */
  findings: Finding[]
  counts: Record<FindingSeverity, number>
  totalCount: number
  filter: FindingFilter
  onFilterChange: (filter: FindingFilter) => void
  selectedNo: number | null
  /** 書類で今開いているページ */
  currentPage: number
  onSelect: (no: number) => void
  /** PC: 前後の指摘へ移る。スマホでは出さない */
  showNav: boolean
  onPrev: () => void
  onNext: () => void
  /** スマホ: 各指摘に「書類で見る」を出す */
  onShowInDocument?: (no: number) => void
}

const pillBase = {
  px: 1.5,
  py: 0.75,
  minHeight: { xs: 44, md: 36 },
  borderRadius: '16px',
  fontSize: 14,
  lineHeight: '20px',
  whiteSpace: 'nowrap',
} as const

const navBtnSx = {
  minHeight: { xs: 44, md: 36 },
  px: 1.25,
  py: 0.75,
  gap: 0.5,
  fontSize: 13,
  fontWeight: 700,
  color: UI.ink,
  border: `1.5px solid ${UI.inkFaint}`,
  borderRadius: '6px',
  bgcolor: UI.field,
  '&.Mui-disabled': { color: UI.inkFaint, borderColor: UI.rule },
} as const

/** 指摘の一覧。選んだ指摘は「いまの文章」と「書き換え案」まで開き、ほかは1行にたたむ。 */
export function ResumeFindingsPane({
  findings,
  counts,
  totalCount,
  filter,
  onFilterChange,
  selectedNo,
  currentPage,
  onSelect,
  showNav,
  onPrev,
  onNext,
  onShowInDocument,
}: ResumeFindingsPaneProps) {
  const selectedRef = useRef<HTMLLIElement | null>(null)
  useEffect(() => {
    selectedRef.current?.scrollIntoView({ block: 'nearest' })
  }, [selectedNo])

  const index = findings.findIndex((f) => f.no === selectedNo)
  const pills: { key: FindingFilter; label: string; n: number }[] = [
    { key: 'all', label: 'すべて', n: totalCount },
    { key: 'critical', label: FINDING_SEVERITY_LABEL.critical, n: counts.critical },
    { key: 'warning', label: FINDING_SEVERITY_LABEL.warning, n: counts.warning },
    { key: 'info', label: FINDING_SEVERITY_LABEL.info, n: counts.info },
  ]

  const pages = Array.from(new Set(findings.map((f) => f.page)))

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minWidth: 0,
        height: { md: 800 },
        bgcolor: UI.field,
        border: `1px solid ${UI.rule}`,
        borderRadius: '8px',
        overflow: 'hidden',
      }}
    >
      <Stack spacing={1.5} sx={{ px: 2.5, pt: 2, pb: 1.75, borderBottom: `1px solid ${UI.ruleSoft}`, flexShrink: 0 }}>
        <Stack direction="row" spacing={1} role="group" aria-label="重要度で絞り込む" sx={{ flexWrap: 'wrap', rowGap: 1 }}>
          {pills.map((p) => {
            const active = filter === p.key
            return (
              <ButtonBase
                key={p.key}
                aria-pressed={active}
                onClick={() => onFilterChange(p.key)}
                sx={{
                  ...pillBase,
                  fontWeight: active ? 700 : 400,
                  bgcolor: active ? UI.mark : UI.field,
                  color: active ? 'primary.contrastText' : UI.inkSoft,
                  border: active ? `1px solid ${UI.mark}` : `1px solid ${UI.rule}`,
                }}
              >
                {p.label} {p.n}
              </ButtonBase>
            )
          })}
        </Stack>
        {showNav && (
          <Stack direction="row" alignItems="center" justifyContent="space-between">
            <Typography sx={{ fontSize: 14, fontWeight: 700, color: UI.ink }} aria-live="polite">
              {findings.length === 0 ? '該当する指摘はありません' : `指摘 ${index + 1} / ${findings.length} を表示中`}
            </Typography>
            <Stack direction="row" spacing={1}>
              <Button onClick={onPrev} disabled={index <= 0} sx={navBtnSx} startIcon={<ChevronLeftIcon sx={{ fontSize: 16 }} />}>
                前の指摘
              </Button>
              <Button onClick={onNext} disabled={index < 0 || index >= findings.length - 1} sx={navBtnSx} endIcon={<ChevronRightIcon sx={{ fontSize: 16 }} />}>
                次の指摘
              </Button>
            </Stack>
          </Stack>
        )}
      </Stack>

      <Box component="ol" sx={{ flex: 1, overflow: 'auto', listStyle: 'none', m: 0, p: 0 }}>
        {pages.map((page) => {
          const group = findings.filter((f) => f.page === page)
          const here = page === currentPage
          return (
            <Box component="li" key={page} sx={{ listStyle: 'none' }}>
              <Stack
                direction="row"
                alignItems="center"
                spacing={1}
                sx={{ px: 2.5, py: 1, bgcolor: here ? UI.markTint : UI.tint, color: here ? UI.mark : UI.inkSoft }}
              >
                <Typography sx={{ fontSize: 14, fontWeight: 700, color: 'inherit' }}>{page}ページ目</Typography>
                {here && <Typography sx={{ fontSize: 13, fontWeight: 700, color: 'inherit' }}>・ いま表示中</Typography>}
                <Typography sx={{ fontSize: 13, color: UI.inkSoft }}>{group.length}件</Typography>
              </Stack>
              <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0 }}>
                {group.map((f) => {
                  const selected = f.no === selectedNo
                  return (
                    <Box
                      component="li"
                      key={f.no}
                      ref={selected ? selectedRef : undefined}
                      sx={{
                        px: 2.5,
                        py: 1.75,
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 1.25,
                        bgcolor: UI.field,
                        border: selected ? `2px solid ${UI.mark}` : 'none',
                        borderBottom: selected ? `2px solid ${UI.mark}` : `1px solid ${UI.ruleSoft}`,
                        ...(selected ? { px: 'calc(20px - 2px)', py: 'calc(14px - 2px)' } : {}),
                      }}
                    >
                      <Stack direction="row" alignItems="center" spacing={1.25}>
                        <NoBadge no={f.no} selected={selected} />
                        <SeverityMark severity={f.severity} />
                        {f.source === 'rule' && (
                          <Typography sx={{ fontSize: 12, fontWeight: 700, color: UI.inkSoft }}>ルールで検出</Typography>
                        )}
                        <Box sx={{ flex: 1 }} />
                        {onShowInDocument && (
                          <Button
                            onClick={() => onShowInDocument(f.no)}
                            startIcon={<VisibilityOutlinedIcon sx={{ fontSize: 16 }} />}
                            sx={{ ...navBtnSx, fontSize: 13 }}
                          >
                            書類で見る
                          </Button>
                        )}
                        {!selected && !onShowInDocument && <ExpandMoreIcon sx={{ fontSize: 18, color: UI.inkSoft }} aria-hidden />}
                      </Stack>
                      <ButtonBase
                        onClick={() => onSelect(f.no)}
                        aria-expanded={selected}
                        sx={{ display: 'block', width: '100%', textAlign: 'left', fontSize: 15, lineHeight: '24px', fontWeight: 700, color: UI.ink, borderRadius: '4px' }}
                      >
                        {f.message}
                      </ButtonBase>
                      {selected && (
                        <>
                          {f.quote && (
                            <Box sx={{ px: 1.75, py: 1.25, bgcolor: UI.tint, borderRadius: '6px' }}>
                              <Typography sx={{ fontSize: 12, lineHeight: '18px', fontWeight: 700, color: UI.inkSoft }}>いまの文章</Typography>
                              <Typography sx={{ fontSize: 15, lineHeight: '24px', color: UI.ink, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
                                {f.quote}
                              </Typography>
                            </Box>
                          )}
                          {f.withheld ? (
                            <Typography sx={{ fontSize: 13, color: UI.inkSoft }}>
                              この行は読み取りの信頼度が低いため、書き換え案は出していません。元の書類を見て、ご自身で書き直してください。
                            </Typography>
                          ) : f.rewrite ? (
                            <>
                              <Box sx={{ px: 1.75, py: 1.25, bgcolor: UI.okTint, borderRadius: '6px' }}>
                                <Typography sx={{ fontSize: 12, lineHeight: '18px', fontWeight: 700, color: UI.inkSoft }}>書き換え案（AIの提案）</Typography>
                                <Typography sx={{ fontSize: 15, lineHeight: '24px', fontWeight: 700, color: UI.ink, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
                                  {f.rewrite}
                                </Typography>
                              </Box>
                              <Typography sx={{ fontSize: 12, lineHeight: '18px', color: UI.inkSoft }}>
                                事実と違う内容になっていないか、元の文章と見比べてから使ってください。
                              </Typography>
                            </>
                          ) : null}
                          {f.supplement && (
                            <Typography sx={{ fontSize: 14, lineHeight: '22px', color: UI.inkSoft, whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
                              {f.rewrite || f.quote ? `補足: ${f.supplement}` : `改善案: ${f.supplement}`}
                            </Typography>
                          )}
                        </>
                      )}
                    </Box>
                  )
                })}
              </Box>
            </Box>
          )
        })}
      </Box>
    </Box>
  )
}
