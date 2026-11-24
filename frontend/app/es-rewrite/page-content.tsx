'use client'

import { Suspense, useMemo, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Box } from '@mui/material'
import { PageLoading } from '@/components/common/PageLoading'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { CONTENT_MAX_WIDTH, UI } from '@/lib/design/tokens'
import { createTextDiff, findUniqueTextRange } from './text-diff'
import { readApiErrorMessage } from './api-error'
import { CHAR_LIMIT_MAX, CHAR_LIMIT_MIN, type CharLimitMode, type EsMode, type ReviewResult, type RewriteResult } from './types'
import EsRewriteHeader from './components/EsRewriteHeader'
import ModeToggle from './components/ModeToggle'
import EsInputForm from './components/EsInputForm'
import ReviewResultPanel from './components/ReviewResultPanel'
import RewriteResultPanel from './components/RewriteResultPanel'

// ES添削・リライト画面。state と送信処理をここで持ち、表示は components/ の部品へ渡す（#2569）
function ESRewriteContent() {
  const router = useRouter()
  const searchParams = useSearchParams()

  const [mode, setMode] = useState<EsMode>('rewrite')
  const [originalText, setOriginalText] = useState('')
  const [questionType, setQuestionType] = useState('学チカ')
  // 設問文。同一設問の前回結果と突き合わせるキーになる(#1531)。
  // 種別だけだと「学チカ」と「ガクチカ」が別扱いになり、差分が永久に出ない。
  const [question, setQuestion] = useState('')
  const [techStack, setTechStack] = useState('')
  const [companyName, setCompanyName] = useState(searchParams.get('company_name') || '')
  const [loading, setLoading] = useState(false)
  const [rewriteResult, setRewriteResult] = useState<RewriteResult | null>(null)
  const [reviewResult, setReviewResult] = useState<ReviewResult | null>(null)
  const [reviewedOriginalText, setReviewedOriginalText] = useState('')
  // 添削リクエストに使った企業名。入力欄はあとから編集できるため結果と一緒に保持する
  const [reviewedCompany, setReviewedCompany] = useState('')
  // 設問の文字数上限(#1523)。数値入力は空文字も扱うため文字列で持つ
  const [charLimit, setCharLimit] = useState('')
  const [charLimitMode, setCharLimitMode] = useState<CharLimitMode>('within')
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
    const submittedText = originalText
    setLoading(true)
    setError(null)
    setReviewResult(null)

    try {
      const res = await fetch('/api/es/review', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          es_text: submittedText,
          question,
          question_type: questionType,
          company_name: companyName,
          ...charLimitBody,
        }),
      })
      if (!res.ok) throw new Error(await readApiErrorMessage(res, '添削に失敗しました。再試行してください。'))
      setReviewedCompany(companyName.trim())
      setReviewedOriginalText(submittedText)
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
  const reviewDiff = useMemo(
    () => reviewResult ? createTextDiff(reviewedOriginalText, reviewResult.improved_text) : null,
    [reviewResult, reviewedOriginalText],
  )
  const anchoredFeedback = useMemo(() => {
    if (!reviewResult) return []
    return (reviewResult.feedback_items ?? []).flatMap((item, index) => {
      const range = findUniqueTextRange(reviewedOriginalText, item.source_excerpt)
      if (!range) return []
      return [{
        ...item,
        index,
        ...range,
        anchorId: `es-original-anchor-${index}`,
      }]
    })
  }, [reviewResult, reviewedOriginalText])

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: UI.paper }}>
      {/* 帯と区切り線は全幅のまま、中身だけ本文と同じ列に入れる（#1783）。
          padding は列ラッパーに置き、行のボックスは入力欄と同じ幅にする。 */}
      <EsRewriteHeader onBack={() => router.push('/')} />

      {/* Mode toggle */}
      <ModeToggle
        mode={mode}
        onSelectReview={() => { setMode('review'); setRewriteResult(null); setError(null) }}
        onSelectRewrite={() => { setMode('rewrite'); setReviewResult(null); setError(null) }}
      />

      {/* Main */}
      <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', px: { xs: 2, md: 4 }, pb: 4 }}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', lg: result ? '1fr 1fr' : '1fr' },
            gap: 3,
            alignItems: 'start',
          }}
        >
          {/* ── Left: Input ── */}
          <EsInputForm
            mode={mode}
            questionType={questionType}
            setQuestionType={setQuestionType}
            question={question}
            setQuestion={setQuestion}
            originalText={originalText}
            setOriginalText={setOriginalText}
            charLimit={charLimit}
            setCharLimit={setCharLimit}
            parsedCharLimit={parsedCharLimit}
            charLimitInvalid={charLimitInvalid}
            charLimitMode={charLimitMode}
            setCharLimitMode={setCharLimitMode}
            techStack={techStack}
            setTechStack={setTechStack}
            companyName={companyName}
            setCompanyName={setCompanyName}
            loading={loading}
            error={error}
            onSubmit={mode === 'review' ? handleReview : handleRewrite}
          />

          {/* ── Right: Result ── */}
          {mode === 'review' && reviewResult && (
            <ReviewResultPanel
              reviewResult={reviewResult}
              reviewedOriginalText={reviewedOriginalText}
              reviewedCompany={reviewedCompany}
              requestedCharLimit={requestedCharLimit}
              reviewDiff={reviewDiff}
              anchoredFeedback={anchoredFeedback}
              copied={copied}
              onCopy={handleCopy}
            />
          )}

          {mode === 'rewrite' && rewriteResult && (
            <RewriteResultPanel
              rewriteResult={rewriteResult}
              originalText={originalText}
              requestedCharLimit={requestedCharLimit}
              copied={copied}
              onCopy={handleCopy}
            />
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
