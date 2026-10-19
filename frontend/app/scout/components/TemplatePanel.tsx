'use client'

import { FormEvent, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import type { ScoutState, ScoutTemplate } from '@/lib/scout/store'

export function TemplatePanel({
  state,
  onSave,
  onDelete,
}: {
  state: ScoutState
  onSave: (input: { id?: number; title: string; body: string }) => string | null
  onDelete: (id: number) => void
}) {
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [error, setError] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ScoutTemplate | null>(null)

  const resetForm = () => {
    setTitle('')
    setBody('')
    setEditingId(null)
    setError('')
  }

  const startEdit = (t: ScoutTemplate) => {
    setEditingId(t.id)
    setTitle(t.title)
    setBody(t.body)
    setError('')
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const result = onSave({
      id: editingId ?? undefined,
      title,
      body,
    })
    if (result) {
      setError(result)
      return
    }
    resetForm()
  }

  return (
    <Stack spacing={3}>
      <Typography variant="body2" color="text.secondary">
        よく使う文面を保存します。{'{{学生名}}'} と {'{{企業名}}'} は送信時に自動で入ります。
      </Typography>

      <Box component="form" onSubmit={submit}>
        <Stack spacing={2}>
          {error && <Alert severity="error">{error}</Alert>}
          <TextField
            label="タイトル"
            required
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            inputProps={{ maxLength: 80 }}
          />
          <TextField
            label="本文"
            required
            multiline
            minRows={5}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            helperText="例: {{学生名}} さんへ。{{企業名}}の採用担当です。"
          />
          <Stack direction="row" spacing={1}>
            <Button type="submit" variant="contained">
              {editingId ? 'テンプレートを更新' : 'テンプレートを保存'}
            </Button>
            {editingId && (
              <Button type="button" onClick={resetForm}>
                新規作成に戻る
              </Button>
            )}
          </Stack>
        </Stack>
      </Box>

      {state.templates.length === 0 ? (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography fontWeight="bold" gutterBottom>
            まだテンプレートがありません
          </Typography>
          <Typography variant="body2" color="text.secondary">
            上のフォームから1件保存すると、送信時に選べます。
          </Typography>
        </Paper>
      ) : (
        <Stack spacing={1.5}>
          {state.templates.map((t) => (
            <Paper key={t.id} variant="outlined" sx={{ p: 2 }}>
              <Typography fontWeight="bold">{t.title}</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', mt: 1 }}>
                {t.body}
              </Typography>
              <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
                <Button size="small" onClick={() => startEdit(t)}>
                  この文面を編集
                </Button>
                <Button size="small" color="error" onClick={() => setDeleteTarget(t)}>
                  削除
                </Button>
              </Stack>
            </Paper>
          ))}
        </Stack>
      )}

      <Dialog open={Boolean(deleteTarget)} onClose={() => setDeleteTarget(null)}>
        <DialogTitle>テンプレートを削除しますか</DialogTitle>
        <DialogContent>
          <Typography>
            「{deleteTarget?.title}」を削除します。送信済みのスカウト文面は残ります。
          </Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>キャンセル</Button>
          <Button
            color="error"
            variant="contained"
            onClick={() => {
              if (deleteTarget) onDelete(deleteTarget.id)
              setDeleteTarget(null)
              if (editingId === deleteTarget?.id) resetForm()
            }}
          >
            削除する
          </Button>
        </DialogActions>
      </Dialog>
    </Stack>
  )
}
