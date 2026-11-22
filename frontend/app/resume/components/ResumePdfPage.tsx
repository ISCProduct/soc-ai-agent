'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { Box, Skeleton } from '@mui/material'
import { Document, Page, pdfjs } from 'react-pdf'
import { authService } from '@/lib/auth'
import { UI } from '@/lib/design/tokens'
import type { Finding } from '../utils'
import { ResumePdfOverlay } from './ResumePdfOverlay'

// PDF.js の処理は Web Worker で行う。ファイルは自分のドメインから配る（CSP の script-src 'self' で足りる）
pdfjs.GlobalWorkerOptions.workerSrc = new URL('pdfjs-dist/build/pdf.worker.min.mjs', import.meta.url).toString()

type ResumePdfPageProps = {
  documentId: number
  page: number
  /** このページの指摘 */
  findings: Finding[]
  selectedNo: number | null
  onSelect: (no: number) => void
  onNumPages: (n: number) => void
  /** 読み込めなかった（書類がまだ無い・壊れている・通信失敗）。呼び出し側が代わりの表示に切り替える */
  onError: () => void
}

const MAX_PAGE_WIDTH = 560

// 日本語の書類は、フォントを埋め込まず文字コードの対応表（CMap）に頼ることが多い。
// 無いと文字が崩れる。public/pdfjs に置いた写しを使う（pdfjs-dist を上げたら `npm run pdfjs:assets` で更新する）
const PDF_OPTIONS = {
  cMapUrl: '/pdfjs/cmaps/',
  cMapPacked: true,
  standardFontDataUrl: '/pdfjs/standard_fonts/',
} as const

/** 実際の書類（PDF）の1ページを描き、指摘の位置に番号つきの強調を重ねる。 */
export default function ResumePdfPage({ documentId, page, findings, selectedNo, onSelect, onNumPages, onError }: ResumePdfPageProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)

  // 描く幅は、置き場所の幅に合わせる（スマホでは画面幅いっぱい）
  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const update = () => setWidth(Math.min(el.clientWidth, MAX_PAGE_WIDTH))
    update()
    const ro = new ResizeObserver(update)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  // file を毎回作り直すと、PDF を読み直してしまう
  const file = useMemo(
    () => ({
      url: `/api/resume/document?document_id=${documentId}`,
      httpHeaders: authService.getUserFetchHeaders(),
      withCredentials: true,
    }),
    [documentId],
  )

  const unplaced = findings.filter((f) => !f.box).length

  return (
    <Box ref={wrapRef} sx={{ width: '100%' }}>
      {width > 0 && (
        <Document
          file={file}
          options={PDF_OPTIONS}
          onLoadSuccess={({ numPages }) => onNumPages(numPages)}
          onLoadError={onError}
          onSourceError={onError}
          loading={<Skeleton variant="rectangular" width={width} height={Math.round(width * 1.41)} />}
          error={null}
        >
          <Box sx={{ position: 'relative', width, mx: 'auto', bgcolor: UI.field, border: `1px solid ${UI.rule}`, lineHeight: 0 }}>
            <Page
              pageNumber={page}
              width={Math.max(0, width - 2)}
              renderTextLayer={false}
              renderAnnotationLayer={false}
              onLoadError={onError}
              loading={<Skeleton variant="rectangular" width={width - 2} height={Math.round(width * 1.41)} />}
            />
            <ResumePdfOverlay findings={findings} selectedNo={selectedNo} onSelect={onSelect} />
          </Box>
        </Document>
      )}
      {unplaced > 0 && (
        <Box sx={{ mt: 1.5, fontSize: 12, textAlign: 'center', color: UI.inkSoft }}>
          このページの指摘のうち{unplaced}件は、書類の上の位置を特定できませんでした。右の一覧で確認してください。
        </Box>
      )}
    </Box>
  )
}
