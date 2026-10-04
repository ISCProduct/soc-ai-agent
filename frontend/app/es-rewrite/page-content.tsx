'use client'

import { Suspense, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
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
import { PRIMARY } from '@/app/interview/constants'
import { PageLoading } from '@/components/common/PageLoading'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'

const QUESTION_TYPES = ['志望動機', '自己PR', '学チカ', 'ガクチカ', 'その他']

// RAG側 models.ES_TEXT_MAX_LENGTH と合わせる。
// 10,000字だった頃は7,300字を超えた入力が必ず 422 になっていた（#1564）
const ES_TEXT_MAX_LENGTH = 6000

// RAG側 models.ES_CHAR_LIMIT_MIN / MAX と合わせる（設問の文字数上限 / #1523）
const CHAR_LIMIT_MIN = 100
const CHAR_LIMIT_MAX = 2000

/**
 * APIプロキシのエラーレスポンス（{ error, status, detail }想定）から
 * 日本語の短いメッセージを取り出す。生JSON/HTMLをそのまま表示しないため(#1015)。
 *
 * 422 のみ detail を優先する。RAGが利用者向けの具体的な案内（例: 「文章が長すぎて
 * 添削できませんでした。文字数を減らしてお試しください。」#1521）を detail に入れる一方、
 * api-proxy が error へ一般文「処理に失敗しました。しばらくしてから再試行してください。」を
 * 入れるため、error だけを読むと何をすれば直るのか分からなくなる。
 *
 * ただし detail は文字列とは限らない。api-proxy の getDetailText は本文に文字列 detail が
 * 無いとレスポンス本文そのものを入れるため（FastAPIのバリデーションエラーは detail が配列で、
 * しかも input にES全文が入る）、案内文の形をしているものだけを採用する。#1015 の
 * 「生JSONを画面に出さない」を壊さないための検査。
 */
const USER_FACING_DETAIL_MAX = 200

function userFacingDetail(detail: unknown): string | undefined {
  if (typeof detail !== 'string') return undefined
  const text = detail.trim()
  if (!text || text.length > USER_FACING_DETAIL_MAX) return undefined
  if (text.startsWith('{') || text.startsWith('[')) return undefined
  return text
}

async function readApiErrorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const data: unknown = await res.json()
    const { error, detail } = (data ?? {}) as { error?: unknown; detail?: unknown }
    const safeDetail = userFacingDetail(detail)
    if (res.status === 422 && safeDetail) return safeDetail
    if (typeof error === 'string' && error) return error
    if (safeDetail) return safeDetail
  } catch { /* ignore */ }
  return fallback
}

type StarBreakdown = {
  situation: string
  task: string
  action: string
  result: string
}

// 字数の結果(#1523)。ES添削・ESリライトで同じ形を返す（生成経路が同じ / #1533）
type CharLimitResult = {
  improved_text_length?: number
  // 指定字数に収まったか。上限未指定なら null
  char_limit_satisfied?: boolean | null
}

type RewriteResult = CharLimitResult & {
  rewritten_text: string
  star: StarBreakdown
}

type ReviewResult = CharLimitResult & {
  specificity_score: number
  star_score: number
  company_fit_score: number | null
  length_balance_score: number
  feedback: string
  improved_text: string
  company_strategy?: string | null
  // 企業情報の取得元。"none" なら企業適合性を評価していない(#1524)
  company_context_source?: 'company_brief' | 'cache' | 'web_search' | 'none'
}

// マーカーは S / T / A / R の頭文字を使う。
// 📍🎯⚡📊 を当てていたが、絵文字と項目の対応に意味が無く、
// STAR という枠組みそのものを隠していた。頭文字なら対応が自明。
const STAR_LABELS: { key: keyof StarBreakdown; label: string; color: string; initial: string }[] = [
  { key: 'situation', label: 'Situation（状況）', color: '#3b82f6', initial: 'S' },
  { key: 'task',      label: 'Task（課題）',      color: '#8b5cf6', initial: 'T' },
  { key: 'action',    label: 'Action（施策）',    color: PRIMARY,   initial: 'A' },
  { key: 'result',    label: 'Result（成果）',    color: '#10b981', initial: 'R' },
]

/**
 * 生成結果の字数表示(#1523)。
 *
 * 字数はサーバが数えた値をそのまま出す（改行と前後の空白を数えない数え方は
 * RAG 側の count_es_chars が唯一の定義。画面で数え直すと定義が二重になる）。
 */
function CharCountNote({ result, charLimit }: { result: CharLimitResult; charLimit: number | null }) {
  const length = result.improved_text_length
  if (typeof length !== 'number' || length <= 0) return null
  const overLimit = result.char_limit_satisfied === false
  return (
    <Box sx={{ mt: 1.5 }}>
      <Typography sx={{ fontSize: 13, fontWeight: 600, color: overLimit ? '#b45309' : '#64748b' }}>
        {charLimit === null ? `${length} 文字` : `${length} / ${charLimit} 文字`}
        （改行と前後の空白は数えません）
      </Typography>
      {overLimit && (
        <Alert severity="warning" role="status" sx={{ mt: 1, borderRadius: 2, fontSize: 13 }}>
          指定字数に収まりませんでした。勝手に切り詰めずそのまま出しているので、削る箇所を選んでから提出してください。もう一度実行すると収まることもあります。
        </Alert>
      )}
    </Box>
  )
}

type ScoreKey = 'specificity_score' | 'star_score' | 'company_fit_score' | 'length_balance_score'

const SCORE_ITEMS: { key: ScoreKey; label: string; color: string }[] = [
  { key: 'specificity_score',    label: '具体性',       color: '#3b82f6' },
  { key: 'star_score',           label: 'STAR法準拠',   color: '#8b5cf6' },
  { key: 'company_fit_score',    label: '企業適合性',   color: PRIMARY },
  { key: 'length_balance_score', label: '文字数バランス', color: '#10b981' },
]

function ESRewriteContent() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [mode, setMode] = useState<'rewrite' | 'review'>('rewrite')
  const [originalText, setOriginalText] = useState('')
  const [questionType, setQuestionType] = useState('学チカ')
  const [techStack, setTechStack] = useState('')
  const [companyName, setCompanyName] = useState(searchParams.get('company_name') || '')
  const [loading, setLoading] = useState(false)
  const [rewriteResult, setRewriteResult] = useState<RewriteResult | null>(null)
  const [reviewResult, setReviewResult] = useState<ReviewResult | null>(null)
  // 添削リクエストに使った企業名。入力欄はあとから編集できるため結果と一緒に保持する
  const [reviewedCompany, setReviewedCompany] = useState('')
  // 設問の文字数上限(#1523)。数値入力は空文字も扱うため文字列で持つ
  const [charLimit, setCharLimit] = useState('')
  const [charLimitMode, setCharLimitMode] = useState<'within' | 'around'>('within')
  // 実行時に使った上限。入力欄はあとから編集できるため結果と一緒に保持する
  const [requestedCharLimit, setRequestedCharLimit] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)

  const parsedCharLimit = charLimit.trim() === '' ? null : Number(charLimit)
  const charLimitInvalid =
    parsedCharLimit !== null &&
    (!Number.isInteger(parsedCharLimit) ||
      parsedCharLimit < CHAR_LIMIT_MIN ||
      parsedCharLimit > CHAR_LIMIT_MAX)

  // 上限が未入力/不正なら送らない（RAG側の既定＝上限なしに任せる）
  const charLimitBody =
    parsedCharLimit !== null && !charLimitInvalid
      ? { char_limit: parsedCharLimit, char_limit_mode: charLimitMode }
      : {}

  const handleRewrite = async () => {
    if (!originalText.trim()) return
    setLoading(true)
    setError(null)
    setRewriteResult(null)

    try {
      const res = await fetch('/api/es/rewrite', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          original_text: originalText,
          question_type: questionType,
          tech_stack: techStack,
          company_name: companyName,
          ...charLimitBody,
        }),
      })
      if (!res.ok) throw new Error(await readApiErrorMessage(res, 'リライトに失敗しました。再試行してください。'))
      setRequestedCharLimit(charLimitInvalid ? null : parsedCharLimit)
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
      const res = await fetch('/api/es/review', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          es_text: originalText,
          question_type: questionType,
          company_name: companyName,
          ...charLimitBody,
        }),
      })
      if (!res.ok) throw new Error(await readApiErrorMessage(res, '添削に失敗しました。再試行してください。'))
      setReviewedCompany(companyName.trim())
      setRequestedCharLimit(charLimitInvalid ? null : parsedCharLimit)
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
            {/*
              「AIがあなたのES文章を添削・リライトします」をやめた。
              誰が添削するかより、何が返ってくるかを書く。
            */}
            <Typography sx={{ fontSize: 12, color: '#64748b' }}>書いた文章を読み、直したほうがよい点と書き直し例を返します</Typography>
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
            ES添削
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
              // APIの上限は10,000字。超えた分を送るとRAGのバリデーションエラーになり、
              // 利用者には何が悪いのか伝わらないため入力段階で止める(#1521)
              slotProps={{ htmlInput: { maxLength: ES_TEXT_MAX_LENGTH } }}
              helperText={`${originalText.length} / ${ES_TEXT_MAX_LENGTH} 文字`}
              sx={{
                mb: 2.5,
                '& .MuiOutlinedInput-root': {
                  fontSize: 14, lineHeight: 1.8,
                  '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                  '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: PRIMARY },
                },
              }}
            />

            {/* 設問の文字数上限(#1523)。「400字以内」が前提のESに合わせる */}
            <Box sx={{ mb: 3 }}>
              <TextField
                fullWidth
                size="small"
                type="number"
                value={charLimit}
                onChange={e => setCharLimit(e.target.value)}
                label="設問の文字数上限（任意）"
                placeholder="例: 400"
                error={charLimitInvalid}
                helperText={
                  charLimitInvalid
                    ? `${CHAR_LIMIT_MIN}〜${CHAR_LIMIT_MAX} の整数で入力してください`
                    : '指定すると、書き直し後の文章をこの字数に収めます（未指定なら元の文章の長さを基準にします）'
                }
                slotProps={{
                  htmlInput: { min: CHAR_LIMIT_MIN, max: CHAR_LIMIT_MAX, step: 50, inputMode: 'numeric' },
                }}
                sx={{
                  '& .MuiOutlinedInput-root': {
                    '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: charLimitInvalid ? undefined : PRIMARY },
                    '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: charLimitInvalid ? undefined : PRIMARY },
                  },
                  '& .MuiInputLabel-root.Mui-focused': { color: charLimitInvalid ? undefined : PRIMARY },
                }}
              />
              {parsedCharLimit !== null && !charLimitInvalid && (
                <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 1.5 }}>
                  {([
                    { value: 'within', label: `${parsedCharLimit}字以内` },
                    { value: 'around', label: `${parsedCharLimit}字程度` },
                  ] as const).map(({ value, label }) => (
                    <Chip
                      key={value}
                      label={label}
                      onClick={() => setCharLimitMode(value)}
                      sx={{
                        cursor: 'pointer', fontWeight: 600,
                        bgcolor: charLimitMode === value ? PRIMARY : '#f1f5f9',
                        color: charLimitMode === value ? '#fff' : '#475569',
                        '&:hover': { bgcolor: charLimitMode === value ? `${PRIMARY}e0` : '#e2e8f0' },
                      }}
                    />
                  ))}
                </Box>
              )}
            </Box>

            {/* 添削モード: 志望企業 / リライトモード: 技術スタック */}
            {mode === 'review' ? (
              <>
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
                {companyName.trim() !== '' && loading && (
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
                    <CircularProgress size={16} />
                    <Typography variant="body2" sx={{ color: '#64748b' }}>企業情報を分析中...</Typography>
                  </Box>
                )}
              </>
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
                {companyName.trim() !== '' && loading && (
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
                    <CircularProgress size={16} />
                    <Typography variant="body2" sx={{ color: '#64748b' }}>企業情報を分析中...</Typography>
                  </Box>
                )}
              </>
            )}

            <Button
              variant="contained"
              fullWidth
              size="large"
              disabled={!originalText.trim() || loading || charLimitInvalid}
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
                : (mode === 'review' ? '添削する' : '書き直す')
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
                    // 未評価（企業情報なし）の項目は行ごと出さない。空のバーは 0/10 に見え、
                    // 「低く評価された」と誤解させるため(#1524)。理由はスコアの下でまとめて案内する。
                    // null だけでなく undefined も弾く（値が欠けたレスポンスで NaN のバーを出さない）
                    if (score == null) return null
                    const pct = (score / 10) * 100
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
                {reviewResult.company_fit_score == null && (
                  !reviewedCompany ? (
                    <Typography sx={{ mt: 2, fontSize: 13, color: '#64748b', lineHeight: 1.8 }}>
                      企業名を入力して添削すると、企業適合性も評価します。
                    </Typography>
                  ) : (
                    // 「企業情報を取得できたか」は company_fit_score ではなく company_context_source で判断する。
                    // 取得できていてもスコアだけ算出できないことがあり（企業情報を根拠にした対策アドバイスは
                    // 出ている）、そこで「公開情報が見つかりませんでした」と出すと画面が矛盾する(#1524)。
                    <Alert
                      severity="info"
                      role="status"
                      sx={{ mt: 2, borderRadius: 2, fontSize: 13 }}
                    >
                      <Typography component="h3" sx={{ fontSize: 13, fontWeight: 700, mb: 0.5 }}>
                        {(reviewResult.company_context_source ?? 'none') === 'none'
                          ? '企業情報を取得できなかったため、企業適合性は評価していません'
                          : '今回は企業適合性の点数を算出できませんでした'}
                      </Typography>
                      <Typography sx={{ fontSize: 13, lineHeight: 1.8 }}>
                        {(reviewResult.company_context_source ?? 'none') === 'none'
                          ? `「${reviewedCompany}」の公開情報が見つかりませんでした。根拠のない点数は出さないようにしています。上の項目とフィードバックは通常どおり評価済みです。企業名を正式名称（例: 株式会社◯◯）で入力し直すと、企業適合性も評価できる場合があります。`
                          : `「${reviewedCompany}」の情報は参照できましたが、点数としてまとめられませんでした。上の項目とフィードバック、企業ごとの対策アドバイスは通常どおり利用できます。点数も知りたい場合は、もう一度「添削する」を押してください。`}
                      </Typography>
                    </Alert>
                  )
                )}
              </Paper>

              {/* Feedback */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 1.5 }}>フィードバック</Typography>
                <Typography variant="body2" sx={{ color: '#475569', lineHeight: 1.8 }}>
                  {reviewResult.feedback}
                </Typography>
              </Paper>

              {reviewResult.company_strategy && (
                <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #f1f5f9', bgcolor: '#fff' }}>
                  <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 1.5 }}>🏢 {reviewedCompany || '志望企業'}への対策アドバイス</Typography>
                  <Typography variant="body2" sx={{ color: '#475569', lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>{reviewResult.company_strategy}</Typography>
                </Paper>
              )}

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
                  <Typography sx={{ fontSize: 14, lineHeight: 1.9, color: '#1e293b', whiteSpace: 'pre-wrap' }}>
                    {reviewResult.improved_text}
                  </Typography>
                  <CharCountNote result={reviewResult} charLimit={requestedCharLimit} />
                  <Divider sx={{ my: 2 }} />
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
                <CharCountNote result={rewriteResult} charLimit={requestedCharLimit} />
              </Paper>

              {/* STAR breakdown */}
              <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: '1px solid #e2e8f0', bgcolor: '#fff' }}>
                <Typography sx={{ fontWeight: 700, fontSize: 16, mb: 0.5 }}>STAR法 分解</Typography>
                {/* 学生には初見の用語なので、見出しの下で一度だけ説明する。 */}
                <Typography sx={{ fontSize: 13, color: '#64748b', mb: 2 }}>
                  「どんな状況で、何が課題で、何をして、どうなったか」の4つが書けているかを見ます。
                </Typography>
                <Stack spacing={2}>
                  {STAR_LABELS.map(({ key, label, color, initial }, idx) => (
                    <Box key={key}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.8 }}>
                        <Box sx={{
                          width: 28, height: 28, borderRadius: 1.5, bgcolor: `${color}15`,
                          display: 'flex', alignItems: 'center', justifyContent: 'center',
                          fontSize: 14, fontWeight: 700, color,
                        }}>
                          {initial}
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
      <BottomNavSpacer />
    </Box>
  )
}

export default function PageContent() {
  return (
    <Suspense fallback={<PageLoading message="ES添削・リライト画面を準備しています..." />}>
      <ESRewriteContent />
    </Suspense>
  )
}
