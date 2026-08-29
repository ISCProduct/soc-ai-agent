'use client'

import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'next/navigation'
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
import { infoFieldEnabled, resolveIndustryFieldProfile } from '@/lib/admin-company-field-profile'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { CompanyAspectTabs } from '@/components/admin/CompanyAspectTabs'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import { applyInfoPayload, WORK_STYLE_OPTIONS } from '@/lib/admin-company-form'

export default function PageContent() {
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
  const [relationsFetchedAt, setRelationsFetchedAt] = useState<string | null>(null)
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
        setRelationsFetchedAt(data.relations_fetched_at || null)
        setLastModelUsed(data.last_model_used || '')
        setLastFetchConfidence(data.last_fetch_confidence || '')
        setPreviewPending(false)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }

  useEffect(() => {
    loadCompany()
  }, [id])

  const infoSetters = {
    setDescription, setIndustry, setLocation, setWebsiteUrl,
    setFoundedYear, setEmployeeCount, setMainBusiness, setCulture,
    setWorkStyle, setTechStack, setWelfareDetails, setSourceType,
    setSourceUrl, setLastModelUsed, setLastFetchConfidence,
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
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
      if (!res.ok) {
        setError(
          (typeof data.error === 'string' && data.error) ||
            '企業情報の取得に失敗しました。時間をおいて再度お試しください。',
        )
        return
      }
      applyInfoPayload(data, infoSetters)
      setPreviewPending(true)
      setSuccess('プレビュー取得が完了しました。内容を確認・修正してから「確定して保存」してください。')
    } catch {
      setError('企業情報の取得中に通信エラーが発生しました。時間をおいて再度お試しください。')
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
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
      if (!res.ok) {
        setError(
          (typeof data.error === 'string' && data.error) ||
            '強制再取得に失敗しました。時間をおいて再度お試しください。',
        )
        return
      }
      applyInfoPayload(data, infoSetters)
      loadCompany()
      if (data.budget_exceeded) {
        setSuccess('月次 Search 予算超過のため、既存キャッシュのみ返却しました（新規 Search なし）。コスト画面を確認してください。')
      } else if (data.from_cache && data.skip_reason === 'ttl') {
        setSuccess('TTL 内のためキャッシュを返却しました。再取得する場合は「強制再取得して保存」を使ってください。')
      } else {
        setSuccess('DBへ強制再取得・保存しました。')
      }
    } catch {
      setError('強制再取得中に通信エラーが発生しました。時間をおいて再度お試しください。')
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
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
      if (!res.ok) {
        setError(
          (typeof data.error === 'string' && data.error) ||
            '確定保存に失敗しました。時間をおいて再度お試しください。',
        )
        return
      }
      applyInfoPayload(data, infoSetters)
      loadCompany()
      setSuccess('プレビュー内容を確定して保存しました。')
    } catch {
      setError('確定保存中に通信エラーが発生しました。時間をおいて再度お試しください。')
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

  const profile = useMemo(() => resolveIndustryFieldProfile(industry), [industry])
  const show = (key: Parameters<typeof infoFieldEnabled>[1]) => infoFieldEnabled(profile, key)

  return (
    <AdminFormContainer
      title={`${name || '企業'}（会社概要）`}
      description={`会社の基本情報を確認・編集します。業種「${profile.label}」に合わせて、関連する入力画面が変わります。`}
      maxWidth={900}
      backLabel="企業一覧に戻る"
      backHref="/admin/companies"
    >
      <CompanyAspectTabs companyId={id} active="info" industry={industry} />
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}
      {isLowTrust && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          出典の信頼度が低めです。公式サイトURLを設定してから、もう一度取得してください。
        </Alert>
      )}
      {previewPending && (
        <Alert severity="info" sx={{ mb: 2 }}>
          まだ下書きの取得結果です。内容を確認してから「確定して保存」を押してください。
        </Alert>
      )}
      {dataStatus === 'published' && (
        <Alert severity="success" sx={{ mb: 2 }}>
          この企業は公開中です。公開したまま情報を更新できます。
        </Alert>
      )}
      {!profile.techAspectEnabled && (
        <Alert severity="info" sx={{ mb: 2 }}>
          この業種では「技術情報」の登録は不要です。会社概要と関連企業を確認すれば公開準備ができます。
        </Alert>
      )}

      <Stack spacing={2}>
        <TextField label="企業名" value={name} onChange={(e) => setName(e.target.value)} required />

        <Stack direction="row" alignItems="center" spacing={1} flexWrap="wrap" useFlexGap>
          <Button
            variant="outlined"
            color="secondary"
            onClick={handleAiFetch}
            disabled={!name.trim() || aiLoading || forceLoading}
            startIcon={aiLoading ? <CircularProgress size={14} color="inherit" /> : null}
          >
            {aiLoading ? '取得中...' : 'プレビュー取得（未保存）'}
          </Button>
          <Button
            variant="contained"
            color="primary"
            size="small"
            onClick={handleConfirmSave}
            disabled={!previewPending || confirmLoading || !name.trim()}
            startIcon={confirmLoading ? <CircularProgress size={14} color="inherit" /> : null}
          >
            {confirmLoading ? '確定中...' : '確定して保存'}
          </Button>
          <Button
            variant="outlined"
            onClick={handleForceFetchAndSave}
            disabled={forceLoading || aiLoading}
            startIcon={forceLoading ? <CircularProgress size={14} color="inherit" /> : null}
          >
            {forceLoading ? '再取得中...' : '強制再取得して保存'}
          </Button>
        </Stack>

        <Typography variant="body2" color="text.secondary">
          最終取得: 会社概要 {formatTs(infoFetchedAt)}
          {profile.techAspectEnabled ? ` ／ ${profile.techAspectLabel} ${formatTs(techFetchedAt)}` : ''}
          {' ／ '}関連企業 {formatTs(relationsFetchedAt)}
        </Typography>

        <Divider />

        {show('description') && (
          <TextField
            label="企業概要"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            multiline
            minRows={2}
          />
        )}
        {show('industry') && (
          <TextField
            label="業種"
            value={industry}
            onChange={(e) => setIndustry(e.target.value)}
            helperText={`判定中のプロファイル: ${profile.label}${profile.techAspectEnabled ? `（「${profile.techAspectLabel}」タブあり）` : '（技術タブなし）'}`}
          />
        )}
        {show('location') && (
          <TextField label="所在地" value={location} onChange={(e) => setLocation(e.target.value)} />
        )}
        {show('website_url') && (
          <TextField label="公式サイトURL" value={websiteUrl} onChange={(e) => setWebsiteUrl(e.target.value)} />
        )}
        {(show('founded_year') || show('employee_count')) && (
          <Stack direction="row" spacing={2}>
            {show('founded_year') && (
              <TextField
                label="設立年"
                value={foundedYear}
                onChange={(e) => setFoundedYear(e.target.value)}
                type="number"
                sx={{ flex: 1 }}
              />
            )}
            {show('employee_count') && (
              <TextField
                label="従業員数"
                value={employeeCount}
                onChange={(e) => setEmployeeCount(e.target.value)}
                type="number"
                sx={{ flex: 1 }}
                helperText="取得時は連結を優先し、単体／連結を保存します"
              />
            )}
          </Stack>
        )}
        {show('main_business') && (
          <TextField
            label="主要事業内容"
            value={mainBusiness}
            onChange={(e) => setMainBusiness(e.target.value)}
            multiline
            minRows={2}
          />
        )}
        {show('culture') && (
          <TextField
            label="企業文化"
            value={culture}
            onChange={(e) => setCulture(e.target.value)}
            multiline
            minRows={2}
          />
        )}
        {show('work_style') && (
          <TextField
            select
            label="勤務スタイル"
            value={workStyle}
            onChange={(e) => setWorkStyle(e.target.value)}
          >
            <MenuItem value="">未設定</MenuItem>
            {WORK_STYLE_OPTIONS.map((opt) => (
              <MenuItem key={opt} value={opt}>{opt}</MenuItem>
            ))}
          </TextField>
        )}
        {show('tech_stack') && (
          <TextField
            label="技術スタック"
            value={techStack}
            onChange={(e) => setTechStack(e.target.value)}
            placeholder="例: Go, TypeScript, React"
            multiline
            minRows={1}
          />
        )}
        {show('welfare_details') && (
          <TextField
            label="福利厚生"
            value={welfareDetails}
            onChange={(e) => setWelfareDetails(e.target.value)}
            multiline
            minRows={2}
          />
        )}

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
