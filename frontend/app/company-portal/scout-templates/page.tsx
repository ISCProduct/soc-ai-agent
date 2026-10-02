'use client'

import { FormEvent, useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
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
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company/auth'
import { companyScoutService, type ScoutTemplate } from '@/lib/company/scouts'

export default function CompanyPortalScoutTemplatesPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [items, setItems] = useState<ScoutTemplate[]>([])
  const [error, setError] = useState('')
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [editingId, setEditingId] = useState<number | null>(null)
  const [formError, setFormError] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<ScoutTemplate | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await companyScoutService.listTemplates()
      setItems(res.items)
      setError('')
    } catch (e) {
      setError(e instanceof Error ? e.message : 'テンプレートを取得できませんでした')
    }
  }, [])

  useEffect(() => {
    if (!companyAuthService.getStoredUser()) {
      router.replace('/company-portal/sign-in')
      return
    }
    load().finally(() => setLoading(false))
  }, [router, load])

  const resetForm = () => {
    setTitle('')
    setBody('')
    setEditingId(null)
    setFormError('')
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setFormError('')
    try {
      if (editingId) {
        await companyScoutService.updateTemplate(editingId, title, body)
      } else {
        await companyScoutService.createTemplate(title, body)
      }
      resetForm()
      await load()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存に失敗しました')
    }
  }

  if (loading) return <PageLoading message="テンプレートを読み込んでいます..." />

  return (
    <PageContainer maxWidth={800}>
      <Button sx={{ mb: 2 }} onClick={() => router.push('/company-portal')}>
        ← ダッシュボードへ
      </Button>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        スカウトテンプレート
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        {'{{学生名}}'} と {'{{企業名}}'} は送信時に自動で入ります。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Box component="form" onSubmit={submit} sx={{ mb: 4 }}>
        <Stack spacing={2}>
          {formError && <Alert severity="error">{formError}</Alert>}
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

      {items.length === 0 ? (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography fontWeight="bold" gutterBottom>
            まだテンプレートがありません
          </Typography>
          <Typography variant="body2" color="text.secondary">
            上のフォームから1件保存すると、学生詳細から送信時に選べます。
          </Typography>
        </Paper>
      ) : (
        <Stack spacing={1.5}>
          {items.map((t) => (
            <Paper key={t.id} variant="outlined" sx={{ p: 2 }}>
              <Typography fontWeight="bold">{t.title}</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', mt: 1 }}>
                {t.body}
              </Typography>
              <Stack direction="row" spacing={1} sx={{ mt: 1.5 }}>
                <Button
                  size="small"
                  onClick={() => {
                    setEditingId(t.id)
                    setTitle(t.title)
                    setBody(t.body)
                  }}
                >
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
          <Typography>「{deleteTarget?.title}」を削除します。送信済みの文面は残ります。</Typography>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteTarget(null)}>キャンセル</Button>
          <Button
            color="error"
            variant="contained"
            onClick={async () => {
              if (!deleteTarget) return
              try {
                await companyScoutService.deleteTemplate(deleteTarget.id)
                if (editingId === deleteTarget.id) resetForm()
                setDeleteTarget(null)
                await load()
              } catch (e) {
                setError(e instanceof Error ? e.message : '削除に失敗しました')
                setDeleteTarget(null)
              }
            }}
          >
            削除する
          </Button>
        </DialogActions>
      </Dialog>
    </PageContainer>
  )
}
