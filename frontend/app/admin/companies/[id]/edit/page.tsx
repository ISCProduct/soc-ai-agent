'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Collapse,
  Divider,
  IconButton,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@mui/material'
import RefreshIcon from '@mui/icons-material/Refresh'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

type CompanyInfoResult = {
  description: string
  industry: string
  location: string
  website_url: string
  founded_year: number
  employee_count: number
  main_business: string
  culture: string
  work_style: string
}

type JobPosition = {
  title: string
  job_url: string
  employment_type: string
  work_location: string
  remote_option: boolean
  min_salary: number
  max_salary: number
  required_skills: string[]
  preferred_skills: string[]
  description: string
}

type PersonaProfile = {
  technical_orientation: number
  teamwork_orientation: number
  leadership_orientation: number
  creativity_orientation: number
  stability_orientation: number
  growth_orientation: number
  work_life_balance: number
  challenge_seeking: number
  detail_orientation: number
  communication_skill: number
}

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
  const [infoLoading, setInfoLoading] = useState(false)
  const [infoPreview, setInfoPreview] = useState<CompanyInfoResult | null>(null)
  const [jobsLoading, setJobsLoading] = useState(false)
  const [jobsResult, setJobsResult] = useState<JobPosition[] | null>(null)
  const [personaLoading, setPersonaLoading] = useState(false)
  const [personaResult, setPersonaResult] = useState<PersonaProfile | null>(null)
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

  const handleFetchInfo = async (force = false) => {
    setInfoLoading(true)
    setError('')
    setInfoPreview(null)
    try {
      const url = `/api/admin/companies/${id}/fetch-info${force ? '?force=true' : ''}`
      const res = await fetch(url, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        setError(d?.message || '企業情報の自動取得に失敗しました')
        return
      }
      const data: CompanyInfoResult = await res.json()
      setInfoPreview(data)
      setSuccess(force ? 'AIで企業基本情報を再取得してDBに保存しました' : 'DBから企業基本情報を取得しました')
    } finally {
      setInfoLoading(false)
    }
  }

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

  const handleFetchJobs = async (force = false) => {
    setJobsLoading(true)
    setError('')
    setJobsResult(null)
    try {
      const url = `/api/admin/companies/${id}/fetch-jobs${force ? '?force=true' : ''}`
      const res = await fetch(url, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        setError(d?.message || '求人情報の自動取得に失敗しました')
        return
      }
      const data = await res.json()
      setJobsResult(data.positions ?? [])
      setSuccess(force ? `AIで求人情報を再取得して${data.total ?? 0}件保存しました` : `DBから求人情報${data.total ?? 0}件を取得しました`)
    } finally {
      setJobsLoading(false)
    }
  }

  const handleFetchPersona = async (force = false) => {
    setPersonaLoading(true)
    setError('')
    setPersonaResult(null)
    try {
      const url = `/api/admin/companies/${id}/fetch-persona${force ? '?force=true' : ''}`
      const res = await fetch(url, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        setError(d?.message || '人物像の自動取得に失敗しました')
        return
      }
      const data: PersonaProfile = await res.json()
      setPersonaResult(data)
      setSuccess(force ? 'AIで人物像を再分析してCompanyWeightProfileに保存しました' : 'DBから求める人物像を取得しました')
    } finally {
      setPersonaLoading(false)
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
        <Box sx={{ p: 2, bgcolor: '#f0fdf4', border: '1px solid #bbf7d0', borderRadius: 2 }}>
          <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#14532d', mb: 1 }}>
            企業基本情報の自動取得
          </Typography>
          <Typography sx={{ fontSize: 13, color: '#166534', mb: 2 }}>
            OpenAI Web Searchで企業概要・業種・所在地・公式URL・事業内容などを取得してDBに保存します。
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
            <Button
              variant="contained"
              color="success"
              onClick={() => handleFetchInfo(false)}
              disabled={infoLoading}
              startIcon={infoLoading ? <CircularProgress size={16} color="inherit" /> : undefined}
            >
              {infoLoading ? '取得中...' : '🔍 企業基本情報を取得'}
            </Button>
            <Tooltip title="DBキャッシュを無視してAIで再取得する">
              <span>
                <IconButton
                  size="small"
                  onClick={() => handleFetchInfo(true)}
                  disabled={infoLoading}
                  sx={{ color: '#166534', border: '1px solid #166534', borderRadius: 1 }}
                >
                  <RefreshIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          </Stack>

          <Collapse in={infoPreview !== null}>
            {infoPreview && (
              <Card variant="outlined" sx={{ mt: 1 }}>
                <CardContent>
                  <Typography variant="subtitle2" sx={{ mb: 1, color: '#166534' }}>取得結果プレビュー（DB保存済み）</Typography>
                  <Stack spacing={0.5}>
                    {infoPreview.description && (
                      <Typography variant="body2"><strong>概要:</strong> {infoPreview.description}</Typography>
                    )}
                    {infoPreview.industry && (
                      <Typography variant="body2"><strong>業種:</strong> {infoPreview.industry}</Typography>
                    )}
                    {infoPreview.location && (
                      <Typography variant="body2"><strong>所在地:</strong> {infoPreview.location}</Typography>
                    )}
                    {infoPreview.website_url && (
                      <Typography variant="body2"><strong>公式URL:</strong> <a href={infoPreview.website_url} target="_blank" rel="noopener noreferrer">{infoPreview.website_url}</a></Typography>
                    )}
                    {infoPreview.founded_year > 0 && (
                      <Typography variant="body2"><strong>設立年:</strong> {infoPreview.founded_year}年</Typography>
                    )}
                    {infoPreview.employee_count > 0 && (
                      <Typography variant="body2"><strong>従業員数:</strong> {infoPreview.employee_count.toLocaleString()}名</Typography>
                    )}
                    {infoPreview.main_business && (
                      <Typography variant="body2"><strong>主要事業:</strong> {infoPreview.main_business}</Typography>
                    )}
                    {infoPreview.culture && (
                      <Typography variant="body2"><strong>企業文化:</strong> {infoPreview.culture}</Typography>
                    )}
                    {infoPreview.work_style && (
                      <Typography variant="body2"><strong>勤務スタイル:</strong> {infoPreview.work_style}</Typography>
                    )}
                  </Stack>
                </CardContent>
              </Card>
            )}
          </Collapse>
        </Box>

        <Divider />

        {/* 求人情報の自動取得 */}
        <Box sx={{ p: 2, bgcolor: '#fefce8', border: '1px solid #fde047', borderRadius: 2 }}>
          <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#713f12', mb: 1 }}>
            求人情報の自動取得（採用ページ・Wantedly）
          </Typography>
          <Typography sx={{ fontSize: 13, color: '#854d0e', mb: 2 }}>
            企業の採用ページをクロールし、WantedlyのWebSearchも組み合わせて求人情報をDBに保存します。
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
            <Button
              variant="contained"
              onClick={() => handleFetchJobs(false)}
              disabled={jobsLoading}
              startIcon={jobsLoading ? <CircularProgress size={16} color="inherit" /> : undefined}
              sx={{ bgcolor: '#ca8a04', '&:hover': { bgcolor: '#a16207' } }}
            >
              {jobsLoading ? '取得中...' : '📋 求人情報を取得'}
            </Button>
            <Tooltip title="DBキャッシュを無視してAIで再取得する">
              <span>
                <IconButton
                  size="small"
                  onClick={() => handleFetchJobs(true)}
                  disabled={jobsLoading}
                  sx={{ color: '#a16207', border: '1px solid #a16207', borderRadius: 1 }}
                >
                  <RefreshIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          </Stack>

          <Collapse in={jobsResult !== null}>
            {jobsResult && jobsResult.length === 0 && (
              <Typography sx={{ fontSize: 13, color: '#92400e' }}>求人情報が見つかりませんでした。</Typography>
            )}
            {jobsResult && jobsResult.length > 0 && (
              <Stack spacing={1}>
                {jobsResult.map((job, i) => (
                  <Card key={i} variant="outlined">
                    <CardContent sx={{ py: 1, '&:last-child': { pb: 1 } }}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Typography sx={{ fontWeight: 700, fontSize: 13 }}>{job.title}</Typography>
                        {job.job_url && (
                          <a href={job.job_url} target="_blank" rel="noopener noreferrer" style={{ fontSize: 12, color: '#0284c7' }}>
                            求人ページ →
                          </a>
                        )}
                      </Box>
                      <Stack direction="row" spacing={1} sx={{ mt: 0.5, flexWrap: 'wrap', gap: 0.5 }}>
                        {job.employment_type && <Chip label={job.employment_type} size="small" />}
                        {job.work_location && <Chip label={job.work_location} size="small" variant="outlined" />}
                        {job.remote_option && <Chip label="リモート可" size="small" color="success" />}
                        {(job.min_salary > 0 || job.max_salary > 0) && (
                          <Chip label={`${job.min_salary}〜${job.max_salary}万円`} size="small" variant="outlined" />
                        )}
                      </Stack>
                      {job.description && (
                        <Typography sx={{ fontSize: 12, color: 'text.secondary', mt: 0.5 }}>{job.description}</Typography>
                      )}
                    </CardContent>
                  </Card>
                ))}
              </Stack>
            )}
          </Collapse>
        </Box>

        <Divider />

        {/* 求める人物像の自動分析 */}
        <Box sx={{ p: 2, bgcolor: '#fdf4ff', border: '1px solid #e9d5ff', borderRadius: 2 }}>
          <Typography sx={{ fontWeight: 700, fontSize: 14, color: '#581c87', mb: 1 }}>
            求める人物像の自動分析（CompanyWeightProfile）
          </Typography>
          <Typography sx={{ fontSize: 13, color: '#6b21a8', mb: 2 }}>
            企業の採用情報・社風をAIで分析し、10カテゴリのマッチングスコアをDBに保存します。
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 2 }}>
            <Button
              variant="contained"
              onClick={() => handleFetchPersona(false)}
              disabled={personaLoading}
              startIcon={personaLoading ? <CircularProgress size={16} color="inherit" /> : undefined}
              sx={{ bgcolor: '#7c3aed', '&:hover': { bgcolor: '#6d28d9' } }}
            >
              {personaLoading ? '分析中...' : '👤 人物像を分析'}
            </Button>
            <Tooltip title="DBキャッシュを無視してAIで再分析する">
              <span>
                <IconButton
                  size="small"
                  onClick={() => handleFetchPersona(true)}
                  disabled={personaLoading}
                  sx={{ color: '#6d28d9', border: '1px solid #6d28d9', borderRadius: 1 }}
                >
                  <RefreshIcon fontSize="small" />
                </IconButton>
              </span>
            </Tooltip>
          </Stack>

          <Collapse in={personaResult !== null}>
            {personaResult && (
              <Card variant="outlined">
                <CardContent>
                  <Typography variant="subtitle2" sx={{ mb: 1, color: '#6b21a8' }}>分析結果（DB保存済み）</Typography>
                  <Stack spacing={0.5}>
                    {[
                      ['技術志向', personaResult.technical_orientation],
                      ['チームワーク', personaResult.teamwork_orientation],
                      ['リーダーシップ', personaResult.leadership_orientation],
                      ['創造性', personaResult.creativity_orientation],
                      ['安定志向', personaResult.stability_orientation],
                      ['成長志向', personaResult.growth_orientation],
                      ['ワークライフバランス', personaResult.work_life_balance],
                      ['チャレンジ精神', personaResult.challenge_seeking],
                      ['細部への注意力', personaResult.detail_orientation],
                      ['コミュニケーション力', personaResult.communication_skill],
                    ].map(([label, value]) => (
                      <Box key={label as string} sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Typography sx={{ fontSize: 12, width: 160, flexShrink: 0 }}>{label}</Typography>
                        <Box sx={{ flex: 1, bgcolor: '#e9d5ff', borderRadius: 1, height: 8 }}>
                          <Box sx={{ width: `${value}%`, bgcolor: '#7c3aed', borderRadius: 1, height: 8 }} />
                        </Box>
                        <Typography sx={{ fontSize: 12, width: 30, textAlign: 'right' }}>{value}</Typography>
                      </Box>
                    ))}
                  </Stack>
                </CardContent>
              </Card>
            )}
          </Collapse>
        </Box>

        <Divider />

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
