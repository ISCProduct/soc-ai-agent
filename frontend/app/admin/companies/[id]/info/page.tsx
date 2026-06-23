'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Alert,
  Button,
  CircularProgress,
  Divider,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

export default function AdminCompanyInfoEditPage() {
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
  const [description, setDescription] = useState('')
  const [industry, setIndustry] = useState('')
  const [location, setLocation] = useState('')
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [foundedYear, setFoundedYear] = useState('')
  const [employeeCount, setEmployeeCount] = useState('')
  const [mainBusiness, setMainBusiness] = useState('')
  const [culture, setCulture] = useState('')
  const [workStyle, setWorkStyle] = useState('')
  const [sourceType, setSourceType] = useState('manual')
  const [sourceUrl, setSourceUrl] = useState('')
  const [dataStatus, setDataStatus] = useState('draft')
  const [isProvisional, setIsProvisional] = useState(false)

  useEffect(() => {
    fetch(`/api/admin/companies/${id}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => {
        setName(data.name || '')
        setDescription(data.description || '')
        setIndustry(data.industry || '')
        setLocation(data.location || '')
        setWebsiteUrl(data.website_url || '')
        setFoundedYear(data.founded_year ? String(data.founded_year) : '')
        setEmployeeCount(data.employee_count ? String(data.employee_count) : '')
        setMainBusiness(data.main_business || '')
        setCulture(data.culture || '')
        setWorkStyle(data.work_style || '')
        setSourceType(data.source_type || 'manual')
        setSourceUrl(data.source_url || '')
        setDataStatus(data.data_status || 'draft')
        setIsProvisional(data.is_provisional ?? false)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }, [id])

  const handleAiFetch = async () => {
    if (!name.trim()) return
    setAiLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await fetch('/api/admin/companies/web-search', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({ name }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || 'AI企業情報取得に失敗しました')
        return
      }
      if (data.description) setDescription(data.description)
      if (data.industry) setIndustry(data.industry)
      if (data.location) setLocation(data.location)
      if (data.website_url) setWebsiteUrl(data.website_url)
      if (data.founded_year) setFoundedYear(String(data.founded_year))
      if (data.employee_count) setEmployeeCount(String(data.employee_count))
      if (data.main_business) setMainBusiness(data.main_business)
      if (data.culture) setCulture(data.culture)
      if (data.work_style) setWorkStyle(data.work_style)
      setSuccess('AI情報取得が完了しました。内容を確認・修正してから保存してください。')
    } finally {
      setAiLoading(false)
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
        name,
        description,
        industry,
        location,
        website_url: websiteUrl,
        founded_year: foundedYear ? parseInt(foundedYear, 10) : undefined,
        employee_count: employeeCount ? parseInt(employeeCount, 10) : undefined,
        main_business: mainBusiness,
        culture,
        work_style: workStyle,
        source_type: sourceType,
        source_url: sourceUrl,
        is_provisional: isProvisional,
        data_status: dataStatus,
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
      title={`基本情報編集: ${name}`}
      maxWidth={700}
      backLabel="← 企業一覧に戻る"
      backHref="/admin/companies"
    >
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}

      <Stack spacing={2}>
        <TextField label="企業名" value={name} onChange={(e) => setName(e.target.value)} required />

        <Stack direction="row" alignItems="center" spacing={1}>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleAiFetch}
            disabled={!name.trim() || aiLoading}
            startIcon={aiLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {aiLoading ? 'AI検索中...' : '🤖 AIで企業情報を自動取得'}
          </Button>
          <Typography variant="caption" color="text.secondary">
            企業名をもとにAIが情報を取得してフォームに反映します
          </Typography>
        </Stack>

        <Divider />

        <TextField
          label="企業概要"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          multiline
          minRows={2}
        />
        <TextField label="業種" value={industry} onChange={(e) => setIndustry(e.target.value)} />
        <TextField label="所在地" value={location} onChange={(e) => setLocation(e.target.value)} />
        <TextField label="公式サイトURL" value={websiteUrl} onChange={(e) => setWebsiteUrl(e.target.value)} />
        <Stack direction="row" spacing={2}>
          <TextField
            label="設立年"
            value={foundedYear}
            onChange={(e) => setFoundedYear(e.target.value)}
            type="number"
            sx={{ flex: 1 }}
          />
          <TextField
            label="従業員数"
            value={employeeCount}
            onChange={(e) => setEmployeeCount(e.target.value)}
            type="number"
            sx={{ flex: 1 }}
          />
        </Stack>
        <TextField
          label="主要事業内容"
          value={mainBusiness}
          onChange={(e) => setMainBusiness(e.target.value)}
          multiline
          minRows={2}
        />
        <TextField
          label="企業文化"
          value={culture}
          onChange={(e) => setCulture(e.target.value)}
          multiline
          minRows={2}
        />
        <TextField
          select
          label="勤務スタイル"
          value={workStyle}
          onChange={(e) => setWorkStyle(e.target.value)}
        >
          <MenuItem value="">未設定</MenuItem>
          <MenuItem value="リモート">リモート</MenuItem>
          <MenuItem value="ハイブリッド">ハイブリッド</MenuItem>
          <MenuItem value="オフィス">オフィス</MenuItem>
        </TextField>

        <Divider />

        <TextField select label="出典タイプ" value={sourceType} onChange={(e) => setSourceType(e.target.value)}>
          <MenuItem value="official">公式サイト</MenuItem>
          <MenuItem value="job_site">就活/転職サイト</MenuItem>
          <MenuItem value="manual">手入力</MenuItem>
        </TextField>
        <TextField label="出典URL" value={sourceUrl} onChange={(e) => setSourceUrl(e.target.value)} />
        <TextField select label="ステータス" value={dataStatus} onChange={(e) => setDataStatus(e.target.value)}>
          <MenuItem value="draft">下書き</MenuItem>
          <MenuItem value="published">公開</MenuItem>
        </TextField>
        <TextField
          select
          label="暫定データ"
          value={isProvisional ? 'yes' : 'no'}
          onChange={(e) => setIsProvisional(e.target.value === 'yes')}
        >
          <MenuItem value="yes">暫定</MenuItem>
          <MenuItem value="no">確定</MenuItem>
        </TextField>

        <Button variant="contained" onClick={handleSave} disabled={!name.trim()}>
          保存する
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
