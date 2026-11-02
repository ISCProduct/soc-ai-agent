'use client'

import { useState } from 'react'
import {
  Alert,
  Button,
  Checkbox,
  FormControlLabel,
  IconButton,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import {
  AdminAssessmentError,
  createSchoolTemplate,
  validateDraft,
  type QuestionDraft,
  type TemplateDraft,
} from '@/lib/admin/assessment-templates'

const MAX_QUESTIONS = 60
const emptyQuestion = (): QuestionDraft => ({ body: '', weight_category: '', reverse_scored: false })

interface Props {
  schoolId: number
  categories: string[]
  onCreated: () => void
  onCancel: () => void
}

export function TemplateForm({ schoolId, categories, onCreated, onCancel }: Props) {
  const [draft, setDraft] = useState<TemplateDraft>({
    title: '',
    description: '',
    due_at: '',
    questions: [emptyQuestion(), emptyQuestion(), emptyQuestion()],
  })
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const setQuestion = (i: number, patch: Partial<QuestionDraft>) =>
    setDraft((d) => ({ ...d, questions: d.questions.map((q, j) => (j === i ? { ...q, ...patch } : q)) }))

  const submit = async () => {
    const invalid = validateDraft(draft)
    if (invalid) {
      setError(invalid)
      return
    }
    setSaving(true)
    setError(null)
    try {
      await createSchoolTemplate(schoolId, draft)
      onCreated()
    } catch (e) {
      // 入力不正はサーバーのメッセージを先生にそのまま見せる。それ以外は入力内容が残っていることを伝える。
      setError(
        e instanceof AdminAssessmentError && e.status === 400
          ? e.message
          : '作成できませんでした。入力した内容は残っています。時間をおいてもう一度お試しください',
      )
      setSaving(false)
    }
  }

  return (
    <Paper variant="outlined" sx={{ p: { xs: 2, sm: 3 } }} component="section" aria-labelledby="template-form-title">
      <Typography id="template-form-title" variant="h6" component="h2" fontWeight="bold" mb={1}>
        検査を作る
      </Typography>
      {/* 後から知ると「作ったのに効かない」という不信になるため、作成画面の最上部で明言する */}
      <Alert severity="warning" sx={{ mb: 2 }} data-testid="template-form-notice">
        ここで作った検査は<strong>おすすめ企業の計算には使われません</strong>。
        学校内で学生の傾向を知るための参考値です。学校ごとに問題が違うため、他校の結果とは比べられません。
      </Alert>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} role="alert" data-testid="template-form-error">
          {error}
        </Alert>
      )}

      <Stack spacing={2}>
        <TextField
          label="タイトル"
          required
          value={draft.title}
          onChange={(e) => setDraft({ ...draft, title: e.target.value })}
          slotProps={{ htmlInput: { maxLength: 100 } }}
        />
        <TextField
          label="説明（学生に表示されます）"
          multiline
          minRows={2}
          value={draft.description}
          onChange={(e) => setDraft({ ...draft, description: e.target.value })}
          slotProps={{ htmlInput: { maxLength: 500 } }}
        />
        <TextField
          label="受検期限（任意）"
          type="datetime-local"
          value={draft.due_at}
          onChange={(e) => setDraft({ ...draft, due_at: e.target.value })}
          slotProps={{ inputLabel: { shrink: true } }}
          helperText="期限を過ぎても受検はできます。先生の一覧で「期限超過」と表示されます。"
        />

        <Typography variant="subtitle1" component="h3" fontWeight="bold">
          問題（{draft.questions.length}問）
        </Typography>
        <Typography variant="body2" color="text.secondary" mt={-1}>
          学生は「あてはまる〜あてはまらない」の5段階で答えます。カテゴリは、その問題がどの傾向を測るかを選びます。
        </Typography>

        {draft.questions.map((q, i) => (
          <Paper key={i} variant="outlined" sx={{ p: 2 }}>
            <Stack spacing={1.5}>
              <Stack direction="row" alignItems="center" justifyContent="space-between">
                <Typography variant="body2" fontWeight="bold">
                  {i + 1}問目
                </Typography>
                <IconButton
                  size="small"
                  aria-label={`${i + 1}問目を削除`}
                  disabled={draft.questions.length <= 1}
                  onClick={() => setDraft((d) => ({ ...d, questions: d.questions.filter((_, j) => j !== i) }))}
                >
                  <DeleteOutlineIcon fontSize="small" />
                </IconButton>
              </Stack>
              <TextField
                label="問題文"
                required
                value={q.body}
                onChange={(e) => setQuestion(i, { body: e.target.value })}
                slotProps={{ htmlInput: { maxLength: 200 } }}
              />
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} alignItems={{ sm: 'center' }}>
                <TextField
                  select
                  required
                  label="測る傾向"
                  value={q.weight_category}
                  onChange={(e) => setQuestion(i, { weight_category: e.target.value })}
                  sx={{ minWidth: 220 }}
                >
                  {categories.map((c) => (
                    <MenuItem key={c} value={c}>
                      {c}
                    </MenuItem>
                  ))}
                </TextField>
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={q.reverse_scored}
                      onChange={(e) => setQuestion(i, { reverse_scored: e.target.checked })}
                    />
                  }
                  label="逆転項目（「あてはまる」がこの傾向と反対を表す問題）"
                />
              </Stack>
            </Stack>
          </Paper>
        ))}

        <Button
          variant="outlined"
          disabled={draft.questions.length >= MAX_QUESTIONS}
          onClick={() => setDraft((d) => ({ ...d, questions: [...d.questions, emptyQuestion()] }))}
          sx={{ alignSelf: 'flex-start' }}
        >
          問題を追加
        </Button>

        <Stack direction={{ xs: 'column-reverse', sm: 'row' }} spacing={1} justifyContent="flex-end">
          <Button onClick={onCancel} disabled={saving}>
            キャンセル
          </Button>
          <Button variant="contained" onClick={() => void submit()} disabled={saving}>
            {saving ? '作成しています…' : '検査を作成する'}
          </Button>
        </Stack>
      </Stack>
    </Paper>
  )
}
