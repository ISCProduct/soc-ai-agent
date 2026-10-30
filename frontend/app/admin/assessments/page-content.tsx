'use client'

import { useCallback, useEffect, useState } from 'react'
import { Alert, Button, Card, CardContent, Paper, Stack, Typography } from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { SchoolFilterSelect } from '@/components/admin/SchoolFilterSelect'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { formatDueAt } from '@/lib/assessments'
import {
  listSchoolTemplates,
  listStudentStatuses,
  STATUS_SORT_HINT,
  type SchoolTemplate,
  type StudentStatusList,
} from '@/lib/admin/assessment-templates'
import { StatusList } from './status-list'
import { TemplateForm } from './template-form'

export default function PageContent() {
  const [schoolId, setSchoolId] = useState<number | undefined>(undefined)
  const [templates, setTemplates] = useState<SchoolTemplate[]>([])
  const [categories, setCategories] = useState<string[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [creating, setCreating] = useState(false)
  const [selected, setSelected] = useState<number | null>(null)
  const [statuses, setStatuses] = useState<StudentStatusList | null>(null)
  const [statusLoading, setStatusLoading] = useState(false)
  const [statusError, setStatusError] = useState<string | null>(null)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin && !user?.is_staff) window.location.href = '/'
  }, [])

  const load = useCallback(async () => {
    if (schoolId === undefined) return
    setLoading(true)
    setError(null)
    try {
      const data = await listSchoolTemplates(schoolId)
      setTemplates(data.templates ?? [])
      setCategories(data.categories ?? [])
    } catch {
      setError('検査の一覧を読み込めませんでした')
    } finally {
      setLoading(false)
    }
  }, [schoolId])

  useEffect(() => {
    setSelected(null)
    setStatuses(null)
    setCreating(false)
    void load()
  }, [load])

  const openStatuses = async (templateId: number) => {
    if (schoolId === undefined) return
    setSelected(templateId)
    setStatuses(null)
    setStatusLoading(true)
    setStatusError(null)
    try {
      setStatuses(await listStudentStatuses(schoolId, templateId))
    } catch {
      setStatusError('受検状況を読み込めませんでした')
    } finally {
      setStatusLoading(false)
    }
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.wide}>
      <AdminPageHeader
        title="学校独自の適性検査"
        description="学校ごとの検査を作り、学生の受検状況を確認します。"
        backHref="/admin"
        actions={
          schoolId !== undefined && !creating ? (
            <Button variant="contained" onClick={() => setCreating(true)}>
              検査を作る
            </Button>
          ) : undefined
        }
      />

      <Alert severity="info" sx={{ mb: 2 }}>
        学校独自の検査の結果は<strong>おすすめ企業の計算には使われません</strong>。
        学生ごとの点数は表示せず、傾向の強い項目名だけを参考として表示します。
      </Alert>

      <Stack direction="row" spacing={2} mb={2}>
        <SchoolFilterSelect value={schoolId} onChange={setSchoolId} />
      </Stack>

      {schoolId === undefined ? (
        <Paper>
          <EmptyState title="学校を選んでください" description="学校を選ぶと、その学校の検査と受検状況が表示されます。" />
        </Paper>
      ) : (
        <Stack spacing={3}>
          {creating && (
            <TemplateForm
              schoolId={schoolId}
              categories={categories}
              onCancel={() => setCreating(false)}
              onCreated={() => {
                setCreating(false)
                void load()
              }}
            />
          )}

          {error && (
            <Alert severity="error" action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}>
              {error}
            </Alert>
          )}

          {loading ? (
            <ListSkeleton rows={2} label="検査を読み込み中" />
          ) : templates.length === 0 && !error && !creating ? (
            <Paper>
              <EmptyState
                title="学校独自の検査はまだありません"
                description="「検査を作る」から、学生に受けてもらう検査を用意できます。"
                action={<Button variant="contained" onClick={() => setCreating(true)}>検査を作る</Button>}
              />
            </Paper>
          ) : (
            <Stack spacing={1.5} component="ul" sx={{ listStyle: 'none', p: 0, m: 0 }} aria-label="学校独自の検査">
              {templates.map((t) => (
                <Card key={t.template_id} variant="outlined" component="li">
                  <CardContent>
                    <Stack direction={{ xs: 'column', sm: 'row' }} justifyContent="space-between" gap={1}>
                      <div>
                        <Typography variant="h6" component="h2" fontWeight="bold">
                          {t.title}
                        </Typography>
                        <Typography variant="body2" color="text.secondary">
                          全{t.question_count}問
                          {t.due_at ? ` ／ 受検期限 ${formatDueAt(t.due_at)}` : ' ／ 期限なし'}
                        </Typography>
                      </div>
                      <Button
                        variant={selected === t.template_id ? 'contained' : 'outlined'}
                        onClick={() => void openStatuses(t.template_id)}
                        sx={{ alignSelf: { sm: 'center' } }}
                      >
                        受検状況を見る
                      </Button>
                    </Stack>
                  </CardContent>
                </Card>
              ))}
            </Stack>
          )}

          {selected !== null && (
            <section aria-labelledby="status-title">
              <Typography id="status-title" variant="h6" component="h2" fontWeight="bold" mb={0.5}>
                受検状況{statuses ? `：${statuses.title}` : ''}
              </Typography>
              <Typography variant="body2" color="text.secondary" mb={1.5}>
                {STATUS_SORT_HINT}
              </Typography>
              {statusError ? (
                <Alert
                  severity="error"
                  action={<Button color="inherit" size="small" onClick={() => void openStatuses(selected)}>再読み込み</Button>}
                >
                  {statusError}
                </Alert>
              ) : (
                <StatusList data={statuses} loading={statusLoading} />
              )}
            </section>
          )}
        </Stack>
      )}
    </PageContainer>
  )
}
