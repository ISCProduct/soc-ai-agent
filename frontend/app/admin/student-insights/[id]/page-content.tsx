'use client'

import { Suspense, useCallback, useEffect, useState } from 'react'
import NextLink from 'next/link'
import { useSearchParams } from 'next/navigation'
import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Divider,
  Link as MuiLink,
  Paper,
  Stack,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'

// ES添削1件分。Backend の es_reviews をそのまま受ける（#1531）。
type ESReview = {
  id: number
  created_at: string
  question: string
  question_type: string
  company_name?: string | null
  specificity_score: number
  star_score: number
  length_balance_score: number
  company_fit_score?: number | null
  feedback: string
  improved_text: string
  es_text: string
}

const SCORE_LABELS: { key: keyof ESReview; label: string }[] = [
  { key: 'specificity_score', label: '具体性' },
  { key: 'star_score', label: 'STAR' },
  { key: 'length_balance_score', label: '文字数' },
  { key: 'company_fit_score', label: '企業適合' },
]

function formatDateTime(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return d.toLocaleString('ja-JP', {
    year: 'numeric', month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

// 担当校は一覧から URL で引き継ぐ。個別画面で学校を選び直させるのは不自然なうえ、
// 一覧で見ていた学校と別の学校を指定できてしまう。
export default function PageContent({ studentId }: { studentId: string }) {
  return (
    <Suspense fallback={null}>
      <Content studentId={studentId} />
    </Suspense>
  )
}

function Content({ studentId }: { studentId: string }) {
  const searchParams = useSearchParams()
  const rawSchoolId = searchParams.get('school_id')
  const schoolId = rawSchoolId && /^\d+$/.test(rawSchoolId) ? Number(rawSchoolId) : undefined
  const [schoolRequired, setSchoolRequired] = useState<boolean | null>(null)
  const [reviews, setReviews] = useState<ESReview[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    getAdminSchoolAccess()
      .then((access) => {
        if (!cancelled) setSchoolRequired(access.restricted)
      })
      .catch(() => {
        if (!cancelled) setSchoolRequired(true)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const load = useCallback(async (isCancelled?: () => boolean) => {
    if (schoolRequired === null) return
    // 担当校を持つ管理者は school_id が必須（無いと400）。確定するまで取得しない。
    if (schoolRequired && schoolId === undefined) return
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({
        ...(schoolId !== undefined ? { school_id: String(schoolId) } : {}),
      })
      const res = await fetch(`/api/admin/teacher/students/${studentId}/es-reviews?${params}`, {
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) throw new Error('ES添削の履歴を取得できませんでした')
      const data = await res.json()
      if (isCancelled?.()) return
      setReviews(data.es_reviews ?? [])
    } catch (e: unknown) {
      if (isCancelled?.()) return
      setReviews([])
      setError(e instanceof Error ? e.message : 'ES添削の履歴を取得できませんでした')
    } finally {
      if (!isCancelled?.()) setLoading(false)
    }
  }, [studentId, schoolId, schoolRequired])

  useEffect(() => {
    let cancelled = false
    load(() => cancelled)
    return () => {
      cancelled = true
    }
  }, [load])

  return (
    <Box sx={{ p: { xs: 2, sm: 3 }, maxWidth: 960, mx: 'auto' }}>
      <MuiLink component={NextLink} href="/admin/student-insights" sx={{ fontSize: 14 }}>
        ← 学生一覧へ戻る
      </MuiLink>
      <Typography variant="h5" sx={{ fontWeight: 700, mt: 2, mb: 3 }}>
        ES添削の履歴
      </Typography>

      {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

      {loading ? (
        <Stack alignItems="center" sx={{ py: 6 }}><CircularProgress /></Stack>
      ) : reviews.length === 0 ? (
        <Alert severity="info">
          この学生のES添削の履歴はまだありません。担当校外・他学園の学生の履歴は表示されません。
        </Alert>
      ) : (
        <Stack spacing={2}>
          {reviews.map((r) => (
            <Paper key={r.id} elevation={0} sx={{ p: { xs: 2, sm: 3 }, borderRadius: 2, border: '1px solid', borderColor: 'divider' }}>
              <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" sx={{ mb: 1 }}>
                <Chip size="small" label={r.question_type} />
                {r.company_name && <Chip size="small" variant="outlined" label={r.company_name} />}
                <Typography sx={{ fontSize: 12, color: 'text.secondary', ml: 'auto' }}>
                  {formatDateTime(r.created_at)}
                </Typography>
              </Stack>

              {r.question && (
                <Typography sx={{ fontWeight: 600, mb: 1 }}>{r.question}</Typography>
              )}

              <Stack direction="row" spacing={2} flexWrap="wrap" sx={{ mb: 1.5 }}>
                {SCORE_LABELS.map(({ key, label }) => {
                  const v = r[key]
                  // 企業適合性は企業名を指定した添削でしか付かない。
                  // 未評価を 0 と出すと「低く評価された」に見える。
                  if (typeof v !== 'number') return null
                  return (
                    <Typography key={String(key)} sx={{ fontSize: 13, color: 'text.secondary' }}>
                      {label} <Box component="span" sx={{ fontWeight: 700, color: 'text.primary' }}>{v}</Box> / 10
                    </Typography>
                  )
                })}
              </Stack>

              <Divider sx={{ my: 1.5 }} />
              <Typography sx={{ fontSize: 13, color: 'text.secondary', whiteSpace: 'pre-wrap' }}>
                {r.feedback}
              </Typography>
            </Paper>
          ))}
        </Stack>
      )}
    </Box>
  )
}
