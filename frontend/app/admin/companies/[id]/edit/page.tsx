'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

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
  const [aiLoading, setAiLoading] = useState(false)
  const [name, setName] = useState('')
  const [techStack, setTechStack] = useState<string[]>([])
  const [infraStack, setInfraStack] = useState<string[]>([])
  const [cicdTools, setCicdTools] = useState<string[]>([])
  const [devStyle, setDevStyle] = useState('')

  useEffect(() => {
    const admin = authService.getStoredUser()
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
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }, [id])

  const handleAiFill = async () => {
    setAiLoading(true)
    setError('')
    const admin = authService.getStoredUser()
    try {
      const res = await fetch(`/api/admin/companies/${id}/tech-stack-search`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        setError(d?.error || 'AI自動入力に失敗しました')
        return
      }
      const data = await res.json()
      if (data.tech_stack?.length) setTechStack(data.tech_stack)
      if (data.infra_stack?.length) setInfraStack(data.infra_stack)
      if (data.cicd_tools?.length) setCicdTools(data.cicd_tools)
      if (data.development_style) setDevStyle(data.development_style)
      setSuccess('AI自動入力が完了しました（保存ボタンで確定してください）')
    } finally {
      setAiLoading(false)
    }
  }

  const handleSave = async () => {
    setError('')
    setSuccess('')
    const admin = authService.getStoredUser()
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

  return (
    <AdminFormContainer
      title={`技術スタック編集: ${name}`}
      maxWidth={800}
      backLabel="← 戻る"
      onBack={() => router.back()}
    >
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}
      <Stack spacing={3}>
        <Button
          variant="contained"
          color="secondary"
          onClick={handleAiFill}
          disabled={aiLoading}
          sx={{ alignSelf: 'flex-start' }}
        >
          {aiLoading ? 'AI検索中...' : '🤖 AI自動入力（OpenAI WebSearch）'}
        </Button>

        <Divider />

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
