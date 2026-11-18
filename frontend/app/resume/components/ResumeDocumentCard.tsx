'use client'

import { useRef, useState } from 'react'
import { Alert, Box, Button, ButtonBase, LinearProgress, Stack, TextField, Typography } from '@mui/material'
import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import DescriptionIcon from '@mui/icons-material/Description'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import { UI } from '@/lib/design/tokens'
import { ResumeCard } from './ResumeCard'

type ResumeDocumentCardProps = {
  fileName: string
  documentId: number | null
  loading: boolean
  uploadError: string
  onSelectFile: (file: File) => void
  onLoadUrl: (url: string) => void
  onReset: () => void
}

const linkSx = {
  display: 'inline-flex',
  alignItems: 'center',
  p: 0,
  minHeight: 44,
  minWidth: 44,
  font: 'inherit',
  textDecoration: 'underline',
  textUnderlinePosition: 'from-font',
  borderRadius: '2px',
} as const

/** ① 書類。ファイルを置く（または選ぶ）と、そのまま読み込む。Google Docs は URL から。 */
export function ResumeDocumentCard({
  fileName,
  documentId,
  loading,
  uploadError,
  onSelectFile,
  onLoadUrl,
  onReset,
}: ResumeDocumentCardProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [urlOpen, setUrlOpen] = useState(false)
  const [url, setUrl] = useState('')

  const pick = (file: File | undefined) => {
    if (file) onSelectFile(file)
  }

  return (
    <ResumeCard title="1　レビューする書類">
      {documentId !== null ? (
        <Stack
          direction="row"
          alignItems="center"
          spacing={1.75}
          sx={{ px: 2.25, py: 2, bgcolor: UI.okTint, borderRadius: '8px' }}
        >
          <DescriptionIcon sx={{ fontSize: 32, color: UI.mark }} aria-hidden />
          <Box sx={{ flex: 1, minWidth: 0 }}>
            <Typography sx={{ fontSize: 17, lineHeight: '26px', fontWeight: 700, color: UI.ink, overflowWrap: 'anywhere' }}>
              {fileName}
            </Typography>
            <Stack direction="row" alignItems="center" spacing={0.75} role="status">
              <CheckCircleIcon sx={{ fontSize: 16, color: 'success.main' }} aria-hidden />
              <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>読み込めました</Typography>
            </Stack>
          </Box>
          <ButtonBase onClick={onReset} sx={{ ...linkSx, fontSize: 15, color: UI.ink, flexShrink: 0 }}>
            変更する
          </ButtonBase>
        </Stack>
      ) : loading ? (
        <Box role="status" sx={{ px: 3, py: 5, bgcolor: UI.tint, borderRadius: '8px', textAlign: 'center' }}>
          <Typography sx={{ fontSize: 17, fontWeight: 700, color: UI.ink, mb: 1.5 }}>書類を読み込んでいます</Typography>
          <LinearProgress aria-label="読み込みの進み具合" />
        </Box>
      ) : (
        <Box
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault()
            setDragging(false)
            pick(e.dataTransfer.files?.[0])
          }}
          sx={{
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 1.5,
            px: 3,
            py: 5,
            bgcolor: dragging ? UI.markTint : UI.tint,
            border: `1.5px dashed ${dragging ? UI.mark : UI.inkFaint}`,
            borderRadius: '8px',
            textAlign: 'center',
          }}
        >
          <CloudUploadIcon sx={{ fontSize: 40, color: UI.mark }} aria-hidden />
          <Typography sx={{ fontSize: 17, lineHeight: '26px', fontWeight: 700, color: UI.ink }}>ここにファイルをドラッグ</Typography>
          <Typography sx={{ fontSize: 14, color: UI.inkSoft }}>PDF または DOCX に対応しています</Typography>
          <Button
            variant="outlined"
            onClick={() => inputRef.current?.click()}
            sx={{ bgcolor: UI.field, color: UI.ink, borderColor: UI.inkFaint, borderWidth: 1.5, fontSize: 15, px: 2.75, '&:hover': { borderWidth: 1.5, borderColor: UI.ink, bgcolor: UI.field } }}
          >
            ファイルを選ぶ
          </Button>
          <input
            ref={inputRef}
            type="file"
            hidden
            accept=".pdf,.docx"
            aria-label="レビューする書類のファイル"
            onChange={(e) => {
              pick(e.target.files?.[0])
              // 同じファイルをもう一度選べるようにする
              e.target.value = ''
            }}
          />
        </Box>
      )}

      {uploadError && <Alert severity="error">{uploadError}</Alert>}

      {documentId === null && (
        <Box sx={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', columnGap: 0.75, fontSize: 14, color: UI.inkSoft }}>
          {urlOpen ? (
            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
              <TextField
                label="Google Docs の URL"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder="https://docs.google.com/…"
                size="small"
                fullWidth
              />
              <Button variant="contained" disabled={!url.trim() || loading} onClick={() => onLoadUrl(url.trim())} sx={{ whiteSpace: 'nowrap' }}>
                読み込む
              </Button>
            </Stack>
          ) : (
            <>
              Google Docs の場合は
              <ButtonBase onClick={() => setUrlOpen(true)} sx={{ ...linkSx, fontSize: 14, fontWeight: 700, color: UI.mark, verticalAlign: 'baseline' }}>
                URL から読み込む
              </ButtonBase>
            </>
          )}
        </Box>
      )}
    </ResumeCard>
  )
}
