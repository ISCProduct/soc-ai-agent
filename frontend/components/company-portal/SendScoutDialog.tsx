'use client'

import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@mui/material'
import {
  companyScoutService,
  interpolateScoutBody,
  type ScoutTemplate,
} from '@/lib/company/scouts'
import { companyProfileService } from '@/lib/company/profile'

export function SendScoutDialog({
  open,
  userId,
  studentName,
  onClose,
  onSent,
}: {
  open: boolean
  userId: number
  studentName: string
  onClose: () => void
  onSent?: () => void
}) {
  const [templates, setTemplates] = useState<ScoutTemplate[]>([])
  const [templateId, setTemplateId] = useState<number>(0)
  const [companyName, setCompanyName] = useState('貴社')
  const [remainingMs, setRemainingMs] = useState(0)
  const [loading, setLoading] = useState(false)
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    setError('')
    setNotice('')
    try {
      const [tpl, cool, profile] = await Promise.all([
        companyScoutService.listTemplates(),
        companyScoutService.cooldown(userId),
        companyProfileService.get().catch(() => null),
      ])
      setTemplates(tpl.items)
      setTemplateId(tpl.items[0]?.id ?? 0)
      setRemainingMs(cool.remaining_ms)
      if (profile?.name) setCompanyName(profile.name)
    } catch (e) {
      setError(e instanceof Error ? e.message : '準備に失敗しました')
    } finally {
      setLoading(false)
    }
  }, [userId])

  useEffect(() => {
    if (open) void load()
  }, [open, load])

  const template = templates.find((t) => t.id === templateId)
  const preview = useMemo(() => {
    if (!template) return ''
    return interpolateScoutBody(template.body, { studentName, companyName })
  }, [template, studentName, companyName])

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    if (!template) {
      setError('テンプレートを選んでください')
      return
    }
    setSending(true)
    setError('')
    setNotice('')
    try {
      await companyScoutService.send({
        userId,
        templateId: template.id,
        message: preview,
      })
      setNotice(`${studentName}さんへスカウトを送りました`)
      setRemainingMs(24 * 60 * 60 * 1000)
      onSent?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : '送信に失敗しました')
    } finally {
      setSending(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>スカウトを送る</DialogTitle>
      <DialogContent>
        <Stack spacing={2} component="form" id="send-scout-form" onSubmit={submit} sx={{ pt: 1 }}>
          <Typography variant="body2" color="text.secondary">
            {studentName}さん向けに、定型文を差し込んで送ります。
          </Typography>
          {loading && <Alert severity="info">読み込み中...</Alert>}
          {error && <Alert severity="error">{error}</Alert>}
          {notice && <Alert severity="success">{notice}</Alert>}
          {!loading && remainingMs > 0 && (
            <Alert severity="warning">
              同じ学生への再送は24時間空ける必要があります（クールダウン中）。
            </Alert>
          )}
          {!loading && templates.length === 0 ? (
            <Alert severity="info">
              先にテンプレートを作成してください。ダッシュボードの「スカウトテンプレート」から追加できます。
            </Alert>
          ) : (
            <FormControl fullWidth disabled={loading || templates.length === 0}>
              <InputLabel id="send-scout-template">テンプレート</InputLabel>
              <Select
                labelId="send-scout-template"
                label="テンプレート"
                value={template?.id ?? ''}
                onChange={(e) => setTemplateId(Number(e.target.value))}
              >
                {templates.map((t) => (
                  <MenuItem key={t.id} value={t.id}>
                    {t.title}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
          <Typography variant="subtitle2">差し込み後の文面</Typography>
          <Typography
            variant="body2"
            sx={{
              whiteSpace: 'pre-wrap',
              p: 2,
              bgcolor: 'action.hover',
              borderRadius: 1,
              minHeight: 120,
            }}
          >
            {preview || 'テンプレートを選ぶと文面が表示されます'}
          </Typography>
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>閉じる</Button>
        <Button
          type="submit"
          form="send-scout-form"
          variant="contained"
          disabled={sending || loading || !template || remainingMs > 0 || Boolean(notice)}
        >
          スカウトを送る
        </Button>
      </DialogActions>
    </Dialog>
  )
}
