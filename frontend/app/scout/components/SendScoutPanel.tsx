'use client'

import { FormEvent, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  FormControl,
  InputLabel,
  MenuItem,
  Paper,
  Select,
  Stack,
  Typography,
} from '@mui/material'
import {
  MOCK_CANDIDATES,
  cooldownRemainingMs,
  formatCooldown,
  interpolateScoutBody,
  lastScoutToStudent,
  type ScoutState,
  type ScoutTemplate,
} from '@/lib/scout/store'

export function SendScoutPanel({
  state,
  onSend,
}: {
  state: ScoutState
  onSend: (userId: number, studentName: string, template: ScoutTemplate) => string | null
}) {
  const [userId, setUserId] = useState<number>(MOCK_CANDIDATES[0].id)
  const [templateId, setTemplateId] = useState<number>(state.templates[0]?.id ?? 0)
  const [notice, setNotice] = useState('')
  const [error, setError] = useState('')

  // テンプレート削除・追加後に選択が空振りしないように揃える
  useEffect(() => {
    if (state.templates.length === 0) {
      setTemplateId(0)
      return
    }
    if (!state.templates.some((t) => t.id === templateId)) {
      setTemplateId(state.templates[0].id)
    }
  }, [state.templates, templateId])

  const student = MOCK_CANDIDATES.find((c) => c.id === userId) ?? MOCK_CANDIDATES[0]
  const template = state.templates.find((t) => t.id === templateId)

  const remaining = useMemo(
    () => cooldownRemainingMs(lastScoutToStudent(state.scouts, student.id), Date.now()),
    [state.scouts, student.id],
  )
  const blocked = state.blockedCompanyIds.includes(state.companyName)

  const preview = template
    ? interpolateScoutBody(template.body, {
        studentName: student.name,
        companyName: state.companyName,
      })
    : ''

  const submit = (e: FormEvent) => {
    e.preventDefault()
    setNotice('')
    setError('')
    if (!template) {
      setError('テンプレートを選んでください')
      return
    }
    const result = onSend(student.id, student.name, template)
    if (result) {
      setError(result)
      return
    }
    setNotice(`${student.name}さんへスカウトを送りました`)
  }

  return (
    <Stack spacing={2} component="form" onSubmit={submit}>
      <Typography variant="body2" color="text.secondary">
        学生を選び、定型文を差し込んだ文面を確認してから送ります。
      </Typography>

      {blocked && (
        <Alert severity="warning">この企業からのスカウトは学生にブロックされています。</Alert>
      )}
      {!blocked && remaining > 0 && (
        <Alert severity="warning">
          同じ学生へ短期間に送りすぎないよう、再送は{formatCooldown(remaining)}制限されています。
        </Alert>
      )}
      {error && <Alert severity="error">{error}</Alert>}
      {notice && <Alert severity="success">{notice}</Alert>}

      <FormControl fullWidth>
        <InputLabel id="scout-student-label">送る学生</InputLabel>
        <Select
          labelId="scout-student-label"
          label="送る学生"
          value={student.id}
          onChange={(e) => setUserId(Number(e.target.value))}
        >
          {MOCK_CANDIDATES.map((c) => (
            <MenuItem key={c.id} value={c.id}>
              {c.name}（{c.school}）
            </MenuItem>
          ))}
        </Select>
      </FormControl>

      {state.templates.length === 0 ? (
        <Alert severity="info">先にテンプレートを作成してください。</Alert>
      ) : (
        <FormControl fullWidth>
          <InputLabel id="scout-template-label">テンプレート</InputLabel>
          <Select
            labelId="scout-template-label"
            label="テンプレート"
            value={template?.id ?? ''}
            onChange={(e) => setTemplateId(Number(e.target.value))}
          >
            {state.templates.map((t) => (
              <MenuItem key={t.id} value={t.id}>
                {t.title}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
      )}

      <Paper variant="outlined" sx={{ p: 2 }}>
        <Typography variant="subtitle2" gutterBottom>
          差し込み後の文面
        </Typography>
        {preview ? (
          <Typography variant="body1" sx={{ whiteSpace: 'pre-wrap' }}>
            {preview}
          </Typography>
        ) : (
          <Typography variant="body2" color="text.secondary">
            テンプレートを選ぶと、学生名と企業名が入った文面が表示されます。
          </Typography>
        )}
      </Paper>

      <Button
        type="submit"
        variant="contained"
        disabled={!template || blocked || remaining > 0}
      >
        スカウトを送る
      </Button>
    </Stack>
  )
}
