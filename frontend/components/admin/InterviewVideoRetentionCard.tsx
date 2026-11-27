'use client'

import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  CardContent,
  MenuItem,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess, type AdminSchool } from '@/lib/admin/school-access'

const OPTIONS = [
  { value: '30', label: '30日' },
  { value: '90', label: '90日（既定）' },
  { value: '180', label: '180日' },
  { value: 'unlimited', label: '期限なし' },
] as const

function toChoice(days: number | null): string {
  if (days == null) return 'unlimited'
  return String(days)
}

/**
 * 学校管理者が、面接動画を残す期間を選ぶ（SOCAIAGENT-671）。
 * 職員は今の値だけ見て、変更はできない。
 */
export function InterviewVideoRetentionCard() {
  const [schools, setSchools] = useState<AdminSchool[]>([])
  const [schoolID, setSchoolID] = useState<number | null>(null)
  const [choice, setChoice] = useState('90')
  const [loading, setLoading] = useState(true)
  const [retentionLoading, setRetentionLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState('')
  const canEdit = authService.getStoredUser()?.is_admin === true

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const access = await getAdminSchoolAccess()
        if (cancelled) return
        setSchools(access.schools)
        setSchoolID(access.schools[0]?.id ?? null)
        setError(access.schools.length === 0 ? '' : '')
      } catch {
        if (!cancelled) setError('学校の一覧を読み込めませんでした')
      } finally {
        if (!cancelled) setLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  useEffect(() => {
    if (schoolID == null) {
      setRetentionLoading(false)
      return
    }
    let cancelled = false
    // 切り替え中に前の学校の値が残ったまま保存できないよう、未読込に戻す
    setRetentionLoading(true)
    setChoice('90')
    setSaved('')
    setError('')
    void (async () => {
      try {
        const response = await fetch(`/api/admin/schools/${schoolID}`, {
          headers: authService.getAdminFetchHeaders(),
        })
        if (!response.ok) throw new Error('failed')
        const data = (await response.json()) as { interview_video_retention_days?: number | null }
        if (cancelled) return
        setChoice(toChoice(data.interview_video_retention_days ?? null))
        setError('')
      } catch {
        if (!cancelled) setError('今の保持期間を読み込めませんでした')
      } finally {
        if (!cancelled) setRetentionLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [schoolID])

  const save = async () => {
    if (schoolID == null || !canEdit || retentionLoading) return
    setSaving(true)
    setSaved('')
    setError('')
    try {
      const days = choice === 'unlimited' ? null : Number(choice)
      const response = await fetch(`/api/admin/schools/${schoolID}/interview-video-retention`, {
        method: 'PUT',
        headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ days }),
      })
      if (!response.ok) throw new Error('failed')
      setSaved('保存しました。短くした場合も、公開から30日は今残っている録画をすぐには消しません。')
    } catch {
      setError('保存できませんでした')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Card elevation={0} sx={{ mb: 3, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
      <CardContent>
        <Typography variant="h6" component="h2" gutterBottom>
          面接動画を残す期間
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          学校ごとに 30日、90日、180日、期限なしから選びます。過ぎた録画は削除されます。退会した学生の録画は、退会後12ヶ月で削除します。
        </Typography>
        {loading && <Typography variant="body2">学校の設定を読み込んでいます</Typography>}
        {!loading && error && (
          <Alert severity="error" sx={{ mb: 1 }}>{error}</Alert>
        )}
        {!loading && schools.length === 0 && !error && (
          <Typography variant="body2">担当している学校がありません。</Typography>
        )}
        {!loading && schools.length > 0 && (
          <>
            {schools.length > 1 && (
              <TextField
                select
                label="学校"
                value={schoolID ?? ''}
                onChange={(event) => setSchoolID(Number(event.target.value))}
                sx={{ mr: 2, mb: 2, minWidth: 220 }}
              >
                {schools.map((school) => (
                  <MenuItem key={school.id} value={school.id}>{school.name}</MenuItem>
                ))}
              </TextField>
            )}
            <TextField
              select
              label="残す期間"
              value={choice}
              disabled={!canEdit || retentionLoading}
              onChange={(event) => setChoice(event.target.value)}
              sx={{ mb: 2, minWidth: 220 }}
            >
              {OPTIONS.map((option) => (
                <MenuItem key={option.value} value={option.value}>{option.label}</MenuItem>
              ))}
            </TextField>
            {!canEdit && (
              <Typography variant="body2">変更できるのは学校管理者です。</Typography>
            )}
            {canEdit && (
              <Button
                variant="contained"
                onClick={() => void save()}
                disabled={saving || retentionLoading}
              >
                {saving ? '保存しています' : retentionLoading ? '読み込み中' : 'この期間で保存する'}
              </Button>
            )}
            {saved && <Alert severity="success" sx={{ mt: 2 }}>{saved}</Alert>}
          </>
        )}
      </CardContent>
    </Card>
  )
}
