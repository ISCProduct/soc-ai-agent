'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Chip,
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
  const [name, setName] = useState('')
  const [techStack, setTechStack] = useState<string[]>([])
  const [infraStack, setInfraStack] = useState<string[]>([])
  const [cicdTools, setCicdTools] = useState<string[]>([])
  const [devStyle, setDevStyle] = useState('')

  useEffect(() => {
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

  return (
    <AdminFormContainer
      title={`技術スタック編集: ${name}`}
      maxWidth={800}
      backLabel="← 戻る"
      onBack={() => router.back()}
    >
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}

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
