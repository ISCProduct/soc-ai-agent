'use client'

import { useState } from 'react'
import { Box, Button, Card, CardContent, LinearProgress, Stack, TableBody, TableCell, TableHead, TableRow, Typography, useMediaQuery, useTheme } from '@mui/material'
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

const needsFollowUp = (s: StudentAssessmentStatus) => s.status === 'not_started' || s.overdue

export function StatusList({ data, loading }: Props) {
  const [followOnly, setFollowOnly] = useState(false)
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

  const rows = followOnly ? data.students.filter(needsFollowUp) : data.students
  const rollup = data.rollup

  const summary = (
    <Stack spacing={1} mb={1.5}>
      <Typography variant="body2">
        在籍{rollup.student_count}人。受検を始めた人は{rollup.started_count}人（{rollup.started_percent}%）、
        提出済みは{rollup.submitted_count}人（{rollup.submitted_percent}%）。
        期限を過ぎて未提出の人は{rollup.overdue_count}人です。
      </Typography>
      <LinearProgress
        variant="determinate"
        value={rollup.submitted_percent}
        aria-label={`提出済み ${rollup.submitted_percent}%`}
      />
      <Button size="small" variant="outlined" onClick={() => setFollowOnly((v) => !v)} sx={{ alignSelf: 'flex-start' }}>
        {followOnly ? '全員を見る' : '未受検と期限超過だけ見る'}
      </Button>
    </Stack>
  )

  if (narrow) {
    return (
      <Stack spacing={1.5}>
        {summary}
        {rows.length === 0 ? (
          <EmptyState title="該当する学生はいません" description="未受検と期限超過の学生はいません。" />
        ) : (
          <Stack spacing={1.5} component="ul" sx={{ listStyle: 'none', p: 0, m: 0 }} aria-label="学生ごとの受検状況">
        {rows.map((s) => {
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
        )}
      </Stack>
    )
  }

  return (
    <Box>
      {summary}
      {rows.length === 0 ? (
        <EmptyState title="該当する学生はいません" description="未受検と期限超過の学生はいません。" />
      ) : (
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
          {rows.map((s) => {
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
      )}
    </Box>
  )
}
