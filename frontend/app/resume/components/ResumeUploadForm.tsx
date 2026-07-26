'use client'

import {
  Alert,
  Box,
  Button,
  LinearProgress,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'

type ResumeUploadFormProps = {
  sourceType: string
  onSourceTypeChange: (value: string) => void
  sourceUrl: string
  onSourceUrlChange: (value: string) => void
  file: File | null
  onFileChange: (file: File | null) => void
  loading: boolean
  uploadError: string
  documentId: number | null
  onUpload: () => void
}

export function ResumeUploadForm({
  sourceType,
  onSourceTypeChange,
  sourceUrl,
  onSourceUrlChange,
  file,
  onFileChange,
  loading,
  uploadError,
  documentId,
  onUpload,
}: ResumeUploadFormProps) {
  return (
    <Paper sx={{ p: 3, mb: 3 }} elevation={2}>
      <Stack spacing={2}>
        <TextField
          select
          label="提出形式"
          value={sourceType}
          onChange={(e) => onSourceTypeChange(e.target.value)}
          fullWidth
        >
          <MenuItem value="pdf">PDF</MenuItem>
          <MenuItem value="docx">DOCX</MenuItem>
          <MenuItem value="google_docs">Google Docs</MenuItem>
        </TextField>
        <TextField
          label="Google Docs / URL (任意)"
          value={sourceUrl}
          onChange={(e) => onSourceUrlChange(e.target.value)}
          placeholder="https://... (PDFエクスポートURL)"
          fullWidth
        />
        <Button variant="outlined" component="label">
          ファイルを選択
          <input
            type="file"
            hidden
            accept=".pdf,.docx"
            onChange={(e) => onFileChange(e.target.files?.[0] ?? null)}
          />
        </Button>
        {file && (
          <Typography variant="body2" color="text.secondary">
            選択ファイル: {file.name}
          </Typography>
        )}
        <Button variant="contained" onClick={onUpload} disabled={loading}>
          アップロード
        </Button>
        {loading && (
          <Box>
            <LinearProgress />
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              アップロード中...
            </Typography>
          </Box>
        )}
        {uploadError && (
          <Alert severity="error">{uploadError}</Alert>
        )}
        {documentId && (
          <Alert severity="success">
            アップロード完了！下のフォームでレビューを実行してください。
          </Alert>
        )}
      </Stack>
    </Paper>
  )
}
