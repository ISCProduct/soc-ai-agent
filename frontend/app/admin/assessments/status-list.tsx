'use client'

import { Box, Card, CardContent, Stack, TableBody, TableCell, TableHead, TableRow, Typography, useMediaQuery, useTheme } from '@mui/material'
import { AdminTableWrapper } from '@/components/admin/AdminTableWrapper'
import { EmptyState } from '@/components/common/EmptyState'
import { TableSkeletonRows, ListSkeleton } from '@/components/common/ListSkeleton'
import { STATUS_DISPLAY, formatDueAt } from '@/lib/assessments'
import type { StudentAssessmentStatus, StudentStatusList } from '@/lib/admin/assessment-templates'

/** 状態は色に頼らず、記号と文言の両方で示す。期限超過は受検状況より優先して見せる。 */
export function statusDisplay(s: StudentAssessmentStatus): { mark: string; label: string } {
  const base = STATUS_DISPLAY[s.status]
  return s.overdue ? { mark: '⚠', label: `期限超過（${base.label}）` } : base
}

const progress = (s: StudentAssessmentStatus) =>
  s.status === 'submitted' ? '全問回答' : `${s.answered_count} / ${s.question_count}問`

const topCategories = (s: StudentAssessmentStatus) =>
  s.status === 'submitted' && s.top_categories.length > 0 ? s.top_categories.join('、') : '—'

const submittedAt = (s: StudentAssessmentStatus) => (s.submitted_at ? formatDueAt(s.submitted_at) : '—')

interface Props {
  data: StudentStatusList | null
  loading: boolean
}

// 列は5つに留める。点数は並べない（成績表として読まれ、学生が正直に答えなくなるため）。
const COLUMNS = 5

export function StatusList({ data, loading }: Props) {
  // 375px では表を縦のカードに切り替える（横スクロールさせない）。
  const narrow = useMediaQuery(useTheme().breakpoints.down('sm'))

  if (loading) {
    return narrow ? (
      <ListSkeleton rows={3} label="受検状況を読み込み中" />
    ) : (
      <AdminTableWrapper>
        <TableBody>
          <TableSkeletonRows rows={5} columns={COLUMNS} label="受検状況を読み込み中" />
        </TableBody>
      </AdminTableWrapper>
    )
  }
  if (!data || data.students.length === 0) {
    return (
      <EmptyState
        title="対象の学生がいません"
        description="この学校に在籍する学生が登録されると、ここに受検状況が表示されます。"
      />
    )
  }

  if (narrow) {
    return (
      <Stack spacing={1.5} component="ul" sx={{ listStyle: 'none', p: 0, m: 0 }} aria-label="学生ごとの受検状況">
        {data.students.map((s) => {
          const st = statusDisplay(s)
          return (
            <Card key={s.student_id} variant="outlined" component="li">
              <CardContent sx={{ '&:last-child': { pb: 2 } }}>
                <Stack direction="row" justifyContent="space-between" gap={1}>
                  <Typography fontWeight="bold">{s.name}</Typography>
                  <Typography variant="body2" fontWeight="bold">
                    <span aria-hidden="true">{st.mark} </span>
                    {st.label}
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary" mt={0.5}>
                  進捗: {progress(s)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  傾向の強い項目: {topCategories(s)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  提出: {submittedAt(s)}
                </Typography>
              </CardContent>
            </Card>
          )
        })}
      </Stack>
    )
  }

  return (
    <Box>
      <AdminTableWrapper tableProps={{ 'aria-label': '学生ごとの受検状況' }}>
        <TableHead>
          <TableRow>
            <TableCell>氏名</TableCell>
            <TableCell>状態</TableCell>
            <TableCell>進捗</TableCell>
            <TableCell>提出</TableCell>
            <TableCell>傾向の強い項目</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {data.students.map((s) => {
            const st = statusDisplay(s)
            return (
              <TableRow key={s.student_id}>
                <TableCell>{s.name}</TableCell>
                <TableCell sx={{ fontWeight: 'bold' }}>
                  <span aria-hidden="true">{st.mark} </span>
                  {st.label}
                </TableCell>
                <TableCell>{progress(s)}</TableCell>
                <TableCell>{submittedAt(s)}</TableCell>
                <TableCell>{topCategories(s)}</TableCell>
              </TableRow>
            )
          })}
        </TableBody>
      </AdminTableWrapper>
    </Box>
  )
}
