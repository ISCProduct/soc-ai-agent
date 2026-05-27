'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  IconButton,
  LinearProgress,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import AutoFixHighIcon from '@mui/icons-material/AutoFixHigh'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CheckIcon from '@mui/icons-material/Check'
import EditNoteIcon from '@mui/icons-material/EditNote'
import RateReviewIcon from '@mui/icons-material/RateReview'
import { BACKEND_URL } from '@/lib/backend-url'
const PRIMARY = '#ec5b13'

const QUESTION_TYPES = ['志望動機', '自己PR', '学チカ', 'ガクチカ', 'その他']

type StarBreakdown = {
  situation: string
  task: string
  action: string
  result: string
}

type RewriteResult = {
  rewritten_text: string
  star: StarBreakdown
}

type ReviewResult = {
  specificity_score: number
  star_score: number
  company_fit_score: number | null
  length_balance_score: number
  feedback: string
  improved_text: string
}

const STAR_LABELS: { key: keyof StarBreakdown; label: string; color: string; emoji: string }[] = [
  { key: 'situation', label: 'Situation（状況）', color: '#3b82f6', emoji: '📍' },
  { key: 'task',      label: 'Task（課題）',      color: '#8b5cf6', emoji: '🎯' },
  { key: 'action',    label: 'Action（施策）',    color: PRIMARY,   emoji: '⚡' },
  { key: 'result',    label: 'Result（成果）',    color: '#10b981', emoji: '📊' },
]

const SCORE_ITEMS: { key: keyof Omit<ReviewResult, 'feedback' | 'improved_text'>; label: string; color: string }[] = [
  { key: 'specificity_score',    label: '具体性',       color: '#3b82f6' },
  { key: 'star_score',           label: 'STAR法準拠',   color: '#8b5cf6' },
  { key: 'company_fit_score',    label: '企業適合性',   color: PRIMARY },
  { key: 'length_balance_score', label: '文字数バランス', color: '#10b981' },
]

export default function ESRewritePage() {
  const router = useRouter()

  const [mode, setMode] = useState<'rewrite' | 'review'>('rewrite')
  const [originalText, setOriginalText] = useState('')
  const [questionType, setQuestionType] = useState('学チカ')
  const [techStack, setTechStack] = useState('')
  const [companyName, setCompanyName] = useState('')
  const [loading, setLoading] = useState(false)
  const [rewriteResult, setRewriteResult] = useState<RewriteResult | null>(null)
  const [reviewResult, setReviewResult] = useState<ReviewResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const handleRewrite = async () => {
    if (!originalText.trim()) return
    setLoading(true)
    setError(null)
    setRewriteResult(null)

    try {
      const res = await fetch(`${BACKEND_URL}/api/es/rewrite`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          original_text: originalText,
          question_type: questionType,
          tech_stack: techStack,
      company_name: companyName,
    }),
  })
  if (!res.ok) throw new Error(await res.text())
  setRewriteResult(await res.json())
    } catch (e: unknown) {
  setError(e instanceof Error ? e.message : 'リライトに失敗しました。再試行してください。')
    } finally {
  setLoading(false)
    }
  }

  const handleReview = async () => {
    if (!originalText.trim()) return
    setLoading(true)
    setError(null)
    setReviewResult(null)

    try {
      const res = await fetch(`${BACKEND_URL}/api/es/review`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          es_text: originalText,
          question_type: questionType,
          company_name: companyName,
        }),
      })
      if (!res.ok) throw new Error(await res.text())
      setReviewResult(await res.json())
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '添削に失敗しました。再試行してください。')
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = async (text: string) => {
    await navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  const result = mode === 'rewrite' ? rewriteResult : reviewResult

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f8f6f6' }}>
      {/* Header */}
      <Box
        component="header"
        sx={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between',
          px: { xs: 2, sm: 3, lg: 8 }, py: 2,
          bgcolor: '#fff', borderBottom: '1px solid #e2e8f0',
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ color: PRIMARY }}><EditNoteIcon sx={{ fontSize: 32 }} /></Box>
          <Box>
            <Typography sx={{ fontWeight: 700, fontSize: { xs: 16, sm: 20 }, color: '#0f172a' }}>ES添削・リライト</Typography>
            <Typography sx={{ fontSize: 12, color: '#64748b' }}>AIがあなたのES文章を添削・リライトします</Typography>
          </Box>
        </Box>
        <IconButton onClick={() => router.push('/')} sx={{ bgcolor: '#f1f5f9', color: '#475569' }}>
          <ArrowBackIcon />
        </IconButton>
      </Box>

      {/* Mode toggle */}
      <Box sx={{ maxWidth: 1200, mx: 'auto', px: { xs: 2, md: 4 }, pt: 3 }}>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mb: 3 }}>
          <Button
            startIcon={<RateReviewIcon />}
            onClick={() => { setMode('review'); setRewriteResult(null); setError(null) }}
            sx={{
              px: 2.5, py: 1, borderRadius: 2, textTransform: 'none', fontWeight: 700, fontSize: 14,
              bgcolor: mode === 'review' ? PRIMARY : '#f1f5f9',
              color: mode === 'review' ? '#fff' : '#475569',
              '&:hover': { bgcolor: mode === 'review' ? `${PRIMARY}e0` : '#e2e8f0' },
            }}
          >
            ES添削（RAGフィードバック）
          </Button>
          <Button
            startIcon={<AutoFixHighIcon />}
            onClick={() => { setMode('rewrite'); setReviewResult(null); setError(null) }}
            sx={{
              px: 2.5, py: 1, borderRadius: 2, textTransform: 'none', fontWeight: 700, fontSize: 14,
              bgcolor: mode === 'rewrite' ? PRIMARY : '#f1f5f9',
              color: mode === 'rewrite' ? '#fff' : '#475569',
              '&:hover': { bgcolor: mode === 'rewrite' ? `${PRIMARY}e0` : '#e2e8f0' },
            }}
          >
            ESリライト（STAR法）
          </Button>
        </Box>
      </Box>

      {/* Main */}
      <Box sx={{ maxWidth: 1200, mx: 'auto', px: { xs: 2, md: 4 }, pb: 4 }}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', lg: result ? '1fr 1fr' : '1fr' },
            gap: 3,
            alignItems: 'start',
          }}
        >
          {/* ── Left: Input ── */}
          <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
            <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2.5 }}>
              {mode === 'review' ? 'ES添削' : 'ESリライト'} — 元の文章を入力
            </Typography>

            {/* 質問種別 */}
            <Box sx={{ mb: 2.5 }}>
              <Typography sx={{ fontSize: 13, fontWeight: 600, color: '#475569', mb: 1 }}>質問種別</Typography>
              <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
                {QUESTION_TYPES.map(type => (
                  <Chip
                    key={type}
                    label={type}
                    onClick={() => setQuestionType(type)}
                    sx={{
                      cursor: 'pointer', fontWeight: 600,
                      bgcolor: questionType === type ? PRIMARY : '#f1f5f9',
                      color: questionType === type ? '#fff' : '#475569',
                      '&:hover': { bgcolor: questionType === type ? `${PRIMARY}e0` : '#e2e8f0' },
                    }}
                  />
                ))}
              </Box>
            </Box>

            {/* ES本文 */}
            <TextField
              multiline
              rows={10}
              fullWidth
              value={originalText}
              onChange={e => setOriginalText(e.target.value)}
              placeholder="例）チームで開発した経験があります。最初は上手くいきませんでしたが、話し合いを重ねて最終的には完成させることができました。この経験から協調性の大切さを学びました。"
              sx={{
                mb: 2.5,
                '& .MuiOutlinedInput-root': {
                  fontSize: 14, lineHeight: 1.8,
                  '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                  '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                },
              }}
            />

            {/* 添削モード: 志望企業 / リライトモード: 技術スタック */}
            {mode === 'review' ? (
              <TextField
                fullWidth
                size="small"
                value={companyName}
                onChange={e => setCompanyName(e.target.value)}
                label="志望企業名（任意・入力で企業適合性を評価）"
                placeholder="例: 株式会社サイバーエージェント"
                sx={{
                  mb: 3,
                  '& .MuiOutlinedInput-root': {
                    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                    '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                  },
                  '& .MuiInputLabel-root.Mui-focused': { color: PRIMARY },
                }}
              />
            ) : (
              <>
                <TextField
                  fullWidth
                  size="small"
                  value={techStack}
                  onChange={e => setTechStack(e.target.value)}
                  label="使用技術スタック（任意）"
                  placeholder="例: React, Node.js, PostgreSQL, Docker"
                  sx={{
                    mb: 3,
                    '& .MuiOutlinedInput-root': {
                      '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                      '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                    },
                    '& .MuiInputLabel-root.Mui-focused': { color: PRIMARY },
                  }}
                />
                <TextField
                  fullWidth
                  size="small"
                  value={companyName}
                  onChange={e => setCompanyName(e.target.value)}
                  label="志望企業名（任意・入力で企業情報を参照）"
                  placeholder="例: 株式会社サイバーエージェント"
                  sx={{
                    mb: 3,
                    '& .MuiOutlinedInput-root': {
                      '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                      '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                    },
                    '& .MuiInputLabel-root.Mui-focused': { color: PRIMARY },
                  }}
                />
              </>
            )}

            <Button
              variant="contained"
              fullWidth
              size="large"
              disabled={!originalText.trim() || loading}
              onClick={mode === 'review' ? handleReview : handleRewrite}
              startIcon={loading ? <CircularProgress size={18} sx={{ color: '#fff' }} /> : (mode === 'review' ? <RateReviewIcon /> : <AutoFixHighIcon />)}
              sx={{
                bgcolor: PRIMARY, '&:hover': { bgcolor: `${PRIMARY}e0` },
                borderRadius: 2, py: 1.5, fontWeight: 700, fontSize: 16,
                textTransform: 'none',
                '&:disabled': { bgcolor: '#e2e8f0', color: '#94a3b8' },
              }}
            >
              {loading
                ? (mode === 'review' ? '添削中...' : 'リライト中...')
                : (mode === 'review' ? 'AIで添削する' : 'AIでリライトする')
              }
            </Button>

            {error && (
              <Alert severity="error" sx={{ mt: 2, borderRadius: 2 }}>{error}</Alert>
            )}
          </Paper>

          {/* ── Right: Result ── */}
          {mode === 'review' && reviewResult && (
            <Stack spacing={2}>
              {/* Score gauges */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `2px solid ${PRIMARY}`, bgcolor: `${PRIMARY}04` }}>
                <Typography sx={{ fontWeight: 700, fontSize: 17, color: PRIMARY, mb: 2.5 }}>添削スコア</Typography>
                <Stack spacing={2}>
                  {SCORE_ITEMS.map(({ key, label, color }) => {
                    const score = reviewResult[key]
                    if (score === null) return (
                      <Box key={key}>
                        <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                          <Typography sx={{ fontSize: 13, fontWeight: 600, color: '#94a3b8' }}>{label}</Typography>
                          <Typography sx={{ fontSize: 12, color: '#94a3b8' }}>企業名未入力</Typography>
                        </Stack>
                        <LinearProgress variant="determinate" value={0} sx={{ height: 6, borderRadius: 3, bgcolor: '#e2e8f0', '& .MuiLinearProgress-bar': { bgcolor: '#e2e8f0' } }} />
                      </Box>
                    )
                    const pct = ((score as number) / 10) * 100
                    return (
                      <Box key={key}>
                        <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                          <Typography sx={{ fontSize: 13, fontWeight: 600, color: '#475569' }}>{label}</Typography>
                          <Typography sx={{ fontSize: 13, fontWeight: 700, color }}>{score} / 10</Typography>
                        </Stack>
                        <LinearProgress
                          variant="determinate"
                          value={pct}
                          sx={{ height: 8, borderRadius: 4, bgcolor: '#e2e8f0', '& .MuiLinearProgress-bar': { bgcolor: color, borderRadius: 4 } }}
                        />
                      </Box>
                    )
                  })}
                </Stack>
              </Paper>

              {/* Feedback */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 1.5 }}>フィードバック</Typography>
                <Typography variant="body2" sx={{ color: '#475569', lineHeight: 1.8 }}>
                  {reviewResult.feedback}
                </Typography>
              </Paper>

              {/* Before / After comparison */}
              {reviewResult.improved_text && (
                <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                    <Typography sx={{ fontWeight: 700, fontSize: 16 }}>改善後テキスト</Typography>
                    <Tooltip title={copied ? 'コピーしました' : 'クリップボードにコピー'}>
                      <IconButton
                        size="small"
                        onClick={() => handleCopy(reviewResult.improved_text)}
                        sx={{ bgcolor: copied ? '#10b981' : '#f1f5f9', '&:hover': { bgcolor: copied ? '#059669' : '#e2e8f0' } }}
                      >
                        {copied
                          ? <CheckIcon sx={{ color: '#fff', fontSize: 18 }} />
                          : <ContentCopyIcon sx={{ color: '#475569', fontSize: 18 }} />
                        }
                      </IconButton>
                    </Tooltip>
                  </Box>
                  <Typography sx={{ fontSize: 14, lineHeight: 1.9, color: '#1e293b', whiteSpace: 'pre-wrap', mb: 2 }}>
                    {reviewResult.improved_text}
                  </Typography>
                  <Divider sx={{ mb: 2 }} />
                  <Typography sx={{ fontWeight: 700, fontSize: 13, mb: 1, color: '#64748b' }}>元の文章</Typography>
                  <Typography variant="body2" sx={{ color: '#94a3b8', lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
                    {originalText}
                  </Typography>
                </Paper>
              )}
            </Stack>
          )}

          {mode === 'rewrite' && rewriteResult && (
            <Stack spacing={2}>
              {/* Rewritten text */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `2px solid ${PRIMARY}`, bgcolor: `${PRIMARY}04` }}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                    <AutoFixHighIcon sx={{ color: PRIMARY, fontSize: 20 }} />
                    <Typography sx={{ fontWeight: 700, fontSize: 17, color: PRIMARY }}>リライト後</Typography>
                  </Box>
                  <Tooltip title={copied ? 'コピーしました' : 'クリップボードにコピー'}>
                    <IconButton
                      size="small"
                      onClick={() => handleCopy(rewriteResult.rewritten_text)}
                      sx={{ bgcolor: copied ? '#10b981' : '#f1f5f9', '&:hover': { bgcolor: copied ? '#059669' : '#e2e8f0' } }}
                    >
                      {copied
                        ? <CheckIcon sx={{ color: '#fff', fontSize: 18 }} />
                        : <ContentCopyIcon sx={{ color: '#475569', fontSize: 18 }} />
                      }
                    </IconButton>
                  </Tooltip>
                </Box>
                <Typography sx={{ fontSize: 14, lineHeight: 1.9, color: '#1e293b', whiteSpace: 'pre-wrap' }}>
                  {rewriteResult.rewritten_text}
                </Typography>
              </Paper>

              {/* STAR breakdown */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 2 }}>STAR法 分解</Typography>
                <Stack spacing={2}>
                  {STAR_LABELS.map(({ key, label, color, emoji }, idx) => (
                    <Box key={key}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.8 }}>
                        <Box sx={{ width: 28, height: 28, borderRadius: 1.5, bgcolor: `${color}15`, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: 14 }}>
                          {emoji}
                        </Box>
                        <Typography sx={{ fontWeight: 700, fontSize: 13, color }}>{label}</Typography>
                      </Box>
                      <Typography variant="body2" sx={{ color: '#475569', lineHeight: 1.75, borderLeft: `3px solid ${color}40`, pl: 1.5 }}>
                        {rewriteResult.star[key] || '—'}
                      </Typography>
                      {idx < STAR_LABELS.length - 1 && <Divider sx={{ mt: 1.5, borderColor: '#f1f5f9' }} />}
                    </Box>
                  ))}
                </Stack>
              </Paper>

              {/* Compare hint */}
              <Paper elevation={0} sx={{ p: 2, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 13, mb: 1, color: '#64748b' }}>元の文章</Typography>
                <Typography variant="body2" sx={{ color: '#94a3b8', lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
                  {originalText}
                </Typography>
              </Paper>
            </Stack>
          )}
        </Box>
      </Box>
    </Box>
  )
}
