'use client'

import { useEffect, useState, Suspense } from 'react'
import { useSearchParams } from 'next/navigation'
import {
  Box,
  Button,
  TextField,
  Typography,
  Paper,
  Stack,
  MenuItem,
  LinearProgress,
  Alert,
  Divider,
  Card,
  CardContent,
  Chip,
} from '@mui/material'
import { authService } from '@/lib/auth'

type ReviewItem = {
  id: number
  page_number: number
  severity: string
  message: string
  suggestion?: string
}

type ReviewResult = {
  review: {
    id: number
    score: number
    summary: string
  }
  items: ReviewItem[]
  annotated_available: boolean
}

const severityConfig: Record<string, { color: 'error' | 'warning' | 'info'; label: string; borderColor: string }> = {
  critical: { color: 'error', label: '重大', borderColor: '#d32f2f' },
  warning: { color: 'warning', label: '注意', borderColor: '#ed6c02' },
  info: { color: 'info', label: '情報', borderColor: '#0288d1' },
}

function ResumeContent() {
  const searchParams = useSearchParams()
  const [userId, setUserId] = useState('')
  const [sessionId, setSessionId] = useState('')
  const [sourceType, setSourceType] = useState('pdf')
  const [sourceUrl, setSourceUrl] = useState('')
  const [companyName, setCompanyName] = useState('')
  const [jobTitle, setJobTitle] = useState('')
  const [candidateType, setCandidateType] = useState('new_grad')
  const [file, setFile] = useState<File | null>(null)
  const [documentId, setDocumentId] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [reviewLoading, setReviewLoading] = useState(false)
  const [uploadError, setUploadError] = useState('')
  const [reviewError, setReviewError] = useState('')
  const [annotateError, setAnnotateError] = useState('')
  const [review, setReview] = useState<ReviewResult | null>(null)
  const [ragReport, setRagReport] = useState('')

  const prefilledCompany = searchParams.get('company_name') || ''
  const prefilledIndustry = searchParams.get('industry') || ''

  useEffect(() => {
    const user = authService.getStoredUser()
    if (user?.user_id) {
      setUserId(String(user.user_id))
    }
    if (typeof window !== 'undefined') {
      const storedSession =
        localStorage.getItem('chat_session_id') ||
        sessionStorage.getItem('chatSessionId') ||
        localStorage.getItem('currentSessionId') ||
        ''
      setSessionId(storedSession)
    }
    if (prefilledCompany) {
      setCompanyName(prefilledCompany)
    }
  }, [prefilledCompany])

  const handleUpload = async () => {
    setUploadError('')
    setReview(null)
    setLoading(true)
    try {
      if (!userId) {
        throw new Error('user_id が取得できません。ログインしてください。')
      }
      const formData = new FormData()
      formData.append('user_id', userId)
      if (sessionId) {
        formData.append('session_id', sessionId)
      }
      formData.append('source_type', sourceType)
      if (sourceUrl) {
        formData.append('source_url', sourceUrl)
      }
      if (file) {
        formData.append('file', file)
      }

      const response = await fetch('/api/resume/upload', {
        method: 'POST',
        body: formData,
      })
      if (!response.ok) {
        const errText = await response.text()
        let message = errText
        try {
          const parsed = JSON.parse(errText)
          message = parsed?.error || parsed?.message || errText
        } catch {
          message = errText || 'Upload failed'
        }
        throw new Error(message)
      }
      const data = await response.json()
      setDocumentId(data?.document?.id ?? null)
    } catch (err) {
      setUploadError(err instanceof Error ? err.message : 'Upload failed')
    } finally {
      setLoading(false)
    }
  }

  const handleReview = async () => {
    if (!documentId) {
      setReviewError('document_id が未設定です')
      return
    }
    if (!companyName.trim() && !jobTitle.trim()) {
      setReviewError('企業名が未入力の場合は応募職種を入力してください')
      return
    }
    setReviewError('')
    setAnnotateError('')
    setReview(null)
    setRagReport('')
    setReviewLoading(true)

    try {
      const response = await fetch(`/api/resume/review/stream?document_id=${documentId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          company_name: companyName,
          job_title: jobTitle,
          candidate_type: candidateType,
        }),
      })

      if (!response.ok || !response.body) {
        const errText = await response.text()
        throw new Error(errText || 'Review failed')
      }

      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''

      while (true) {
        const { done, value } = await reader.read()
        if (done) break

        buffer += decoder.decode(value, { stream: true })
        const lines = buffer.split('\n')
        buffer = lines.pop() ?? ''

        for (const line of lines) {
          if (!line.startsWith('data: ')) continue
          try {
            const data = JSON.parse(line.slice(6))
            if (data.type === 'chunk') {
              setRagReport((prev) => prev + data.text)
            } else if (data.type === 'complete') {
              setReview({ review: data.review, items: data.items, annotated_available: data.annotated_available ?? false })
            } else if (data.type === 'annotate_error') {
              setAnnotateError(data.message)
            } else if (data.type === 'error') {
              throw new Error(data.message)
            }
          } catch (parseErr) {
            if (parseErr instanceof Error && parseErr.message !== 'Unexpected token') {
              throw parseErr
            }
          }
        }
      }
    } catch (err) {
      setReviewError(err instanceof Error ? err.message : 'Review failed')
    } finally {
      setReviewLoading(false)
    }
  }

  const handleDownload = () => {
    if (!documentId) {
      setReviewError('document_id が未設定です')
      return
    }
    window.open(`/api/resume/annotated?document_id=${documentId}`, '_blank')
  }

  return (
    <Box sx={{ p: 4, maxWidth: 900, mx: 'auto' }}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        履歴書・エントリシート レビュー
      </Typography>
      <Typography variant="body1" color="text.secondary" sx={{ mb: prefilledCompany ? 1.5 : 3 }}>
        PDF/DOCX/Google Docsをアップロードして、注釈付きPDFを生成します。
      </Typography>
      {prefilledCompany && (
        <Box sx={{ mb: 3, px: 2, py: 1.5, bgcolor: '#e8f5e9', borderRadius: 1, border: '1px solid #a5d6a7' }}>
          <Typography sx={{ fontSize: 14, color: '#2e7d32', fontWeight: 600 }}>
            {prefilledCompany}{prefilledIndustry ? `（${prefilledIndustry}）` : ''}向けに最適化されたフィードバックを提供します
          </Typography>
          <Typography sx={{ fontSize: 12, color: '#388e3c', mt: 0.5 }}>
            企業の求める人材像を踏まえたアドバイスが反映されます
          </Typography>
        </Box>
      )}

      <Paper sx={{ p: 3, mb: 3 }} elevation={2}>
        <Stack spacing={2}>
          <TextField
            select
            label="提出形式"
            value={sourceType}
            onChange={(e) => setSourceType(e.target.value)}
            fullWidth
          >
            <MenuItem value="pdf">PDF</MenuItem>
            <MenuItem value="docx">DOCX</MenuItem>
            <MenuItem value="google_docs">Google Docs</MenuItem>
          </TextField>
          <TextField
            label="Google Docs / URL (任意)"
            value={sourceUrl}
            onChange={(e) => setSourceUrl(e.target.value)}
            placeholder="https://... (PDFエクスポートURL)"
            fullWidth
          />
          <Button variant="outlined" component="label">
            ファイルを選択
            <input
              type="file"
              hidden
              accept=".pdf,.docx"
              onChange={(e) => setFile(e.target.files?.[0] ?? null)}
            />
          </Button>
          {file && (
            <Typography variant="body2" color="text.secondary">
              選択ファイル: {file.name}
            </Typography>
          )}
          <Button variant="contained" onClick={handleUpload} disabled={loading}>
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

      <Paper sx={{ p: 3 }} elevation={2}>
        <Stack spacing={2}>
          <Typography variant="h6">レビュー実行</Typography>
          <TextField
            label="応募企業名 (任意)"
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            fullWidth
          />
          <TextField
            label={companyName.trim() ? '応募職種 (任意)' : '応募職種 (企業名未入力の場合は必須)'}
            value={jobTitle}
            onChange={(e) => setJobTitle(e.target.value)}
            fullWidth
            required={!companyName.trim()}
            error={!companyName.trim() && !jobTitle.trim()}
            helperText={!companyName.trim() ? '企業名が未入力の場合は職種を入力するとAIレビューが実行されます' : ''}
          />
          <TextField
            select
            label="候補者区分"
            value={candidateType}
            onChange={(e) => setCandidateType(e.target.value)}
            fullWidth
          >
            <MenuItem value="new_grad">新卒</MenuItem>
            <MenuItem value="mid_career">中途</MenuItem>
          </TextField>
          <Button variant="contained" onClick={handleReview} disabled={reviewLoading || !documentId}>
            レビューを生成
          </Button>
          {reviewLoading && (
            <Box>
              <LinearProgress />
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                {ragReport ? '企業別レビューレポートを生成中...' : 'PDFを解析中...（通常30〜60秒かかります）'}
              </Typography>
            </Box>
          )}
          {reviewError && (
            <Alert severity="error">{reviewError}</Alert>
          )}
          {review && !reviewLoading && (
            <Alert severity="success">レビューが完了しました。下の指摘事項をご確認ください。</Alert>
          )}
        </Stack>
      </Paper>

      {ragReport && (
        <Paper sx={{ p: 3, mt: 4 }} elevation={2}>
          <Typography variant="h5" fontWeight="bold" gutterBottom>
            企業別レビューレポート
            {reviewLoading && (
              <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                生成中...
              </Typography>
            )}
          </Typography>
          <Box
            sx={{
              whiteSpace: 'pre-wrap',
              fontFamily: 'inherit',
              fontSize: '0.95rem',
              lineHeight: 1.8,
              color: 'text.primary',
            }}
          >
            {ragReport}
          </Box>
        </Paper>
      )}

      {review && (
        <Paper sx={{ p: 3, mt: 4 }} elevation={2}>
          <Typography variant="h5" fontWeight="bold" gutterBottom>
            指摘事項
          </Typography>
          <Box sx={{ mb: 2 }}>
            <Typography variant="h6" gutterBottom>
              総合スコア: {review.review.score} / 100
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {review.review.summary}
            </Typography>
          </Box>
          <Divider sx={{ mb: 3 }} />
          <Stack spacing={2}>
            {review.items.map((item) => {
              const config = severityConfig[item.severity] ?? { color: 'default' as const, label: item.severity, borderColor: '#9e9e9e' }
              return (
                <Card
                  key={item.id}
                  variant="outlined"
                  sx={{ borderLeft: 4, borderLeftColor: config.borderColor }}
                >
                  <CardContent>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
                      <Chip label={config.label} color={config.color} size="small" />
                      <Typography variant="caption" color="text.secondary">
                        ページ {item.page_number}
                      </Typography>
                    </Box>
                    <Typography variant="body1" fontWeight="medium">
                      {item.message}
                    </Typography>
                    {item.suggestion && (
                      <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                        改善案: {item.suggestion}
                      </Typography>
                    )}
                  </CardContent>
                </Card>
              )
            })}
          </Stack>
          <Box sx={{ mt: 3 }}>
            {annotateError && (
              <Alert severity="warning" sx={{ mb: 2 }}>{annotateError}</Alert>
            )}
            {review.annotated_available && (
              <Button variant="outlined" onClick={handleDownload}>
                注釈PDFをダウンロード
              </Button>
            )}
          </Box>
        </Paper>
      )}
    </Box>
  )
}

export default function ResumePage() {
  return (
    <Suspense fallback={null}>
      <ResumeContent />
    </Suspense>
  )
}
