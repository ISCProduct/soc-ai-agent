'use client'

import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'next/navigation'
import {
  Alert,
  Box,
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
import CompanyRelationGraph from '@/components/admin/CompanyRelationGraph'
import { groupRelationsByCategory, RELATION_CATEGORY_LABELS } from '@/lib/relation-graph'

type RelationEntry = {
  name: string
  relation_type: string
  ratio?: number
  description?: string
}

type MarketInfo = {
  is_listed: boolean
  market_type: string
  stock_code?: string
}

const RELATION_LABELS: Record<string, string> = {
  capital_subsidiary: '子会社',
  capital_affiliate: '関連会社',
  business_partner: '取引先',
  business_procurement: '調達（gBiz）',
  business_subsidy: '補助金（gBiz）',
}

export default function AdminCompanyRelationsPage() {
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
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [relations, setRelations] = useState<RelationEntry[]>([])
  const [marketInfo, setMarketInfo] = useState<MarketInfo>({
    is_listed: false,
    market_type: 'unlisted',
    stock_code: '',
  })
  const [sourceType, setSourceType] = useState('')
  const [lastModelUsed, setLastModelUsed] = useState('')
  const [lastFetchConfidence, setLastFetchConfidence] = useState('')
  const [relationsFetchedAt, setRelationsFetchedAt] = useState<string | null>(null)

  const loadCompany = () => {
    fetch(`/api/admin/companies/${id}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => {
        setName(data.name || '')
        setWebsiteUrl(data.website_url || '')
        setRelationsFetchedAt(data.relations_fetched_at || null)
        setPreviewPending(false)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }

  useEffect(() => {
    loadCompany()
  }, [id])

  const applyRelationsPayload = (data: Record<string, unknown>) => {
    if (Array.isArray(data.relations)) {
      setRelations(
        data.relations.map((r) => {
          const item = r as RelationEntry
          return {
            name: item.name || '',
            relation_type: item.relation_type || 'business_partner',
            ratio: item.ratio,
            description: item.description || '',
          }
        }),
      )
    }
    if (data.market_info && typeof data.market_info === 'object') {
      const m = data.market_info as MarketInfo
      setMarketInfo({
        is_listed: Boolean(m.is_listed),
        market_type: m.market_type || 'unlisted',
        stock_code: m.stock_code || '',
      })
    }
    if (typeof data.source === 'string' && data.source) setSourceType(data.source)
    if (typeof data.model_used === 'string' && data.model_used) setLastModelUsed(data.model_used)
    if (typeof data.confidence === 'string' && data.confidence) setLastFetchConfidence(data.confidence)
  }

  const handleAiFetch = async () => {
    if (!name.trim()) return
    setAiLoading(true)
    setError('')
    setSuccess('')
    try {
      const res = await fetch('/api/admin/companies/web-search-relations', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({ name, website_url: websiteUrl }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError((data?.error as string) || '関係情報取得に失敗しました')
        return
      }
      applyRelationsPayload(data)
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
      const res = await fetch(`/api/admin/companies/${id}/fetch-relations?force=true`, {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        setError((data?.error as string) || '強制再取得に失敗しました')
        return
      }
      applyRelationsPayload(data)
      loadCompany()
      if (data.budget_exceeded) {
        setSuccess('月次 Search 予算超過のため、既存キャッシュのみ返却しました（新規 Search なし）。コスト画面を確認してください。')
      } else if (data.from_cache && data.skip_reason === 'ttl') {
        setSuccess('TTL 内のためキャッシュを返却しました。再取得する場合は「強制再取得して保存」を使ってください。')
      } else {
        setSuccess(`DBへ強制再取得・保存しました（${data.saved_count ?? 0}件）。`)
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
      const res = await fetch(`/api/admin/companies/${id}/confirm-relations`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({
          relations,
          market_info: marketInfo,
          source: sourceType,
          model_used: lastModelUsed,
          confidence: lastFetchConfidence,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError((data?.error as string) || '確定保存に失敗しました')
        return
      }
      applyRelationsPayload(data)
      loadCompany()
      setSuccess(`プレビュー内容を確定して保存しました（${data.saved_count ?? 0}件、relations_fetched_at 更新済み）。`)
    } finally {
      setConfirmLoading(false)
    }
  }

  const updateRelation = (index: number, patch: Partial<RelationEntry>) => {
    setRelations((prev) => prev.map((r, i) => (i === index ? { ...r, ...patch } : r)))
  }

  const addRelation = () => {
    setRelations((prev) => [...prev, { name: '', relation_type: 'business_partner', description: '' }])
  }

  const removeRelation = (index: number) => {
    setRelations((prev) => prev.filter((_, i) => i !== index))
  }

  const formatTs = (v: string | null) => {
    if (!v) return '未取得'
    const d = new Date(v)
    return Number.isNaN(d.getTime()) ? v : d.toLocaleString('ja-JP')
  }

  const groupedRelations = useMemo(() => groupRelationsByCategory(relations), [relations])
  const numericId = Number(id)

  return (
    <AdminFormContainer
      title={`関係・市場情報: ${name}`}
      maxWidth={800}
      backLabel="← 企業一覧に戻る"
      backHref="/admin/companies"
    >
      <ErrorAlert error={error} />
      {success && <Alert severity="success" sx={{ mb: 2 }}>{success}</Alert>}
      {previewPending && (
        <Alert severity="info" sx={{ mb: 2 }}>
          プレビュー未確定です。「確定して保存」で company_relations / company_market_info と relations_fetched_at を更新します。
        </Alert>
      )}

      <Stack spacing={2}>
        <Typography variant="body2" color="text.secondary">
          gBizINFO の調達・補助金関係を優先し、不足分のみ AI Search Lite で補完します（#633 Phase 2）。
        </Typography>

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
            disabled={!previewPending || confirmLoading}
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
        </Stack>

        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Chip size="small" label={`source: ${sourceType || '-'}`} />
          <Chip size="small" label={`confidence: ${lastFetchConfidence || '-'}`} />
          <Chip size="small" label={`model: ${lastModelUsed || '-'}`} />
        </Stack>
        <Typography variant="body2" color="text.secondary">
          relations: {formatTs(relationsFetchedAt)}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          TTL: 関係・市場情報 60日。同一企業の同時取得は singleflight で1本化。月次 Search 上限はコスト画面で確認できます。
        </Typography>

        <Divider />

        <Typography variant="subtitle1" fontWeight="bold">市場情報（任意）</Typography>
        <Stack direction="row" spacing={2}>
          <TextField
            select
            label="上場区分"
            value={marketInfo.market_type}
            onChange={(e) => setMarketInfo({ ...marketInfo, market_type: e.target.value })}
            sx={{ flex: 1 }}
          >
            <MenuItem value="unlisted">非上場</MenuItem>
            <MenuItem value="prime">プライム</MenuItem>
            <MenuItem value="standard">スタンダード</MenuItem>
            <MenuItem value="growth">グロース</MenuItem>
          </TextField>
          <TextField
            label="証券コード"
            value={marketInfo.stock_code || ''}
            onChange={(e) => setMarketInfo({ ...marketInfo, stock_code: e.target.value })}
            sx={{ flex: 1 }}
          />
          <TextField
            select
            label="上場"
            value={marketInfo.is_listed ? 'yes' : 'no'}
            onChange={(e) => setMarketInfo({ ...marketInfo, is_listed: e.target.value === 'yes' })}
            sx={{ flex: 1 }}
          >
            <MenuItem value="no">非上場</MenuItem>
            <MenuItem value="yes">上場</MenuItem>
          </TextField>
        </Stack>

        <Divider />

        <Typography variant="subtitle1" fontWeight="bold">相関図（保存済みデータ）</Typography>
        {Number.isFinite(numericId) && numericId > 0 && <CompanyRelationGraph companyId={numericId} />}

        <Divider />

        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Typography variant="subtitle1" fontWeight="bold">企業関係</Typography>
          <Button size="small" variant="outlined" onClick={addRelation}>行を追加</Button>
        </Stack>

        {relations.length === 0 && (
          <Typography variant="body2" color="text.secondary">関係データがありません。プレビュー取得または行追加してください。</Typography>
        )}

        {groupedRelations.map((categoryGroup) => (
          <Box key={categoryGroup.category} sx={{ mb: 1 }}>
            <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
              {RELATION_CATEGORY_LABELS[categoryGroup.category]}
            </Typography>
            <Stack spacing={1.5}>
              {categoryGroup.companies.map((companyGroup) => (
                <Box
                  key={companyGroup.name}
                  sx={{ p: 2, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}
                >
                  <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
                    <Typography variant="body1" fontWeight="bold">{companyGroup.name}</Typography>
                    <Chip size="small" label={`${companyGroup.entries.length}件`} />
                  </Stack>
                  <Stack spacing={1.5} divider={<Divider flexItem />}>
                    {companyGroup.entries.map(({ index, relation: rel }) => (
                      <Stack key={index} spacing={1.5}>
                        <Stack direction="row" spacing={1}>
                          <TextField
                            label="関連企業名"
                            value={rel.name}
                            onChange={(e) => updateRelation(index, { name: e.target.value })}
                            sx={{ flex: 2 }}
                          />
                          <TextField
                            select
                            label="関係タイプ"
                            value={rel.relation_type}
                            onChange={(e) => updateRelation(index, { relation_type: e.target.value })}
                            sx={{ flex: 1 }}
                          >
                            <MenuItem value="capital_subsidiary">子会社</MenuItem>
                            <MenuItem value="capital_affiliate">関連会社</MenuItem>
                            <MenuItem value="business_partner">取引先</MenuItem>
                            <MenuItem value="business_procurement">調達（gBiz）</MenuItem>
                            <MenuItem value="business_subsidy">補助金（gBiz）</MenuItem>
                          </TextField>
                        </Stack>
                        <Stack direction="row" spacing={1} alignItems="center">
                          <TextField
                            label="説明"
                            value={rel.description || ''}
                            onChange={(e) => updateRelation(index, { description: e.target.value })}
                            sx={{ flex: 1 }}
                          />
                          <Chip size="small" label={RELATION_LABELS[rel.relation_type] || rel.relation_type} />
                          <Button size="small" color="error" onClick={() => removeRelation(index)}>削除</Button>
                        </Stack>
                      </Stack>
                    ))}
                  </Stack>
                </Box>
              ))}
            </Stack>
          </Box>
        ))}
      </Stack>
    </AdminFormContainer>
  )
}
