'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Paper,
  Stack,
  Switch,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableRow,
  TextField,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company-auth'
import { companyJobService, isPublished, type CompanyJob, type JobInput } from '@/lib/company-jobs'

const EMPTY_FORM: JobInput = {
  title: '',
  description: '',
  work_location: '',
  employment_type: '',
  min_salary: 0,
  max_salary: 0,
  remote_option: false,
}

export default function CompanyPortalJobsPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [isOwner, setIsOwner] = useState(false)
  const [jobs, setJobs] = useState<CompanyJob[]>([])
  const [companyPublished, setCompanyPublished] = useState(true)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<JobInput>(EMPTY_FORM)

  const load = useCallback(async () => {
    try {
      const res = await companyJobService.list()
      setJobs(res.jobs)
      setCompanyPublished(res.companyPublished)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '求人を取得できませんでした')
    }
  }, [])

  useEffect(() => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    companyAuthService
      .fetchMe()
      .then((me) => {
        setIsOwner(me.role === 'owner')
        return load()
      })
      .catch(() => {
        companyAuthService.logout()
        router.replace('/company-portal/sign-in')
      })
      .finally(() => setLoading(false))
  }, [router, load])

  const openCreate = () => {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  const openEdit = (job: CompanyJob) => {
    setEditingId(job.id)
    setForm({
      title: job.title,
      description: job.description,
      job_url: job.job_url,
      min_salary: job.min_salary,
      max_salary: job.max_salary,
      employment_type: job.employment_type,
      work_location: job.work_location,
      remote_option: job.remote_option,
    })
    setDialogOpen(true)
  }

  const save = async () => {
    setSaving(true)
    try {
      if (editingId === null) {
        await companyJobService.create(form)
      } else {
        await companyJobService.update(editingId, form)
      }
      setDialogOpen(false)
      await load()
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : '保存できませんでした')
    } finally {
      setSaving(false)
    }
  }

  const togglePublish = async (job: CompanyJob) => {
    setSaving(true)
    try {
      const res = await companyJobService.setPublished(job.id, !isPublished(job))
      setCompanyPublished(res.companyPublished)
      await load()
      setError('')
    } catch (e) {
      // 企業本体が未公開のときはここに理由が出る。
      setError(e instanceof Error ? e.message : '公開状態を変更できませんでした')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return <PageLoading message="求人を読み込んでいます..." />
  }

  return (
    <PageContainer maxWidth={1080}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography variant="h4" fontWeight="bold">
          求人管理
        </Typography>
        <Stack direction="row" spacing={1}>
          {isOwner && (
            <Button variant="contained" onClick={openCreate}>
              求人を作る
            </Button>
          )}
          <Button variant="outlined" onClick={() => router.push('/company-portal')}>
            ダッシュボードへ
          </Button>
        </Stack>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        公開した求人は学生の企業詳細に表示されます。削除はできません（応募が紐づくため、非公開にしてください）。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {!companyPublished && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          企業情報がまだ公開されていません。求人を公開しても学生には表示されない場合があります。
          企業情報の公開については運営にお問い合わせください。
        </Alert>
      )}

      {jobs.length === 0 ? (
        <Paper
          elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', p: 4 }}
        >
          <Typography variant="h6" gutterBottom>
            求人がまだありません
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            求人がないと学生からの応募は発生しません。職種名だけで作成できます。
          </Typography>
          {isOwner ? (
            <Button variant="contained" onClick={openCreate}>
              求人を作る
            </Button>
          ) : (
            <Typography variant="body2" color="text.secondary">
              求人の作成は管理者のみ行えます。
            </Typography>
          )}
        </Paper>
      ) : (
        <Paper
          elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', overflowX: 'auto' }}
        >
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>職種名</TableCell>
                <TableCell>勤務地</TableCell>
                <TableCell>年収</TableCell>
                <TableCell>状態</TableCell>
                {isOwner && <TableCell>操作</TableCell>}
              </TableRow>
            </TableHead>
            <TableBody>
              {jobs.map((job) => (
                <TableRow key={job.id} hover>
                  <TableCell>{job.title}</TableCell>
                  <TableCell>{job.work_location || '—'}</TableCell>
                  <TableCell>
                    {job.min_salary || job.max_salary
                      ? `${job.min_salary || '—'} 〜 ${job.max_salary || '—'} 万円`
                      : '—'}
                  </TableCell>
                  <TableCell>
                    <Chip
                      size="small"
                      color={isPublished(job) ? 'success' : 'default'}
                      label={isPublished(job) ? '公開中' : '下書き'}
                    />
                  </TableCell>
                  {isOwner && (
                    <TableCell>
                      <Stack direction="row" spacing={1}>
                        <Button size="small" variant="outlined" onClick={() => openEdit(job)}>
                          編集
                        </Button>
                        <Button
                          size="small"
                          variant="outlined"
                          disabled={saving}
                          onClick={() => void togglePublish(job)}
                        >
                          {isPublished(job) ? '非公開にする' : '公開する'}
                        </Button>
                      </Stack>
                    </TableCell>
                  )}
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      )}

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{editingId === null ? '求人を作る' : '求人を編集'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="職種名"
              required
              fullWidth
              value={form.title}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
            />
            <TextField
              label="仕事内容"
              fullWidth
              multiline
              minRows={3}
              value={form.description ?? ''}
              onChange={(e) => setForm({ ...form, description: e.target.value })}
            />
            <TextField
              label="勤務地"
              fullWidth
              value={form.work_location ?? ''}
              onChange={(e) => setForm({ ...form, work_location: e.target.value })}
            />
            <TextField
              label="雇用形態"
              fullWidth
              value={form.employment_type ?? ''}
              onChange={(e) => setForm({ ...form, employment_type: e.target.value })}
            />
            <Stack direction="row" spacing={2}>
              <TextField
                label="最低年収（万円）"
                type="number"
                fullWidth
                value={form.min_salary ?? 0}
                onChange={(e) => setForm({ ...form, min_salary: Number(e.target.value) })}
              />
              <TextField
                label="最高年収（万円）"
                type="number"
                fullWidth
                value={form.max_salary ?? 0}
                onChange={(e) => setForm({ ...form, max_salary: Number(e.target.value) })}
              />
            </Stack>
            <FormControlLabel
              control={
                <Switch
                  checked={form.remote_option ?? false}
                  onChange={(e) => setForm({ ...form, remote_option: e.target.checked })}
                />
              }
              label="リモート可"
            />
            <Typography variant="caption" color="text.secondary">
              作成した求人は下書きになります。公開は一覧から行ってください。
            </Typography>
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>キャンセル</Button>
          <Button
            variant="contained"
            disabled={saving || !form.title.trim()}
            onClick={() => void save()}
          >
            保存
          </Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
