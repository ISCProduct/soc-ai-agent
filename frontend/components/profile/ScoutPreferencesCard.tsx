'use client'

import { FormEvent, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  FormControlLabel,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/config'

type IndustryOption = { id: number; name: string; level: number }

type Preferences = {
  desired_industry_id?: number | null
  desired_location: string
  note: string
  allow_scout_visibility: boolean
}

const EMPTY: Preferences = {
  desired_industry_id: null,
  desired_location: '',
  note: '',
  allow_scout_visibility: false,
}

/**
 * スカウト向けの希望条件と公開同意の設定 (#1094)。
 * 公開同意をONにするまで企業の学生検索には一切表示されない。
 */
export function ScoutPreferencesCard() {
  const [prefs, setPrefs] = useState<Preferences>(EMPTY)
  const [industries, setIndustries] = useState<IndustryOption[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    const headers = authService.getUserFetchHeaders()
    Promise.all([
      fetch(`${BACKEND_URL}/api/user/preferences`, { headers }).then((r) =>
        r.ok ? r.json() : Promise.reject(new Error('failed')),
      ),
      fetch(`${BACKEND_URL}/api/user/industries`, { headers })
        .then((r) => (r.ok ? r.json() : { items: [] }))
        .catch(() => ({ items: [] })),
    ])
      .then(([pref, industryRes]) => {
        setPrefs({
          desired_industry_id: pref.desired_industry_id ?? null,
          desired_location: pref.desired_location ?? '',
          note: pref.note ?? '',
          allow_scout_visibility: Boolean(pref.allow_scout_visibility),
        })
        setIndustries((industryRes.items as IndustryOption[]) || [])
      })
      .catch(() => setError('希望条件の取得に失敗しました'))
      .finally(() => setLoading(false))
  }, [])

  const save = async (e: FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      const res = await fetch(`${BACKEND_URL}/api/user/preferences`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({
          desired_industry_id: prefs.desired_industry_id || null,
          desired_location: prefs.desired_location,
          note: prefs.note,
          allow_scout_visibility: prefs.allow_scout_visibility,
        }),
      })
      if (!res.ok) throw new Error('failed')
      setSaved(true)
    } catch {
      setError('保存に失敗しました')
    } finally {
      setSaving(false)
    }
  }

  if (loading) {
    return (
      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
        <CardContent>
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
            <CircularProgress size={24} />
          </Box>
        </CardContent>
      </Card>
    )
  }

  return (
    <Card
      component="form"
      onSubmit={save}
      elevation={0}
      sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}
    >
      <CardContent>
        <Typography variant="h6" gutterBottom>
          スカウト設定
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          企業からのスカウトを受け取るための設定です。公開に同意しない限り、企業の学生検索には表示されません。
        </Typography>

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}
        {saved && (
          <Alert severity="success" sx={{ mb: 2 }}>
            保存しました
          </Alert>
        )}

        <FormControlLabel
          control={
            <Switch
              checked={prefs.allow_scout_visibility}
              onChange={(e) =>
                setPrefs((p) => ({ ...p, allow_scout_visibility: e.target.checked }))
              }
            />
          }
          label="企業に自分のプロフィール・分析結果を公開する"
        />
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 2 }}>
          オフにすると、企業側の一覧・検索から即座に削除されます。
        </Typography>

        <Divider sx={{ mb: 2 }} />

        <Stack spacing={2}>
          <TextField
            select
            fullWidth
            label="希望業界"
            value={prefs.desired_industry_id ? String(prefs.desired_industry_id) : ''}
            onChange={(e) =>
              setPrefs((p) => ({
                ...p,
                desired_industry_id: e.target.value ? Number(e.target.value) : null,
              }))
            }
          >
            <MenuItem value="">指定なし</MenuItem>
            {industries.map((industry) => (
              <MenuItem key={industry.id} value={String(industry.id)}>
                {industry.level > 0 ? `　${industry.name}` : industry.name}
              </MenuItem>
            ))}
          </TextField>

          <TextField
            fullWidth
            label="希望勤務地"
            placeholder="例: 東京都"
            value={prefs.desired_location}
            onChange={(e) => setPrefs((p) => ({ ...p, desired_location: e.target.value }))}
            inputProps={{ maxLength: 100 }}
          />

          <TextField
            fullWidth
            multiline
            minRows={3}
            label="自己PR・希望条件の補足"
            placeholder="例: チーム開発の経験があり、Reactを使ったWebアプリ開発に携わりたいです。"
            value={prefs.note}
            onChange={(e) => setPrefs((p) => ({ ...p, note: e.target.value }))}
            helperText="ここに書いた内容は、企業の自然文検索の対象になります。"
          />
        </Stack>

        <Button type="submit" variant="contained" sx={{ mt: 2 }} disabled={saving}>
          {saving ? '保存中...' : '保存'}
        </Button>
      </CardContent>
    </Card>
  )
}
