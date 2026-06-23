'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import AddIcon from '@mui/icons-material/Add'
import DeleteIcon from '@mui/icons-material/Delete'
import EditIcon from '@mui/icons-material/Edit'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

const CATEGORIES = ['志望動機', '職務経験', '技術', 'カルチャーフィット', '強み・弱み', 'キャリアビジョン', 'その他']

type Question = {
  id: number
  company_id: number
  position: string
  category: string
  question_text: string
  priority: number
  is_required: boolean
  created_at: string
  updated_at: string
}

type QuestionForm = {
  position: string
  category: string
  question_text: string
  priority: number
  is_required: boolean
}

const EMPTY_FORM: QuestionForm = {
  position: '',
  category: '志望動機',
  question_text: '',
  priority: 0,
  is_required: false,
}

export default function AdminInterviewQuestionsPage() {
  const params = useParams()
  const router = useRouter()
  const id = params.id as string

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [companyName, setCompanyName] = useState('')
  const [questions, setQuestions] = useState<Question[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingId, setEditingId] = useState<number | null>(null)
  const [form, setForm] = useState<QuestionForm>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [deleteConfirmId, setDeleteConfirmId] = useState<number | null>(null)

  const fetchJSON = async (url: string) => {
    const response = await fetch(url, { headers: authService.getAdminFetchHeaders() })
    if (!response.ok) {
      throw new Error(`Request failed: ${response.status}`)
    }
    return response.json()
  }

  const fetchQuestions = () => {
    setLoading(true)
    setError('')
    Promise.all([
      fetchJSON(`/api/admin/companies/${id}`),
      fetchJSON(`/api/admin/companies/${id}/interview-questions`),
    ])
      .then(([company, data]) => {
        setCompanyName(company.name || '')
        setQuestions(Array.isArray(data.questions) ? data.questions : [])
      })
      .catch(() => setError('データの取得に失敗しました'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { fetchQuestions() }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  const openCreate = () => {
    setEditingId(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  const openEdit = (q: Question) => {
    setEditingId(q.id)
    setForm({
      position: q.position,
      category: q.category,
      question_text: q.question_text,
      priority: q.priority,
      is_required: q.is_required,
    })
    setDialogOpen(true)
  }

  const handleSave = async () => {
    if (!form.category || !form.question_text.trim()) return
    setSaving(true)
    setError('')
    try {
      const url = editingId
        ? `/api/admin/companies/${id}/interview-questions/${editingId}`
        : `/api/admin/companies/${id}/interview-questions`
      const res = await fetch(url, {
        method: editingId ? 'PUT' : 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getAdminFetchHeaders() },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d?.message || '保存に失敗しました')
      }
      setSuccess(editingId ? '質問を更新しました' : '質問を追加しました')
      setDialogOpen(false)
      fetchQuestions()
    } catch (e: any) {
      setError(e.message)
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (qid: number) => {
    setError('')
    try {
      const res = await fetch(`/api/admin/companies/${id}/interview-questions/${qid}`, {
        method: 'DELETE',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) throw new Error('削除に失敗しました')
      setSuccess('質問を削除しました')
      setDeleteConfirmId(null)
      fetchQuestions()
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <AdminFormContainer
      title={`面接質問管理: ${companyName}`}
      maxWidth={960}
      backLabel="← 戻る"
      onBack={() => router.back()}
    >
      <ErrorAlert error={error} />
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>
          {success}
        </Alert>
      )}

      <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
        <Typography variant="body2" color="text.secondary">
          登録した質問はAI面接のシステムプロンプトに自動的に注入されます。
        </Typography>
        <Button variant="contained" startIcon={<AddIcon />} onClick={openCreate}>
          質問を追加
        </Button>
      </Box>

      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      ) : questions.length === 0 ? (
        <Paper elevation={0} sx={{ p: 4, textAlign: 'center', border: '1px dashed #e2e8f0' }}>
          <Typography color="text.secondary">質問が登録されていません。「質問を追加」から登録してください。</Typography>
        </Paper>
      ) : (
        <TableContainer component={Paper} elevation={0} sx={{ border: '1px solid #e2e8f0' }}>
          <Table size="small">
            <TableHead>
              <TableRow sx={{ bgcolor: '#f8fafc' }}>
                <TableCell sx={{ fontWeight: 700, width: 60 }}>優先度</TableCell>
                <TableCell sx={{ fontWeight: 700, width: 120 }}>カテゴリ</TableCell>
                <TableCell sx={{ fontWeight: 700, width: 140 }}>職種（空=全共通）</TableCell>
                <TableCell sx={{ fontWeight: 700 }}>質問文</TableCell>
                <TableCell sx={{ fontWeight: 700, width: 80 }}>必須</TableCell>
                <TableCell sx={{ fontWeight: 700, width: 100 }}>操作</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {questions.map((q) => (
                <TableRow key={q.id} hover>
                  <TableCell>{q.priority}</TableCell>
                  <TableCell>
                    <Chip label={q.category} size="small" variant="outlined" />
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" color={q.position ? 'text.primary' : 'text.disabled'}>
                      {q.position || '全職種'}
                    </Typography>
                  </TableCell>
                  <TableCell>
                    <Typography variant="body2" sx={{ lineHeight: 1.6 }}>{q.question_text}</Typography>
                  </TableCell>
                  <TableCell>
                    {q.is_required && (
                      <Chip label="必須" size="small" color="error" sx={{ fontWeight: 700 }} />
                    )}
                  </TableCell>
                  <TableCell>
                    <Stack direction="row" spacing={0.5}>
                      <Tooltip title="編集">
                        <IconButton size="small" onClick={() => openEdit(q)}>
                          <EditIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="削除">
                        <IconButton size="small" color="error" onClick={() => setDeleteConfirmId(q.id)}>
                          <DeleteIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </Stack>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
      )}

      {/* 作成/編集ダイアログ */}
      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>{editingId ? '質問を編集' : '質問を追加'}</DialogTitle>
        <DialogContent>
          <Stack spacing={2.5} sx={{ mt: 1 }}>
            <TextField
              select
              label="カテゴリ *"
              value={form.category}
              onChange={(e) => setForm(f => ({ ...f, category: e.target.value }))}
              size="small"
              fullWidth
            >
              {CATEGORIES.map(c => <MenuItem key={c} value={c}>{c}</MenuItem>)}
            </TextField>
            <TextField
              label="職種（空欄=全職種共通）"
              value={form.position}
              onChange={(e) => setForm(f => ({ ...f, position: e.target.value }))}
              size="small"
              fullWidth
              placeholder="例: バックエンドエンジニア"
              helperText="特定の職種のみに使う場合は入力してください"
            />
            <TextField
              label="質問文 *"
              value={form.question_text}
              onChange={(e) => setForm(f => ({ ...f, question_text: e.target.value }))}
              size="small"
              fullWidth
              multiline
              rows={3}
              placeholder="例: 直近のプロジェクトで最も難しかった技術的課題を教えてください。"
            />
            <TextField
              label="優先度（小さいほど先に使われる）"
              type="number"
              value={form.priority}
              onChange={(e) => setForm(f => ({ ...f, priority: parseInt(e.target.value) || 0 }))}
              size="small"
              sx={{ width: 200 }}
            />
            <FormControlLabel
              control={
                <Checkbox
                  checked={form.is_required}
                  onChange={(e) => setForm(f => ({ ...f, is_required: e.target.checked }))}
                />
              }
              label="必須質問（AIが必ず質問する）"
            />
          </Stack>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={() => setDialogOpen(false)}>キャンセル</Button>
          <Button
            variant="contained"
            onClick={handleSave}
            disabled={saving || !form.category || !form.question_text.trim()}
          >
            {saving ? '保存中...' : '保存'}
          </Button>
        </DialogActions>
      </Dialog>

      {/* 削除確認ダイアログ */}
      <Dialog open={deleteConfirmId !== null} onClose={() => setDeleteConfirmId(null)} maxWidth="xs" fullWidth>
        <DialogTitle>質問を削除しますか？</DialogTitle>
        <DialogContent>
          <Typography variant="body2">この操作は取り消せません。</Typography>
        </DialogContent>
        <DialogActions sx={{ px: 3, pb: 2 }}>
          <Button onClick={() => setDeleteConfirmId(null)}>キャンセル</Button>
          <Button
            variant="contained"
            color="error"
            onClick={() => deleteConfirmId && handleDelete(deleteConfirmId)}
          >
            削除
          </Button>
        </DialogActions>
      </Dialog>
    </AdminFormContainer>
  )
}
