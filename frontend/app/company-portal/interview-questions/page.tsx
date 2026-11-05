'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
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
  TextField,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { companyAuthService } from '@/lib/company/auth'
import {
  companyInterviewQuestionService,
  type CompanyInterviewQuestion,
  type CompanyInterviewQuestionInput,
} from '@/lib/company/interview-questions'

const EMPTY_FORM: CompanyInterviewQuestionInput = {
  position: '',
  category: '',
  question_text: '',
  priority: 0,
  is_required: false,
}

export default function CompanyPortalInterviewQuestionsPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [isOwner, setIsOwner] = useState(false)
  const [questions, setQuestions] = useState<CompanyInterviewQuestion[]>([])
  const [loadError, setLoadError] = useState('')
  const [notice, setNotice] = useState('')
  const [saving, setSaving] = useState(false)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<CompanyInterviewQuestionInput>(EMPTY_FORM)
  const [pendingDelete, setPendingDelete] = useState<CompanyInterviewQuestion | null>(null)
  const [actionError, setActionError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setQuestions(await companyInterviewQuestionService.list())
      setLoadError('')
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : '面接質問を取得できませんでした')
    } finally {
      setLoading(false)
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
        setLoading(false)
      })
  }, [router, load])

  const openCreate = () => {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setActionError('')
    setDialogOpen(true)
  }

  const openEdit = (question: CompanyInterviewQuestion) => {
    setEditingId(question.id)
    setForm({
      position: question.position,
      category: question.category,
      question_text: question.question_text,
      priority: question.priority,
      is_required: question.is_required,
    })
    setActionError('')
    setDialogOpen(true)
  }

  const save = async () => {
    setSaving(true)
    setActionError('')
    try {
      if (editingId === null) {
        await companyInterviewQuestionService.create(form)
        setNotice('面接質問を追加しました')
      } else {
        await companyInterviewQuestionService.update(editingId, form)
        setNotice('面接質問を更新しました')
      }
      setDialogOpen(false)
      await load()
    } catch (error) {
      setActionError(error instanceof Error ? error.message : '保存できませんでした')
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    if (!pendingDelete) return
    setSaving(true)
    setActionError('')
    try {
      await companyInterviewQuestionService.remove(pendingDelete.id)
      setQuestions((current) => current.filter((question) => question.id !== pendingDelete.id))
      setPendingDelete(null)
      setNotice('面接質問を削除しました')
    } catch (error) {
      setActionError(error instanceof Error ? error.message : '削除できませんでした')
    } finally {
      setSaving(false)
    }
  }

  return (
    <PageContainer maxWidth={960}>
      <Stack
        direction={{ xs: 'column', sm: 'row' }}
        spacing={2}
        alignItems={{ sm: 'center' }}
        justifyContent="space-between"
        sx={{ mb: 3 }}
      >
        <Box>
          <Typography variant="h4" component="h1" fontWeight="bold" sx={{ mb: 1 }}>
            面接質問
          </Typography>
          <Typography variant="body2" color="text.secondary">
            自社の面接で確認したい質問を登録します。必須質問は面接で優先して聞かれます。
          </Typography>
        </Box>
        {isOwner && (
          <Button variant="contained" onClick={openCreate}>
            面接質問を追加
          </Button>
        )}
      </Stack>

      {loadError && (
        <Alert
          severity="error"
          sx={{ mb: 2 }}
          action={<Button color="inherit" size="small" onClick={() => void load()}>再読み込み</Button>}
        >
          面接質問を取得できませんでした。{loadError}
        </Alert>
      )}
      {notice && <Alert severity="success" sx={{ mb: 2 }} onClose={() => setNotice('')}>{notice}</Alert>}
      {actionError && !dialogOpen && !pendingDelete && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setActionError('')}>{actionError}</Alert>
      )}

      {loading ? (
        <ListSkeleton rows={3} label="面接質問を読み込んでいます" />
      ) : loadError ? null : questions.length === 0 ? (
        <Paper variant="outlined">
          <EmptyState
            title="面接質問がまだ登録されていません"
            description={
              isOwner
                ? '質問を登録すると、自社ならではの面接内容に調整できます。'
                : '企業管理者が質問を登録すると、ここに表示されます。'
            }
            action={isOwner ? <Button variant="contained" onClick={openCreate}>面接質問を追加</Button> : undefined}
          />
        </Paper>
      ) : (
        <Stack spacing={1.5}>
          {questions.map((question) => (
            <Paper
              component="article"
              variant="outlined"
              key={question.id}
              sx={{ p: { xs: 2, sm: 2.5 } }}
            >
              <Stack
                direction={{ xs: 'column', sm: 'row' }}
                spacing={1.5}
                alignItems={{ sm: 'flex-start' }}
                justifyContent="space-between"
              >
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Stack direction="row" spacing={1} useFlexGap flexWrap="wrap" sx={{ mb: 1 }}>
                    <Chip size="small" label={question.category} />
                    <Chip
                      size="small"
                      variant="outlined"
                      label={question.position || '全職種共通'}
                    />
                    {question.is_required && <Chip size="small" color="primary" label="必須" />}
                    <Chip size="small" variant="outlined" label={`優先度 ${question.priority}`} />
                  </Stack>
                  <Typography sx={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>
                    {question.question_text}
                  </Typography>
                </Box>
                {isOwner && (
                  <Stack direction="row" spacing={1}>
                    <Button size="small" onClick={() => openEdit(question)}>編集</Button>
                    <Button
                      size="small"
                      color="error"
                      onClick={() => {
                        setPendingDelete(question)
                        setActionError('')
                      }}
                    >
                      削除
                    </Button>
                  </Stack>
                )}
              </Stack>
            </Paper>
          ))}
        </Stack>
      )}

      <Dialog open={dialogOpen} onClose={() => !saving && setDialogOpen(false)} fullWidth maxWidth="sm">
        <DialogTitle>{editingId === null ? '面接質問を追加' : '面接質問を編集'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2} sx={{ pt: 1 }}>
            {actionError && <Alert severity="error">{actionError}</Alert>}
            <TextField
              label="質問カテゴリ"
              required
              fullWidth
              inputProps={{ maxLength: 100 }}
              value={form.category}
              onChange={(event) => setForm((current) => ({ ...current, category: event.target.value }))}
              helperText="例：志望動機、技術、チームでの経験"
            />
            <TextField
              label="対象職種"
              fullWidth
              inputProps={{ maxLength: 100 }}
              value={form.position}
              onChange={(event) => setForm((current) => ({ ...current, position: event.target.value }))}
              helperText="空欄の場合は全職種共通です"
            />
            <TextField
              label="質問文"
              required
              fullWidth
              multiline
              minRows={3}
              inputProps={{ maxLength: 2000 }}
              value={form.question_text}
              onChange={(event) => setForm((current) => ({ ...current, question_text: event.target.value }))}
            />
            <TextField
              label="優先度"
              type="number"
              inputProps={{ min: 0, max: 999 }}
              value={form.priority}
              onChange={(event) => setForm((current) => ({
                ...current,
                priority: Number.parseInt(event.target.value, 10) || 0,
              }))}
              helperText="小さい数ほど先に質問されます"
            />
            <FormControlLabel
              control={
                <Switch
                  checked={form.is_required}
                  onChange={(event) => setForm((current) => ({ ...current, is_required: event.target.checked }))}
                />
              }
              label="必須質問にする"
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)} disabled={saving}>キャンセル</Button>
          <Button
            variant="contained"
            onClick={() => void save()}
            disabled={saving || !form.category.trim() || !form.question_text.trim()}
          >
            {saving ? '保存中...' : '保存する'}
          </Button>
        </DialogActions>
      </Dialog>

      <Dialog open={Boolean(pendingDelete)} onClose={() => !saving && setPendingDelete(null)}>
        <DialogTitle>面接質問を削除しますか？</DialogTitle>
        <DialogContent>
          {actionError && <Alert severity="error" sx={{ mb: 2 }}>{actionError}</Alert>}
          <Typography>削除した質問は今後の面接で使用されません。</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setPendingDelete(null)} disabled={saving}>キャンセル</Button>
          <Button color="error" variant="contained" onClick={() => void remove()} disabled={saving}>
            {saving ? '削除中...' : '削除する'}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
