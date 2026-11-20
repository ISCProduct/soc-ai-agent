'use client'

import { Box, ButtonBase } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { overlayRect, type Finding } from '../utils'
import { NoBadge } from './ResumeFindingParts'

type ResumePdfOverlayProps = {
  /** このページの指摘 */
  findings: Finding[]
  selectedNo: number | null
  onSelect: (no: number) => void
}

/**
 * 書類のページに重ねる、番号つきの強調。位置はページに対する割合で決めるので、
 * ページを描く大きさ（画面幅）が変わっても同じ場所を指す。
 * 位置が分からない指摘（形式チェックなど）は重ねない。間違った場所を指さないため。
 */
export function ResumePdfOverlay({ findings, selectedNo, onSelect }: ResumePdfOverlayProps) {
  return (
    <Box sx={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
      {findings.map((f) => {
        if (!f.box) return null
        const r = overlayRect(f.box)
        const selected = f.no === selectedNo
        return (
          <Box key={f.no} sx={{ position: 'absolute', left: `${r.left}%`, top: `${r.top}%`, width: `${r.width}%`, height: `${r.height}%` }}>
            <ButtonBase
              onClick={() => onSelect(f.no)}
              aria-pressed={selected}
              aria-label={`指摘${f.no}を選ぶ: ${f.quote || f.message}`}
              sx={{
                position: 'absolute',
                inset: 0,
                pointerEvents: 'auto',
                borderRadius: '4px',
                // 文字が読めるよう、塗りは薄くする
                bgcolor: selected ? `color-mix(in srgb, ${UI.mark} 14%, transparent)` : `color-mix(in srgb, ${UI.flag} 10%, transparent)`,
                border: selected ? `3px solid ${UI.mark}` : `1.5px solid ${UI.flag}`,
              }}
            />
            <Box sx={{ position: 'absolute', left: 0, top: '50%', transform: 'translate(-115%, -50%)', pointerEvents: 'none' }}>
              <NoBadge no={f.no} selected={selected} size={22} onFlag />
            </Box>
          </Box>
        )
      })}
    </Box>
  )
}
