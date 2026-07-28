'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
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

export default function AdminCompanyNewPage() {
  const router = useRouter()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [creating, setCreating] = useState(false)

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
  const [techStack, setTechStack] = useState('')
  const [welfareDetails, setWelfareDetails] = useState('')
  const [sourceType, setSourceType] = useState('manual')
  const [sourceUrl, setSourceUrl] = useState('')
  const [dataStatus, setDataStatus] = useState('draft')
  const [isProvisional, setIsProvisional] = useState(true)
  const [lastModelUsed, setLastModelUsed] = useState('')
  const [lastFetchConfidence, setLastFetchConfidence] = useState('')

  const applyInfoPayload = (data: Record<string, unknown>) => {
    if (typeof data.description === 'string' && data.description) setDescription(data.description)
    if (typeof data.industry === 'string' && data.industry) setIndustry(data.industry)
    if (typeof data.location === 'string' && data.location) setLocation(data.location)
    if (typeof data.website_url === 'string' && data.website_url) setWebsiteUrl(data.website_url)
    if (typeof data.founded_year === 'number' && data.founded_year) setFoundedYear(String(data.founded_year))
    if (typeof data.employee_count === 'number' && data.employee_count) setEmployeeCount(String(data.employee_count))
    if (typeof data.main_business === 'string' && data.main_business) setMainBusiness(data.main_business)
    if (typeof data.culture === 'string' && data.culture) setCulture(data.culture)
    if (typeof data.work_style === 'string' && data.work_style) setWorkStyle(data.work_style)
    if (typeof data.tech_stack === 'string' && data.tech_stack) setTechStack(data.tech_stack)
    if (typeof data.welfare_details === 'string' && data.welfare_details) setWelfareDetails(data.welfare_details)
    if (typeof data.source === 'string' && data.source) setSourceType(data.source)
    if (typeof data.source_url === 'string' && data.source_url) setSourceUrl(data.source_url)
    if (typeof data.model_used === 'string' && data.model_used) setLastModelUsed(data.model_used)
    if (typeof data.confidence === 'string' && data.confidence) setLastFetchConfidence(data.confidence)
  }

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
        body: JSON.stringify({ name, website_url: websiteUrl }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || '企業情報取得に失敗しました')
        return
      }
      applyInfoPayload(data)
      setSuccess('プレビュー取得が完了しました。内容を確認・修正してから「追加する」を押してください。')
    } finally {
      setAiLoading(false)
    }
  }

  const handleCreate = async () => {
    setError('')
    setSuccess('')
    setCreating(true)
    try {
      const res = await fetch('/api/admin/companies', {
        method: 'POST',
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
          tech_stack: techStack,
          welfare_details: welfareDetails,
          source_type: sourceType,
          source_url: sourceUrl || websiteUrl,
          is_provisional: isProvisional,
          data_status: dataStatus,
          last_model_used: lastModelUsed || undefined,
          last_fetch_confidence: lastFetchConfidence || undefined,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || '企業の作成に失敗しました')
        return
      }
      // 追加後は基本情報画面へ。公開済みでも継続して AI 取得・URL 更新が可能。
      router.push(`/admin/companies/${data.id}/info`)
    } finally {
      setCreating(false)
    }
  }

  return (
    <AdminFormContainer title="企業の追加" maxWidth={700} backHref="/admin/companies" backLabel="一覧に戻る">
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}
      <Alert severity="info" sx={{ mb: 2 }}>
        企業名（と任意で公式URL）を入れて「AIで情報取得」すると、概要・URL・業種などを埋められます。
        追加後も公開状態に関係なく、基本情報画面から再取得できます。
      </Alert>

      <Stack spacing={2}>
        <TextField label="企業名" value={name} onChange={(e) => setName(e.target.value)} required />
        <TextField
          label="公式サイトURL（任意・取得のヒント）"
          value={websiteUrl}
          onChange={(e) => setWebsiteUrl(e.target.value)}
          placeholder="https://example.com"
        />

        <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleAiFetch}
            disabled={!name.trim() || aiLoading}
            startIcon={aiLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {aiLoading ? '取得中...' : 'AIで情報取得（プレビュー）'}
          </Button>
          <Typography variant="caption" color="text.secondary">
            gBiz不足時は AI Search Lite。追加前のプレビューなのでまだDBには保存されません。
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
        <TextField
          label="技術スタック"
          value={techStack}
          onChange={(e) => setTechStack(e.target.value)}
          placeholder="例: Go, TypeScript, React"
          multiline
          minRows={1}
        />
        <TextField
          label="福利厚生"
          value={welfareDetails}
          onChange={(e) => setWelfareDetails(e.target.value)}
          multiline
          minRows={2}
        />

        <Divider />

        <TextField select label="出典タイプ" value={sourceType} onChange={(e) => setSourceType(e.target.value)}>
          <MenuItem value="official">公式サイト</MenuItem>
          <MenuItem value="job_site">就活/転職サイト</MenuItem>
          <MenuItem value="gbizinfo">gBizINFO</MenuItem>
          <MenuItem value="web_search">Web検索</MenuItem>
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

        <Button
          variant="contained"
          onClick={handleCreate}
          disabled={!name.trim() || creating}
          startIcon={creating ? <CircularProgress size={16} color="inherit" /> : null}
        >
          {creating ? '追加中...' : '追加する（続けて基本情報画面へ）'}
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
