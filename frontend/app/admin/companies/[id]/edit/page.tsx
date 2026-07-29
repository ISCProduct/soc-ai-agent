'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import {
  resolveIndustryFieldProfile,
  type TechFieldKey,
} from '@/lib/admin-company-field-profile'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { CompanyAspectTabs } from '@/components/admin/CompanyAspectTabs'
import { ErrorAlert } from '@/components/common/ErrorAlert'

function ChipEditor({
  label,
  values,
  onChange,
  placeholder,
}: {
  label: string
  values: string[]
  onChange: (v: string[]) => void
  placeholder?: string
}) {
  const [input, setInput] = useState('')
  const add = () => {
    const v = input.trim()
    if (v && !values.includes(v)) onChange([...values, v])
    setInput('')
  }
  return (
    <Box>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
        {label}
      </Typography>
      <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mb: 1 }}>
        {values.map((v) => (
          <Chip key={v} label={v} onDelete={() => onChange(values.filter((x) => x !== v))} size="small" />
        ))}
        {values.length === 0 && (
          <Typography variant="body2" color="text.secondary">
            まだ登録がありません
          </Typography>
        )}
      </Stack>
      <Stack direction="row" spacing={1}>
        <TextField
          size="small"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          placeholder={placeholder || '入力してEnter'}
          sx={{ flex: 1 }}
        />
        <Button variant="outlined" size="small" onClick={add}>
          追加
        </Button>
      </Stack>
    </Box>
  )
}

function parseJsonArray(s: string): string[] {
  try {
    const parsed = JSON.parse(s)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return s ? s.split(',').map((x) => x.trim()).filter(Boolean) : []
  }
}

function asStringArray(v: unknown): string[] {
  if (!Array.isArray(v)) return []
  return v.map((x) => String(x).trim()).filter(Boolean)
}

export default function AdminCompanyEditPage() {
  const params = useParams()
  const id = params.id as string

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [fetchLoading, setFetchLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [name, setName] = useState('')
  const [industry, setIndustry] = useState('')
  const [techStack, setTechStack] = useState<string[]>([])
  const [infraStack, setInfraStack] = useState<string[]>([])
  const [cicdTools, setCicdTools] = useState<string[]>([])
  const [devStyle, setDevStyle] = useState('')
  const [techFetchedAt, setTechFetchedAt] = useState<string | null>(null)

  const profile = useMemo(() => resolveIndustryFieldProfile(industry), [industry])

  const loadCompany = () => {
    fetch(`/api/admin/companies/${id}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => {
        setName(data.name || '')
        setIndustry(data.industry || '')
        setTechStack(parseJsonArray(data.tech_stack || ''))
        setInfraStack(parseJsonArray(data.infra_stack || ''))
        setCicdTools(parseJsonArray(data.cicd_tools || ''))
        setDevStyle(data.development_style || '')
        setTechFetchedAt(data.tech_fetched_at || null)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }

  useEffect(() => {
    loadCompany()
  }, [id])

  const handleAiFetch = async (force = false) => {
    setFetchLoading(true)
    setError('')
    setSuccess('')
    try {
      const qs = force ? '?force=true' : ''
      const res = await fetch(`/api/admin/companies/${id}/tech-stack-search${qs}`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json().catch(() => ({}))
      if (!res.ok) {
        setError(data?.error || `技術情報の取得に失敗しました (${res.status})`)
        return
      }
      const nextTech = asStringArray(data.tech_stack)
      const nextInfra = asStringArray(data.infra_stack)
      const nextCicd = asStringArray(data.cicd_tools)
      if (nextTech.length > 0) setTechStack(nextTech)
      if (nextInfra.length > 0) setInfraStack(nextInfra)
      if (nextCicd.length > 0) setCicdTools(nextCicd)
      if (typeof data.development_style === 'string' && data.development_style) {
        setDevStyle(data.development_style)
      }
      loadCompany()
      if (nextTech.length === 0 && nextInfra.length === 0 && nextCicd.length === 0) {
        setSuccess(
          '取得は完了しましたが、公開情報から内容を特定できませんでした。手入力するか「最新の情報に更新」を試してください。',
        )
      } else {
        setSuccess(force ? '情報を更新して保存しました。' : '情報を取得して保存しました。')
      }
    } finally {
      setFetchLoading(false)
    }
  }

  const handleSave = async () => {
    setError('')
    setSuccess('')
    setSaving(true)
    try {
      const res = await fetch(`/api/admin/companies/${id}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({
          tech_stack: JSON.stringify(techStack),
          infra_stack: JSON.stringify(infraStack),
          cicd_tools: JSON.stringify(cicdTools),
          development_style: devStyle,
        }),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        setError(d?.error || '保存に失敗しました')
        return
      }
      setSuccess('保存しました')
    } finally {
      setSaving(false)
    }
  }

  const formatTs = (v: string | null) => {
    if (!v) return '未取得'
    const d = new Date(v)
    return Number.isNaN(d.getTime()) ? v : d.toLocaleString('ja-JP')
  }

  const busy = fetchLoading || saving
  const chipValues: Record<Exclude<TechFieldKey, 'development_style'>, string[]> = {
    tech_stack: techStack,
    infra_stack: infraStack,
    cicd_tools: cicdTools,
  }
  const chipSetters: Record<Exclude<TechFieldKey, 'development_style'>, (v: string[]) => void> = {
    tech_stack: setTechStack,
    infra_stack: setInfraStack,
    cicd_tools: setCicdTools,
  }

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.full}>
      <AdminPageHeader
        title={`${name || '企業'}（${profile.techAspectLabel}）`}
        description={
          profile.techAspectEnabled
            ? `${profile.label}向けの項目を表示しています。業種を変えると入力できる内容も変わります。`
            : 'この業界ではこの画面の登録は不要です。'
        }
        backHref="/admin/companies"
        backAriaLabel="企業一覧に戻る"
        actions={
          profile.techAspectEnabled ? (
            <Button variant="contained" onClick={handleSave} disabled={busy} disableElevation>
              {saving ? '保存中…' : '保存'}
            </Button>
          ) : null
        }
      />

      <CompanyAspectTabs companyId={id} active="tech" industry={industry} />
      <ErrorAlert error={error} />
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>
          {success}
        </Alert>
      )}

      {!profile.techAspectEnabled ? (
        <Alert severity="info" sx={{ mb: 2 }}>
          {profile.techDisabledMessage || 'この業界では技術情報の登録は不要です。'}
          <Box sx={{ mt: 1.5 }}>
            <Button component={Link} href={`/admin/companies/${id}/info`} variant="outlined" size="small">
              会社概要へ
            </Button>
          </Box>
        </Alert>
      ) : (
        <>
          <Box
            sx={{
              mb: 2,
              p: 2,
              border: '1px solid',
              borderColor: 'divider',
              borderRadius: '10px',
              bgcolor: 'background.paper',
            }}
          >
            <Stack
              direction={{ xs: 'column', md: 'row' }}
              spacing={2}
              alignItems={{ md: 'center' }}
              justifyContent="space-between"
            >
              <Box sx={{ minWidth: 0 }}>
                <Typography fontWeight={700} sx={{ mb: 0.25 }}>
                  情報の取得・保存
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  業種: {industry || '未設定'}（{profile.label}）／ 最終取得: {formatTs(techFetchedAt)}
                </Typography>
              </Box>
              <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                <Button
                  variant="outlined"
                  color="secondary"
                  onClick={() => handleAiFetch(false)}
                  disabled={busy}
                  startIcon={fetchLoading ? <CircularProgress size={16} color="inherit" /> : null}
                >
                  {fetchLoading ? '取得中…' : `${profile.techAspectLabel}を取得`}
                </Button>
                <Button variant="outlined" onClick={() => handleAiFetch(true)} disabled={busy}>
                  最新の情報に更新
                </Button>
                <Button variant="contained" onClick={handleSave} disabled={busy} disableElevation>
                  {saving ? '保存中…' : '保存'}
                </Button>
              </Stack>
            </Stack>
          </Box>

          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.7fr) minmax(280px, 0.9fr)' },
              gap: 2.5,
              alignItems: 'start',
            }}
          >
            <AdminPanel title={profile.techAspectLabel}>
              <Box sx={{ px: 2.5, py: 2 }}>
                <Stack spacing={3}>
                  {profile.techFields.map((field) => {
                    if (field.key === 'development_style') {
                      return (
                        <TextField
                          key={field.key}
                          select
                          label={field.label}
                          value={devStyle}
                          onChange={(e) => setDevStyle(e.target.value)}
                          size="small"
                          fullWidth
                        >
                          <MenuItem value="">未設定</MenuItem>
                          {(field.options || []).map((s) => (
                            <MenuItem key={s} value={s}>
                              {s}
                            </MenuItem>
                          ))}
                        </TextField>
                      )
                    }
                    return (
                      <ChipEditor
                        key={field.key}
                        label={field.label}
                        values={chipValues[field.key]}
                        onChange={chipSetters[field.key]}
                        placeholder={field.placeholder}
                      />
                    )
                  })}
                </Stack>
              </Box>
            </AdminPanel>

            <Stack spacing={2.5}>
              <AdminPanel title="業種について">
                <Box sx={{ px: 2.5, py: 2 }}>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                    表示項目は会社概要の「業種」に合わせて変わります。業種を変更する場合は会社概要で編集してください。
                  </Typography>
                  <Button component={Link} href={`/admin/companies/${id}/info`} variant="outlined" size="small">
                    会社概要で業種を変更
                  </Button>
                </Box>
              </AdminPanel>

              <AdminPanel title="面接カスタム質問">
                <Box sx={{ px: 2.5, py: 2 }}>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                    AI面接で使う、この企業向けの質問リストを管理できます。
                  </Typography>
                  <Button
                    component={Link}
                    href={`/admin/companies/${id}/interview-questions`}
                    variant="outlined"
                    size="small"
                  >
                    質問を管理
                  </Button>
                </Box>
              </AdminPanel>
            </Stack>
          </Box>
        </>
      )}
    </PageContainer>
  )
}
