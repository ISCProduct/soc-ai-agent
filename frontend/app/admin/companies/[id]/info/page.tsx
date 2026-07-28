'use client'

import { useEffect, useState } from 'react'
import { useParams } from 'next/navigation'
import {
  Alert,
  Button,
  Chip,
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
  const id = params.id as string

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [forceLoading, setForceLoading] = useState(false)
  const [confirmLoading, setConfirmLoading] = useState(false)
  const [previewPending, setPreviewPending] = useState(false)

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
  const [isProvisional, setIsProvisional] = useState(false)
  const [infoFetchedAt, setInfoFetchedAt] = useState<string | null>(null)
  const [jobsFetchedAt, setJobsFetchedAt] = useState<string | null>(null)
  const [techFetchedAt, setTechFetchedAt] = useState<string | null>(null)
  const [lastModelUsed, setLastModelUsed] = useState('')
  const [lastFetchConfidence, setLastFetchConfidence] = useState('')

  const loadCompany = () => {
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
        setTechStack(data.tech_stack || '')
        setWelfareDetails(data.welfare_details || '')
        setSourceType(data.source_type || 'manual')
        setSourceUrl(data.source_url || '')
        setDataStatus(data.data_status || 'draft')
        setIsProvisional(data.is_provisional ?? false)
        setInfoFetchedAt(data.info_fetched_at || null)
        setJobsFetchedAt(data.jobs_fetched_at || null)
        setTechFetchedAt(data.tech_fetched_at || null)
        setLastModelUsed(data.last_model_used || '')
        setLastFetchConfidence(data.last_fetch_confidence || '')
        setPreviewPending(false)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }

  useEffect(() => {
    loadCompany()
  }, [id])

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
      setPreviewPending(true)
      setSuccess('プレビュー取得が完了しました。内容を確認・修正してから「確定して保存」してください。')
    } finally {
      setAiLoading(false)
    }
  }

  const handleForceFetchAndSave = async () => {
    setForceLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await fetch(`/api/admin/companies/${id}/fetch-info?force=true`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || '強制再取得に失敗しました')
        return
      }
      applyInfoPayload(data)
      loadCompany()
      if (data.budget_exceeded) {
        setSuccess('月次 Search 予算超過のため、既存キャッシュのみ返却しました（新規 Search なし）。コスト画面を確認してください。')
      } else if (data.from_cache && data.skip_reason === 'ttl') {
        setSuccess('TTL 内のためキャッシュを返却しました。再取得する場合は「強制再取得して保存」を使ってください。')
      } else {
        setSuccess('DBへ強制再取得・保存しました。')
      }
    } finally {
      setForceLoading(false)
    }
  }

  const handleConfirmSave = async () => {
    setConfirmLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await fetch(`/api/admin/companies/${id}/confirm-info`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({
          description,
          industry,
          location,
          website_url: websiteUrl,
          founded_year: foundedYear ? parseInt(foundedYear, 10) : 0,
          employee_count: employeeCount ? parseInt(employeeCount, 10) : 0,
          main_business: mainBusiness,
          culture,
          work_style: workStyle,
          tech_stack: techStack,
          welfare_details: welfareDetails,
          source: sourceType,
          source_url: sourceUrl,
          model_used: lastModelUsed,
          confidence: lastFetchConfidence,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || '確定保存に失敗しました')
        return
      }
      applyInfoPayload(data)
      loadCompany()
      setSuccess('プレビュー内容を確定して保存しました（取得メタデータも更新済み）。')
    } finally {
      setConfirmLoading(false)
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
        tech_stack: techStack,
        welfare_details: welfareDetails,
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
    setPreviewPending(false)
    setSuccess('保存しました')
  }

  const isLowTrust =
    lastFetchConfidence === 'low' ||
    sourceType === 'llm_web_search' ||
    sourceType === 'ai_knowledge' ||
    sourceType === 'llm_extract'

  const formatTs = (v: string | null) => {
    if (!v) return '未取得'
    const d = new Date(v)
    return Number.isNaN(d.getTime()) ? v : d.toLocaleString('ja-JP')
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
      {isLowTrust && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          出典がモデル知識由来、または信頼度が low です。公式サイトURLを設定して強制再取得してください。
        </Alert>
      )}
      {previewPending && (
        <Alert severity="info" sx={{ mb: 2 }}>
          プレビュー未確定です。「確定して保存」で DB と取得メタデータ（info_fetched_at 等）を更新します。
        </Alert>
      )}
      {dataStatus === 'published' && (
        <Alert severity="success" sx={{ mb: 2 }}>
          この企業は公開中です。公開状態のままプレビュー取得・強制再取得で企業情報や公式URLを更新できます。
        </Alert>
      )}

      <Stack spacing={2}>
        <TextField label="企業名" value={name} onChange={(e) => setName(e.target.value)} required />

        <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleAiFetch}
            disabled={!name.trim() || aiLoading}
            startIcon={aiLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {aiLoading ? '取得中...' : 'プレビュー取得（未保存）'}
          </Button>
          <Button
            variant="contained"
            color="primary"
            onClick={handleConfirmSave}
            disabled={!previewPending || confirmLoading || !name.trim()}
            startIcon={confirmLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {confirmLoading ? '確定中...' : '確定して保存'}
          </Button>
          <Button
            variant="outlined"
            onClick={handleForceFetchAndSave}
            disabled={forceLoading}
            startIcon={forceLoading ? <CircularProgress size={16} color="inherit" /> : null}
          >
            {forceLoading ? '再取得中...' : '強制再取得して保存'}
          </Button>
          <Typography variant="caption" color="text.secondary">
            gBiz不足時は AI Search Lite。壁打ち/面接はDB共有キャッシュを読む（都度Searchしない）
          </Typography>
        </Stack>

        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Chip size="small" label={`source: ${sourceType || '-'}`} />
          <Chip size="small" label={`confidence: ${lastFetchConfidence || '-'}`} color={isLowTrust ? 'warning' : 'default'} />
          <Chip size="small" label={`model: ${lastModelUsed || '-'}`} />
        </Stack>
        <Typography variant="body2" color="text.secondary">
          info: {formatTs(infoFetchedAt)} / jobs: {formatTs(jobsFetchedAt)} / tech: {formatTs(techFetchedAt)}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          TTL: 基本情報 90日 / 求人 7日 / Tech 30日。月次 Search 上限はコスト画面で確認できます。
        </Typography>

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
          <MenuItem value="scrape">スクレイプ</MenuItem>
          <MenuItem value="web_search">Web検索</MenuItem>
          <MenuItem value="llm_extract">LLM抽出（安価・鮮度注意）</MenuItem>
          <MenuItem value="manual">手入力</MenuItem>
          <MenuItem value="llm_web_search">旧LLM知識（非推奨）</MenuItem>
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

        <Button variant="outlined" onClick={handleSave} disabled={!name.trim()}>
          手入力のみ保存（メタデータ更新なし）
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
