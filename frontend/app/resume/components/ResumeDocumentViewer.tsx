'use client'

import { Box, ButtonBase, IconButton, Stack, Typography } from '@mui/material'
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import { UI } from '@/lib/design/tokens'
import type { Finding } from '../utils'
import { NoBadge } from './ResumeFindingParts'

type ResumeDocumentViewerProps = {
  page: number
  pageCount: number
  /** このページの指摘 */
  findings: Finding[]
  selectedNo: number | null
  onSelect: (no: number) => void
  onPageChange: (page: number) => void
}

const navButtonSx = {
  width: 36,
  height: 36,
  borderRadius: '6px',
  border: `1.5px solid ${UI.inkFaint}`,
  bgcolor: UI.field,
  color: UI.ink,
  '&.Mui-disabled': { borderColor: UI.rule, color: UI.inkFaint },
} as const

function Bar({ w }: { w: number | string }) {
  return <Box aria-hidden sx={{ height: 8, width: w, maxWidth: '100%', borderRadius: '2px', bgcolor: UI.ruleSoft }} />
}

/**
 * 書類のページ。指摘された原文を、番号つきで強調して並べる。
 * 書類そのものの描画ではなく、該当箇所を並べた表示（レイアウトは実物と異なる）。
 */
export function ResumeDocumentViewer({ page, pageCount, findings, selectedNo, onSelect, onPageChange }: ResumeDocumentViewerProps) {
  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        minWidth: 0,
        height: { md: 800 },
        bgcolor: UI.ruleSoft,
        border: `1px solid ${UI.rule}`,
        borderRadius: '8px',
        overflow: 'hidden',
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ px: { xs: 1.5, md: 2 }, py: 1.25, gap: 1, bgcolor: UI.field, borderBottom: `1px solid ${UI.rule}`, flexShrink: 0 }}
      >
        <Stack direction="row" alignItems="center" spacing={1}>
          <IconButton aria-label="前のページ" disabled={page <= 1} onClick={() => onPageChange(page - 1)} sx={navButtonSx}>
            <ChevronLeftIcon sx={{ fontSize: 20 }} />
          </IconButton>
          <Typography sx={{ fontSize: 15, fontWeight: 700, color: UI.ink, whiteSpace: 'nowrap' }}>
            {page} / {pageCount} ページ
          </Typography>
          <IconButton aria-label="次のページ" disabled={page >= pageCount} onClick={() => onPageChange(page + 1)} sx={navButtonSx}>
            <ChevronRightIcon sx={{ fontSize: 20 }} />
          </IconButton>
        </Stack>
        <Typography sx={{ fontSize: { xs: 13, md: 14 }, fontWeight: 700, color: UI.mark, whiteSpace: 'nowrap' }}>このページの指摘 {findings.length}件</Typography>
      </Stack>

      <Box sx={{ flex: 1, overflow: 'auto', px: { xs: 1.5, md: 0 }, py: { xs: 2, md: 3.5 } }}>
        <Box
          sx={{
            width: { md: 520 },
            maxWidth: '100%',
            mx: 'auto',
            minHeight: { md: 700 },
            bgcolor: UI.field,
            border: `1px solid ${UI.rule}`,
            pl: { xs: 4.5, md: 4.375 },
            pr: { xs: 2, md: 4.375 },
            py: 4,
          }}
        >
          <Typography sx={{ fontSize: 18, fontWeight: 700, color: UI.ink, mb: 2.5 }}>{page === 1 ? '履　歴　書' : `履歴書（${page}/${pageCount}）`}</Typography>
          <Stack spacing={1.25} sx={{ mb: 3 }} aria-hidden>
            <Bar w={200} />
            <Bar w={320} />
            <Bar w={120} />
          </Stack>

          {findings.length === 0 && (
            <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>このページに指摘はありません。</Typography>
          )}

          {findings.map((f) => {
            const selected = f.no === selectedNo
            const text = f.quote || f.message
            return (
              <Box key={f.no} sx={{ mb: 3 }}>
                <Stack spacing={1.25} sx={{ mb: 2 }} aria-hidden>
                  <Bar w="100%" />
                  <Bar w="58%" />
                </Stack>
                <Box sx={{ position: 'relative' }}>
                  <Box sx={{ position: 'absolute', left: { xs: -34, md: -28 }, top: 6 }}>
                    <NoBadge no={f.no} selected={selected} size={22} onFlag />
                  </Box>
                  <ButtonBase
                    onClick={() => onSelect(f.no)}
                    aria-pressed={selected}
                    aria-label={`指摘${f.no}を選ぶ: ${text}`}
                    sx={{
                      width: '100%',
                      display: 'block',
                      textAlign: 'left',
                      px: 1.5,
                      py: 1,
                      borderRadius: '4px',
                      fontSize: 14,
                      lineHeight: 1.6,
                      color: UI.ink,
                      overflowWrap: 'anywhere',
                      bgcolor: selected ? UI.markTint : UI.flagTint,
                      border: selected ? `3px solid ${UI.mark}` : `1.5px solid ${UI.flag}`,
                      // 太さが変わっても文字の位置が動かないよう、細い側で余白を足す
                      ...(selected ? { px: 'calc(12px - 1.5px)', py: 'calc(8px - 1.5px)' } : {}),
                    }}
                  >
                    {text}
                  </ButtonBase>
                </Box>
              </Box>
            )
          })}

          <Stack spacing={1.25} aria-hidden>
            <Bar w="100%" />
            <Bar w="100%" />
            <Bar w="58%" />
          </Stack>
        </Box>
        <Typography sx={{ mt: 1.5, px: 2, textAlign: 'center', fontSize: 12, color: UI.inkSoft }}>
          指摘された原文を並べた表示です。実際の書類のレイアウトとは異なります。
        </Typography>
      </Box>
    </Box>
  )
}
