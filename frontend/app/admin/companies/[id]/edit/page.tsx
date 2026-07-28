'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
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
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { fetchCompanyPrimary, formatFetchPrimarySummary } from '@/lib/admin-company-fetch'

const DEV_STYLES = ['スクラム', 'ウォーターフォール', 'カンバン', 'アジャイル', 'その他']

function ChipEditor({
  label,
  values,
  onChange,
}: {
  label: string
  values: string[]
  onChange: (v: string[]) => void
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
      </Stack>
      <Stack direction="row" spacing={1}>
        <TextField
          size="small"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && add()}
          placeholder="入力してEnter"
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
  const router = useRouter()
  const id = params.id as string

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [fetchLoading, setFetchLoading] = useState(false)
  const [primaryLoading, setPrimaryLoading] = useState(false)
  const [name, setName] = useState('')
  const [techStack, setTechStack] = useState<string[]>([])
  const [infraStack, setInfraStack] = useState<string[]>([])
  const [cicdTools, setCicdTools] = useState<string[]>([])
  const [devStyle, setDevStyle] = useState('')
  const [techFetchedAt, setTechFetchedAt] = useState<string | null>(null)

  const loadCompany = () => {
    fetch(`/api/admin/companies/${id}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => {
        setName(data.name || '')
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
        setError(data?.error || `技術スタック取得に失敗しました (${res.status})`)
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
        setSuccess('取得は完了しましたが、公開情報から技術スタックを特定できませんでした。手入力するか強制再取得を試してください。')
      } else {
        setSuccess(force ? '技術スタックを強制再取得して保存しました。' : '技術スタックを取得して保存しました。')
      }
    } finally {
      setFetchLoading(false)
    }
  }

  const handleFetchPrimary = async (force = false) => {
    setPrimaryLoading(true)
    setError('')
    setSuccess('')
    try {
      const { ok, status, data } = await fetchCompanyPrimary(
        id,
        authService.getAdminFetchHeaders(),
        force,
      )
      if (!ok) {
        setError(data?.error || `主3種の取得に失敗しました (${status})`)
        return
      }
      if (data.company && typeof data.company === 'object') {
        const company = data.company as Record<string, unknown>
        setTechStack(parseJsonArray(String(company.tech_stack || '')))
        setInfraStack(parseJsonArray(String(company.infra_stack || '')))
        setCicdTools(parseJsonArray(String(company.cicd_tools || '')))
        setDevStyle(String(company.development_style || ''))
        setTechFetchedAt(company.tech_fetched_at ? String(company.tech_fetched_at) : null)
      } else {
        loadCompany()
      }
      if (data.ok === false && Array.isArray(data.errors) && data.errors.length > 0) {
        setError(`一部失敗: ${data.errors.join('; ')}`)
      }
      const summary = formatFetchPrimarySummary(data)
      setSuccess(summary ? `主3種取得完了: ${summary}` : '主3種取得完了')
    } finally {
      setPrimaryLoading(false)
    }
  }

  const handleSave = async () => {
    setError('')
    setSuccess('')
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
  }

  const formatTs = (v: string | null) => {
    if (!v) return '未取得'
    const d = new Date(v)
    return Number.isNaN(d.getTime()) ? v : d.toLocaleString('ja-JP')
  }

  return (
    <AdminFormContainer
      title={`技術スタック編集: ${name}`}
      maxWidth={800}
      backLabel="企業一覧に戻る"
      backHref="/admin/companies"
      onBack={() => router.back()}
    >
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}
      <Alert severity="info" sx={{ mb: 2 }}>
        「主3種をまとめて取得」で Backend が基本・技術・ビジネス関係を順に取得して DB 保存します。
        必要ならこの画面の個別取得で技術だけ再取得できます（TTL 30日）。
        最終取得: {formatTs(techFetchedAt)}
      </Alert>

      <Box sx={{ mb: 3, p: 2, bgcolor: '#f0f9ff', border: '1px solid #bae6fd', borderRadius: 2, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <Box>
          <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#0c4a6e' }}>面接カスタム質問</Typography>
          <Typography sx={{ fontSize: 13, color: '#075985' }}>AI面接で使用する企業別の質問リストを管理できます</Typography>
        </Box>
        <Button
          variant="outlined"
          size="small"
          onClick={() => router.push(`/admin/companies/${id}/interview-questions`)}
          sx={{ borderColor: '#0284c7', color: '#0284c7', whiteSpace: 'nowrap' }}
        >
          質問を管理 →
        </Button>
      </Box>

      <Stack spacing={3}>
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            variant="contained"
            color="secondary"
            onClick={() => handleFetchPrimary(false)}
            disabled={fetchLoading || primaryLoading}
            startIcon={primaryLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {primaryLoading ? '取得中...' : '主3種をまとめて取得'}
          </Button>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => handleFetchPrimary(true)}
            disabled={fetchLoading || primaryLoading}
          >
            主3種を強制再取得
          </Button>
          <Button
            variant="contained"
            color="secondary"
            onClick={() => handleAiFetch(false)}
            disabled={fetchLoading || primaryLoading}
            startIcon={fetchLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {fetchLoading ? '取得中...' : 'AIで技術スタック取得'}
          </Button>
          <Button
            variant="outlined"
            onClick={() => handleAiFetch(true)}
            disabled={fetchLoading || primaryLoading}
          >
            強制再取得
          </Button>
        </Stack>

        <ChipEditor
          label="言語・フレームワーク（例: Go, React, TypeScript）"
          values={techStack}
          onChange={setTechStack}
        />
        <ChipEditor
          label="インフラ（例: AWS, GCP, Azure, オンプレ）"
          values={infraStack}
          onChange={setInfraStack}
        />
        <ChipEditor
          label="CI/CDツール（例: GitHub Actions, Jenkins, CircleCI）"
          values={cicdTools}
          onChange={setCicdTools}
        />
        <TextField
          select
          label="開発手法"
          value={devStyle}
          onChange={(e) => setDevStyle(e.target.value)}
          size="small"
        >
          <MenuItem value="">未設定</MenuItem>
          {DEV_STYLES.map((s) => (
            <MenuItem key={s} value={s}>{s}</MenuItem>
          ))}
        </TextField>

        <Button variant="contained" onClick={handleSave} sx={{ alignSelf: 'flex-end' }}>
          保存
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
