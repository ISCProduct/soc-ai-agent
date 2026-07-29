'use client'

import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'next/navigation'
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
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
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { authService } from '@/lib/auth'
import { sanitizeRelationDescription, isClearOrganizationName } from '@/lib/relation-labels'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel } from '@/components/admin/AdminPanel'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { CompanyAspectTabs } from '@/components/admin/CompanyAspectTabs'
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
  const [industry, setIndustry] = useState('')
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
  const [graphRefreshKey, setGraphRefreshKey] = useState(0)

  const loadCompany = () => {
    fetch(`/api/admin/companies/${id}`, {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => {
        setName(data.name || '')
        setIndustry(data.industry || '')
        setWebsiteUrl(data.website_url || '')
        setRelationsFetchedAt(data.relations_fetched_at || null)
        setPreviewPending(false)
      })
      .catch(() => setError('企業情報の取得に失敗しました'))
  }

  const applyRelationsPayload = (data: Record<string, unknown>) => {
    if (Array.isArray(data.relations)) {
      setRelations(
        data.relations
          .map((r) => {
            const item = r as RelationEntry
            return {
              name: item.name || '',
              relation_type: item.relation_type || 'business_partner',
              ratio: item.ratio,
              // 種別ラベル（主要取引先など）は説明欄に入れない。取引内容だけ残す。
              description: sanitizeRelationDescription(item.description || ''),
            }
          })
          .filter((r) => isClearOrganizationName(r.name)),
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

  const loadSavedRelations = () => {
    fetch(`/api/admin/companies/${id}/fetch-relations`, {
      method: 'POST',
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => (r.ok ? r.json() : null))
      .then((data: Record<string, unknown> | null) => {
        if (data) applyRelationsPayload(data)
      })
      .catch(() => {
        // 保存済みが無い場合は空のまま
      })
  }

  useEffect(() => {
    loadCompany()
    loadSavedRelations()
  }, [id])

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
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
      if (!res.ok) {
        setError(
          (typeof data.error === 'string' && data.error) ||
            '関係情報の取得に失敗しました。時間をおいて再度お試しください。',
        )
        return
      }
      applyRelationsPayload(data)
      setPreviewPending(true)
      setSuccess('プレビュー取得が完了しました。内容を確認・修正してから「確定して保存」してください。')
    } catch {
      setError('関係情報の取得中に通信エラーが発生しました。時間をおいて再度お試しください。')
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
      const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
      if (!res.ok) {
        setError(
          (typeof data.error === 'string' && data.error) ||
            '強制再取得に失敗しました。時間をおいて再度お試しください。',
        )
        return
      }
      applyRelationsPayload(data)
      loadCompany()
      setGraphRefreshKey((k) => k + 1)
      if (data.budget_exceeded) {
        setSuccess('月次の情報取得上限に達しているため、保存済みの情報のみ表示しています。コスト画面を確認するか、時間をおいて再度お試しください。')
      } else if (data.from_cache && data.skip_reason === 'ttl') {
        setSuccess('最近取得した情報があるため、保存済みの内容を表示しています。最新にしたい場合は「最新の情報に更新」を使ってください。')
      } else {
        setSuccess(`関連企業情報を更新して保存しました（${data.saved_count ?? 0}件）。`)
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
      const res = await fetch(`/api/admin/companies/${id}/confirm-relations`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          ...authService.getAdminFetchHeaders(),
        },
        body: JSON.stringify({
          relations: relations
            .filter((r) => isClearOrganizationName(r.name || ''))
            .map((r) => ({
              ...r,
              description: sanitizeRelationDescription(r.description || ''),
            })),
          market_info: marketInfo,
          source: sourceType,
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
      applyRelationsPayload(data)
      loadCompany()
      setGraphRefreshKey((k) => k + 1)
      setSuccess(`内容を確定して保存しました（${data.saved_count ?? 0}件）。`)
      setPreviewPending(false)
    } catch {
      setError('確定保存中に通信エラーが発生しました。時間をおいて再度お試しください。')
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
  const busy = aiLoading || forceLoading || confirmLoading

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.full}>
      <AdminPageHeader
        title={`${name || '企業'}（関連企業）`}
        description="関連企業のつながりを図で確認し、必要なら一覧で修正して保存します。"
        backHref="/admin/companies"
        backAriaLabel="企業一覧に戻る"
      />

      <CompanyAspectTabs companyId={id} active="relations" industry={industry} />
      <ErrorAlert error={error} />
      {success && (
        <Alert severity="success" sx={{ mb: 2 }} onClose={() => setSuccess('')}>
          {success}
        </Alert>
      )}
      {previewPending && (
        <Alert severity="info" sx={{ mb: 2 }}>
          まだ下書きの取得結果です。内容を確認してから「確定して保存」を押してください。下書きの間は上の相関図は更新されません。
        </Alert>
      )}

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
              最終取得: {formatTs(relationsFetchedAt)}
              {previewPending ? ' ／ いまは下書きを編集中です' : ''}
            </Typography>
          </Box>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button
              variant="outlined"
              color="secondary"
              onClick={handleAiFetch}
              disabled={!name.trim() || busy}
              startIcon={aiLoading ? <CircularProgress size={16} color="inherit" /> : null}
            >
              {aiLoading ? '取得中…' : '情報を取得して確認'}
            </Button>
            <Button
              variant="contained"
              color="primary"
              onClick={handleConfirmSave}
              disabled={!previewPending || busy}
              startIcon={confirmLoading ? <CircularProgress size={16} color="inherit" /> : null}
              disableElevation
            >
              {confirmLoading ? '保存中…' : '確定して保存'}
            </Button>
            <Button
              variant="outlined"
              onClick={handleForceFetchAndSave}
              disabled={busy}
              startIcon={forceLoading ? <CircularProgress size={16} color="inherit" /> : null}
            >
              {forceLoading ? '更新中…' : '最新の情報に更新'}
            </Button>
          </Stack>
        </Stack>
      </Box>

      <Stack spacing={2.5}>
        <AdminPanel
          title="関連企業のつながり"
          headerRight={
            <Typography variant="body2" color="text.secondary">
              保存済みの資本関係を図で表示します
            </Typography>
          }
        >
          <Box sx={{ px: { xs: 1.5, sm: 2 }, py: 2 }}>
            {Number.isFinite(numericId) && numericId > 0 ? (
              <CompanyRelationGraph key={graphRefreshKey} companyId={numericId} />
            ) : (
              <Typography variant="body2" color="text.secondary">
                企業を読み込み中です…
              </Typography>
            )}
          </Box>
        </AdminPanel>

        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', lg: 'minmax(0, 1.6fr) minmax(280px, 0.9fr)' },
            gap: 2.5,
            alignItems: 'start',
          }}
        >
          <AdminPanel
            title="関連企業の一覧"
            headerRight={
              <Button size="small" variant="outlined" onClick={addRelation} disabled={busy}>
                行を追加
              </Button>
            }
          >
            <Box sx={{ px: 2.5, py: 2 }}>
              {relations.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  まだ関連企業がありません。「情報を取得して確認」か「行を追加」で登録できます。
                </Typography>
              ) : (
                <Stack spacing={2}>
                  {groupedRelations.map((categoryGroup) => (
                    <Box key={categoryGroup.category}>
                      <Typography variant="subtitle2" color="text.secondary" sx={{ mb: 1 }}>
                        {RELATION_CATEGORY_LABELS[categoryGroup.category]}
                      </Typography>
                      <Stack spacing={1.5}>
                        {categoryGroup.companies.map((companyGroup) => (
                          <Box
                            key={companyGroup.name}
                            sx={{ p: 2, border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}
                          >
                            <Stack direction="row" alignItems="center" spacing={1} sx={{ mb: 1.5 }}>
                              <Typography variant="body1" fontWeight={700}>
                                {companyGroup.name}
                              </Typography>
                              <Chip size="small" label={`${companyGroup.entries.length}件`} />
                            </Stack>
                            <Stack spacing={1.5} divider={<Divider flexItem />}>
                              {companyGroup.entries.map(({ index, relation: rel }) => (
                                <Stack key={index} spacing={1.5}>
                                  <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1}>
                                    <TextField
                                      label="関連企業名"
                                      value={rel.name}
                                      onChange={(e) => updateRelation(index, { name: e.target.value })}
                                      sx={{ flex: 2 }}
                                      size="small"
                                    />
                                    <TextField
                                      select
                                      label="関係の種類"
                                      value={rel.relation_type}
                                      onChange={(e) => updateRelation(index, { relation_type: e.target.value })}
                                      sx={{ flex: 1, minWidth: 140 }}
                                      size="small"
                                    >
                                      <MenuItem value="capital_subsidiary">子会社</MenuItem>
                                      <MenuItem value="capital_affiliate">関連会社</MenuItem>
                                      <MenuItem value="business_partner">取引先</MenuItem>
                                      <MenuItem value="business_procurement">調達（gBiz）</MenuItem>
                                      <MenuItem value="business_subsidy">補助金（gBiz）</MenuItem>
                                    </TextField>
                                  </Stack>
                                  <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ sm: 'flex-start' }}>
                                    <TextField
                                      label="取引内容・関係の内容"
                                      value={rel.description || ''}
                                      onChange={(e) => updateRelation(index, { description: e.target.value })}
                                      placeholder={
                                        rel.relation_type === 'business_partner'
                                          ? '例: 決済代行（空なら表示は「主要取引先」）'
                                          : '例: 完全子会社'
                                      }
                                      helperText={
                                        rel.relation_type === 'business_partner'
                                          ? rel.description?.trim()
                                            ? '具体的な取引内容（推定でも可）'
                                            : '未入力のため一覧・図では「主要取引先」と表示されます'
                                          : '出資比率・提携内容など、分かる範囲で具体的に'
                                      }
                                      sx={{ flex: 1 }}
                                      size="small"
                                      multiline
                                      minRows={2}
                                    />
                                    <Chip
                                      size="small"
                                      label={
                                        rel.relation_type === 'business_partner' && !rel.description?.trim()
                                          ? '主要取引先'
                                          : RELATION_LABELS[rel.relation_type] || rel.relation_type
                                      }
                                      sx={{ mt: { sm: 1 } }}
                                    />
                                    <Button size="small" color="error" onClick={() => removeRelation(index)} sx={{ mt: { sm: 1 } }}>
                                      削除
                                    </Button>
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
              )}
            </Box>
          </AdminPanel>

          <Stack spacing={2.5}>
            <AdminPanel title="市場情報（任意）">
              <Box sx={{ px: 2.5, py: 2 }}>
                <Stack spacing={2}>
                  <TextField
                    select
                    label="上場区分"
                    value={marketInfo.market_type}
                    onChange={(e) => setMarketInfo({ ...marketInfo, market_type: e.target.value })}
                    fullWidth
                    size="small"
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
                    fullWidth
                    size="small"
                  />
                  <TextField
                    select
                    label="上場"
                    value={marketInfo.is_listed ? 'yes' : 'no'}
                    onChange={(e) => setMarketInfo({ ...marketInfo, is_listed: e.target.value === 'yes' })}
                    fullWidth
                    size="small"
                  >
                    <MenuItem value="no">非上場</MenuItem>
                    <MenuItem value="yes">上場</MenuItem>
                  </TextField>
                </Stack>
              </Box>
            </AdminPanel>

            <Accordion
              disableGutters
              elevation={0}
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: '10px !important',
                '&:before': { display: 'none' },
                overflow: 'hidden',
              }}
            >
              <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                <Box>
                  <Typography fontWeight={700}>詳しい取得情報</Typography>
                  <Typography variant="body2" color="text.secondary">
                    通常は開かなくて大丈夫です
                  </Typography>
                </Box>
              </AccordionSummary>
              <AccordionDetails>
                <Stack spacing={1}>
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    <Chip size="small" label={`取得元: ${sourceType || '-'}`} />
                    <Chip size="small" label={`確信度: ${lastFetchConfidence || '-'}`} />
                    <Chip size="small" label={`モデル: ${lastModelUsed || '-'}`} />
                  </Stack>
                  <Typography variant="body2" color="text.secondary">
                    関連情報の最終取得: {formatTs(relationsFetchedAt)}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    関係・市場情報は約60日間キャッシュされます。月次の情報取得上限はコスト画面で確認できます。
                  </Typography>
                </Stack>
              </AccordionDetails>
            </Accordion>
          </Stack>
        </Box>
      </Stack>
    </PageContainer>
  )
}
