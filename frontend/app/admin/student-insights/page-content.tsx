'use client'

import { Fragment, useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Checkbox,
  Chip,
  Collapse,
  FormControlLabel,
  InputAdornment,
  Paper,
  Stack,
  TableBody,
  TableCell,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import { authService } from '@/lib/auth'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminTableWrapper } from '@/components/admin/AdminTableWrapper'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'
import { ScoreBar } from '@/components/admin/ScoreBar'
import { EmptyState } from '@/components/common/EmptyState'
import { TableSkeletonRows } from '@/components/common/ListSkeleton'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'
import {
  displayCategories,
  displayIndustries,
  displayTypeLabel,
  INACTIVE_DAYS,
  isInactive,
  lastActivityLabel,
  lowMatchApplications,
  resumeAttentionLabel,
  NO_DATA_LABEL,
  type StudentTendency,
} from '@/lib/student-insights'
import { LOW_MATCH_THRESHOLD } from '@/lib/low-match'
import { sendTeacherGuidance } from '@/lib/teacher-guidance'

const guidanceKey = (userId: number, kind: 'low_match' | 'resume') => `${userId}:${kind}`
const COLUMNS = 6

export default function PageContent() {
  const [adminEmail, setAdminEmail] = useState('')
  const [students, setStudents] = useState<StudentTendency[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [query, setQuery] = useState('')
  const [lowMatchOnly, setLowMatchOnly] = useState(false)
  const [resumeNeedsAttentionOnly, setResumeNeedsAttentionOnly] = useState(false)
  const [inactiveOnly, setInactiveOnly] = useState(false)
  const [expanded, setExpanded] = useState<number | null>(null)
  // 初回取得前は Empty ではなく Skeleton を出す
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  // 送信中・送信済は「生徒×案内種別」単位で持つ（同時送信や2種類目の案内を妨げない）
  const [sendingKeys, setSendingKeys] = useState<Set<string>>(() => new Set())
  const [sentKeys, setSentKeys] = useState<Set<string>>(() => new Set())
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  // 担当校を持つ管理者は school_id が必須(無いと403)なので、学校が確定するまで取得しない
  const [schoolRequired, setSchoolRequired] = useState<boolean | null>(null)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
    // 教員入口の「対応が必要な学生」から ?inactive_days=14 で来たら絞り込んだ状態で開く（#1573）。
    // 取得は adminEmail が入るまで走らないので、同じ描画で両方を確定させる。
    const params = new URLSearchParams(window.location.search)
    setInactiveOnly(params.has('inactive_days'))
    setLowMatchOnly(params.get('low_match_only') === 'true')
    setResumeNeedsAttentionOnly(params.get('resume_needs_attention_only') === 'true')
    setAdminEmail(user.email)
  }, [])

  useEffect(() => {
    let cancelled = false
    getAdminSchoolAccess()
      .then((access) => {
        if (!cancelled) setSchoolRequired(access.restricted)
      })
      .catch(() => {
        if (!cancelled) setSchoolRequired(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const fetchStudents = useCallback(async (isCancelled?: () => boolean) => {
    if (!adminEmail || schoolRequired === null) return
    if (schoolRequired && schoolId === undefined) return
    setLoading(true)
    setError('')
    try {
      const params = new URLSearchParams({
        limit: String(rowsPerPage),
        offset: String(page * rowsPerPage),
        ...(query ? { q: query } : {}),
        ...(schoolId !== undefined ? { school_id: String(schoolId) } : {}),
        ...(lowMatchOnly ? { low_match_only: 'true' } : {}),
        ...(resumeNeedsAttentionOnly ? { resume_needs_attention_only: 'true' } : {}),
        ...(inactiveOnly ? { inactive_days: String(INACTIVE_DAYS) } : {}),
      })
      const res = await fetch(`/api/admin/teacher/students/tendency-analysis?${params}`, {
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) throw new Error('傾向分析データの取得に失敗しました')
      const data = await res.json()
      if (isCancelled?.()) return
      setStudents(data.students ?? [])
      setTotal(data.total ?? 0)
    } catch (e: unknown) {
      if (isCancelled?.()) return
      setStudents([])
      setTotal(0)
      setError(e instanceof Error ? e.message : '傾向分析データの取得に失敗しました')
    } finally {
      if (!isCancelled?.()) setLoading(false)
    }
  }, [adminEmail, schoolRequired, schoolId, page, rowsPerPage, query, lowMatchOnly, resumeNeedsAttentionOnly, inactiveOnly])

  // 未活動の判定は描画のたびに同じ基準時刻で行う
  const [now] = useState(() => new Date())
  const filtered = Boolean(query) || lowMatchOnly || resumeNeedsAttentionOnly || inactiveOnly
  const clearFilters = () => {
    setQuery('')
    setLowMatchOnly(false)
    setResumeNeedsAttentionOnly(false)
    setInactiveOnly(false)
    setPage(0)
  }

  // 検索入力のたびに投げると古いレスポンスが新しい結果を上書きするため、デバウンス+キャンセルする
  useEffect(() => {
    let cancelled = false
    const timer = setTimeout(() => {
      fetchStudents(() => cancelled)
    }, query ? 400 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [fetchStudents, query])

  const sendGuidance = async (s: StudentTendency, kind: 'low_match' | 'resume') => {
    const key = guidanceKey(s.user_id, kind)
    setSendingKeys((prev) => new Set(prev).add(key))
    setError('')
    try {
      await sendTeacherGuidance(s.user_id, {
        kind,
        suggested_industries: displayIndustries(s).map((i) => i.industry_name),
      })
      setSentKeys((prev) => new Set(prev).add(key))
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '案内の送信に失敗しました')
    } finally {
      setSendingKeys((prev) => {
        const next = new Set(prev)
        next.delete(key)
        return next
      })
    }
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.wide}>
      <AdminPageHeader
        title="生徒の傾向分析"
        description="担当する生徒のスコア傾向を、タイプと向いている業界で一覧比較します。"
        backHref="/admin"
      />

      {/* 教育現場での誤用を防ぐため、断定的な評価ではないことを明示する */}
      <Alert severity="info" sx={{ mb: 2 }}>
        ここに表示されるタイプや業界は、これまでのスコアから算出した<strong>あくまで参考情報</strong>です。
        生徒の適性や進路を断定するものではありません。面談のきっかけとしてご活用ください。
      </Alert>

      {error && (
        <Alert severity="error" onClose={() => setError('')} sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* タブレット幅以下では折り返す（UI監査 R4 / #1573） */}
      <Stack direction="row" useFlexGap spacing={2} mb={2} sx={{ flexWrap: 'wrap', alignItems: 'center' }}>
        <TextField
          placeholder="氏名・メール・学校名で検索"
          value={query}
          onChange={(e) => { setQuery(e.target.value); setPage(0) }}
          size="small"
          sx={{ flex: '1 1 240px' }}
          InputProps={{
            startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>,
          }}
        />
        <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(0) }} />
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={inactiveOnly}
              onChange={(e) => { setInactiveOnly(e.target.checked); setPage(0) }}
            />
          }
          label={`最近${INACTIVE_DAYS}日間、活動がない生徒のみ`}
          slotProps={{ typography: { variant: 'body2' } }}
        />
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={lowMatchOnly}
              onChange={(e) => { setLowMatchOnly(e.target.checked); setPage(0) }}
            />
          }
          label={`マッチ度${LOW_MATCH_THRESHOLD}未満の応募がある生徒のみ`}
          slotProps={{ typography: { variant: 'body2' } }}
        />
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={resumeNeedsAttentionOnly}
              onChange={(e) => { setResumeNeedsAttentionOnly(e.target.checked); setPage(0) }}
            />
          }
          label="履歴書要対応の生徒のみ"
          slotProps={{ typography: { variant: 'body2' } }}
        />
      </Stack>

      <Paper elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
        <AdminTableWrapper>
          <TableHead>
            <TableRow sx={{ bgcolor: 'action.hover' }}>
              <TableCell>氏名</TableCell>
              <TableCell>最終活動</TableCell>
              <TableCell>タイプ</TableCell>
              <TableCell>気になる点</TableCell>
              <TableCell>学生への案内</TableCell>
              <TableCell>詳細</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableSkeletonRows rows={5} columns={COLUMNS} label="生徒の一覧を読み込み中" />
            ) : students.length === 0 ? (
              <TableRow>
                <TableCell colSpan={COLUMNS}>
                  {error ? null : filtered ? (
                    <EmptyState
                      title="条件に合う生徒はいません"
                      description="絞り込みを外すと、担当の生徒全員を表示します。"
                      action={<Button variant="outlined" onClick={clearFilters}>絞り込みを外す</Button>}
                    />
                  ) : (
                    <EmptyState
                      title="担当の生徒がまだいません"
                      description="生徒が学校のアカウントで登録すると、ここに並びます。"
                    />
                  )}
                </TableCell>
              </TableRow>
            ) : students.map((s) => {
              const categories = displayCategories(s)
              const industries = displayIndustries(s)
              const typeLabel = displayTypeLabel(s)
              const noData = typeLabel === NO_DATA_LABEL
              const lowMatches = lowMatchApplications(s)
              const resumeLabel = resumeAttentionLabel(s)
              const inactive = isInactive(s, now)
              const canGuideLowMatch = lowMatches.length > 0
              const canGuideResume = Boolean(resumeLabel)
              const lowMatchKey = guidanceKey(s.user_id, 'low_match')
              const resumeKey = guidanceKey(s.user_id, 'resume')
              const lowMatchSent = sentKeys.has(lowMatchKey)
              const resumeSent = sentKeys.has(resumeKey)
              const open = expanded === s.user_id
              const detailId = `student-detail-${s.user_id}`
              return (
                <Fragment key={s.user_id}>
                  <TableRow hover>
                    <TableCell>
                      <Typography fontWeight={500}>{s.name || '—'}</Typography>
                      <Typography variant="body2" color="text.secondary">{s.email}</Typography>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" fontWeight={inactive ? 700 : 400}>
                        {lastActivityLabel(s, now)}
                      </Typography>
                      {inactive && (
                        <Typography variant="caption" color="text.secondary">
                          {`最近${INACTIVE_DAYS}日間、活動がありません`}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Chip
                        label={typeLabel}
                        size="small"
                        variant={noData ? 'outlined' : 'filled'}
                        color={noData ? 'default' : 'primary'}
                      />
                    </TableCell>
                    <TableCell>
                      {lowMatches.length === 0 && !resumeLabel ? (
                        <Typography variant="body2" color="text.disabled">—</Typography>
                      ) : (
                        <Stack spacing={0.5} sx={{ minWidth: 180 }}>
                          {lowMatches.map((a, i) => (
                            <Chip
                              key={`${a.company_name}-${i}`}
                              label={`低マッチの応募: ${a.company_name}（${Math.round(a.match_score)}）`}
                              size="small"
                              color="warning"
                              variant="outlined"
                            />
                          ))}
                          {resumeLabel && (
                            <Chip label={`履歴書: ${resumeLabel}`} size="small" color="warning" variant="outlined" />
                          )}
                        </Stack>
                      )}
                    </TableCell>
                    <TableCell>
                      <Stack spacing={0.5} sx={{ minWidth: 180 }}>
                        {canGuideLowMatch && (
                          <Button
                            size="small"
                            variant="outlined"
                            disabled={sendingKeys.has(lowMatchKey) || lowMatchSent}
                            onClick={() => sendGuidance(s, 'low_match')}
                          >
                            {lowMatchSent ? '送信済（学生のホームに表示中）' : '応募先の見直しを案内する'}
                          </Button>
                        )}
                        {canGuideResume && (
                          <Button
                            size="small"
                            variant="outlined"
                            color="warning"
                            disabled={sendingKeys.has(resumeKey) || resumeSent}
                            onClick={() => sendGuidance(s, 'resume')}
                          >
                            {resumeSent ? '送信済（学生のホームに表示中）' : '履歴書の見直しを案内する'}
                          </Button>
                        )}
                        {!canGuideLowMatch && !canGuideResume && (
                          <Typography variant="body2" color="text.disabled">—</Typography>
                        )}
                      </Stack>
                    </TableCell>
                    <TableCell>
                      <Button
                        size="small"
                        aria-expanded={open}
                        aria-controls={detailId}
                        onClick={() => setExpanded(open ? null : s.user_id)}
                      >
                        {open ? '閉じる' : 'スコアを見る'}
                      </Button>
                    </TableCell>
                  </TableRow>
                  <TableRow>
                    <TableCell colSpan={COLUMNS} sx={{ py: 0, borderBottom: open ? undefined : 'none' }}>
                      <Collapse in={open} unmountOnExit>
                        <Stack id={detailId} direction={{ xs: 'column', md: 'row' }} spacing={4} sx={{ py: 2 }}>
                          <Stack spacing={0.75} sx={{ minWidth: 220 }}>
                            <Typography variant="subtitle2">上位カテゴリ</Typography>
                            {categories.length === 0 ? (
                              <Typography variant="body2" color="text.secondary">まだ計測されていません</Typography>
                            ) : categories.map((c) => (
                              <ScoreBar key={c.category} label={c.category} score={c.score} />
                            ))}
                          </Stack>
                          <Stack spacing={0.75} sx={{ minWidth: 220 }}>
                            <Typography variant="subtitle2">向いている業界 TOP3</Typography>
                            {industries.length === 0 ? (
                              <Typography variant="body2" color="text.secondary">まだ計測されていません</Typography>
                            ) : industries.map((i) => (
                              <ScoreBar key={i.industry_id} label={i.industry_name} score={i.score} color="secondary.main" />
                            ))}
                          </Stack>
                        </Stack>
                      </Collapse>
                    </TableCell>
                  </TableRow>
                </Fragment>
              )
            })}
          </TableBody>
        </AdminTableWrapper>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, p) => setPage(p)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={(e) => { setRowsPerPage(Number(e.target.value)); setPage(0) }}
          rowsPerPageOptions={[10, 25, 50]}
          labelRowsPerPage="表示件数:"
          labelDisplayedRows={({ from, to, count }) => `${from}–${to} / ${count}`}
        />
      </Paper>
    </PageContainer>
  )
}
