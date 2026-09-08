'use client'

import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Chip,
  CircularProgress,
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
import { getAdminSchoolAccess } from '@/lib/admin-school-access'
import {
  displayCategories,
  displayIndustries,
  displayTypeLabel,
  formatScore,
  NO_DATA_LABEL,
  type StudentTendency,
} from '@/lib/student-insights'

export default function PageContent() {
  const [adminEmail, setAdminEmail] = useState('')
  const [students, setStudents] = useState<StudentTendency[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [rowsPerPage, setRowsPerPage] = useState(25)
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  // 担当校を持つ管理者は school_id が必須(無いと403)なので、学校が確定するまで取得しない
  const [schoolRequired, setSchoolRequired] = useState<boolean | null>(null)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
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
  }, [adminEmail, schoolRequired, schoolId, page, rowsPerPage, query])

  // 検索入力のたびに投げると古いレスポンスが新しい結果を上書きするため、デバウンス+キャンセルする
  useEffect(() => {
    let cancelled = false
    const timer = setTimeout(() => {
      fetchStudents(() => cancelled)
    }, query ? 400 : 0)
    return () => { cancelled = true; clearTimeout(timer) }
  }, [fetchStudents, query])

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

      <Stack direction="row" spacing={2} mb={2}>
        <TextField
          placeholder="氏名・メール・学校名で検索"
          value={query}
          onChange={(e) => { setQuery(e.target.value); setPage(0) }}
          size="small"
          sx={{ flex: 1 }}
          InputProps={{
            startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment>,
          }}
        />
        <SchoolFilterSelect value={schoolId} onChange={(id) => { setSchoolId(id); setPage(0) }} />
      </Stack>

      <Paper elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
        <AdminTableWrapper>
          <TableHead>
            <TableRow sx={{ bgcolor: '#f5f5f5' }}>
              <TableCell>氏名</TableCell>
              <TableCell>メール</TableCell>
              <TableCell>タイプ</TableCell>
              <TableCell>上位カテゴリ</TableCell>
              <TableCell>向いている業界 TOP3</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {loading ? (
              <TableRow>
                <TableCell colSpan={5} align="center" sx={{ py: 4 }}>
                  <CircularProgress size={24} />
                </TableCell>
              </TableRow>
            ) : students.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} align="center" sx={{ py: 4, color: 'text.secondary' }}>
                  生徒が見つかりません
                </TableCell>
              </TableRow>
            ) : students.map((s) => {
              const categories = displayCategories(s)
              const industries = displayIndustries(s)
              const typeLabel = displayTypeLabel(s)
              const noData = typeLabel === NO_DATA_LABEL
              return (
                <TableRow key={s.user_id} hover>
                  <TableCell>
                    <Typography fontWeight={500}>{s.name || '—'}</Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color="text.secondary">{s.email}</Typography>
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
                    {categories.length === 0 ? (
                      <Typography variant="body2" color="text.disabled">—</Typography>
                    ) : (
                      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                        {categories.map((c) => (
                          <Chip
                            key={c.category}
                            label={`${c.category} ${formatScore(c.score)}`}
                            size="small"
                            variant="outlined"
                          />
                        ))}
                      </Stack>
                    )}
                  </TableCell>
                  <TableCell>
                    {industries.length === 0 ? (
                      <Typography variant="body2" color="text.disabled">—</Typography>
                    ) : (
                      <Stack direction="row" spacing={0.5} flexWrap="wrap" useFlexGap>
                        {industries.map((i) => (
                          <Chip
                            key={i.industry_id}
                            label={`${i.industry_name} ${formatScore(i.score)}`}
                            size="small"
                            variant="outlined"
                            color="secondary"
                          />
                        ))}
                      </Stack>
                    )}
                  </TableCell>
                </TableRow>
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
