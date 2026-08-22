'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  CircularProgress,
  Divider,
  IconButton,
  Typography,
} from '@mui/material'
import CloseIcon from '@mui/icons-material/Close'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import {
  buildCompanyDetailPath,
} from '@/lib/correlation-diagram-navigation'
import {
  fetchCompanySummary,
  formatEmployeeCount,
  marketColors,
  marketLabels,
  type CompanySummary,
  type MarketType,
} from '@/lib/company-data'

export const CORRELATION_DETAIL_PANEL_WIDTH = 360

type CorrelationCompanyDetailPanelProps = {
  companyId: number
  marketType?: MarketType
  onClose: () => void
}

export default function CorrelationCompanyDetailPanel({
  companyId,
  marketType = 'unlisted',
  onClose,
}: CorrelationCompanyDetailPanelProps) {
  const [company, setCompany] = useState<CompanySummary | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function load() {
      setLoading(true)
      setError(null)
      setCompany(null)
      try {
        const data = await fetchCompanySummary(companyId)
        if (!cancelled) setCompany(data)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : '企業情報の取得に失敗しました')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    void load()
    return () => {
      cancelled = true
    }
  }, [companyId])

  return (
    <Box
      component="aside"
      aria-label="企業情報"
      sx={{
        width: { xs: '100%', md: CORRELATION_DETAIL_PANEL_WIDTH },
        flexShrink: 0,
        height: '100%',
        borderLeft: { xs: 'none', md: '1px solid #ddd' },
        borderTop: { xs: '1px solid #ddd', md: 'none' },
        bgcolor: '#fff',
        display: 'flex',
        flexDirection: 'column',
        minHeight: 0,
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'flex-start',
          justifyContent: 'space-between',
          gap: 1,
          p: 2,
          pb: 1.5,
          flexShrink: 0,
        }}
      >
        <Box sx={{ minWidth: 0 }}>
          <Typography variant="overline" color="text.secondary" sx={{ lineHeight: 1.2 }}>
            企業情報
          </Typography>
          <Typography
            variant="h6"
            sx={{ fontSize: '1.05rem', fontWeight: 700, lineHeight: 1.35, wordBreak: 'break-word' }}
          >
            {company?.name ?? (loading ? '読み込み中…' : `企業 ${companyId}`)}
          </Typography>
        </Box>
        <IconButton aria-label="企業情報を閉じる" onClick={onClose} size="small">
          <CloseIcon fontSize="small" />
        </IconButton>
      </Box>

      <Divider />

      <Box sx={{ p: 2, flex: 1, overflow: 'auto', minHeight: 0 }}>
        {loading && (
          <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
            <CircularProgress size={28} />
          </Box>
        )}

        {!loading && error && (
          <Typography color="error" variant="body2">
            {error}
          </Typography>
        )}

        {!loading && !error && company && (
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.5 }}>
            <Box
              sx={{
                display: 'inline-flex',
                alignSelf: 'flex-start',
                px: 1,
                py: 0.25,
                borderRadius: 1,
                bgcolor: marketColors[marketType],
                color: '#fff',
                fontSize: 12,
                fontWeight: 600,
              }}
            >
              {marketLabels[marketType]}
            </Box>

            <InfoRow label="業界" value={company.industry} />
            <InfoRow label="所在地" value={company.location} />
            <InfoRow
              label="従業員数"
              value={formatEmployeeCount(company.employee_count, company.employee_count_basis) || undefined}
            />
            <InfoRow
              label="設立"
              value={company.founded_year != null ? `${company.founded_year}年` : undefined}
            />
            <InfoRow label="事業内容" value={company.main_business} />
            <InfoRow label="概要" value={company.description} multiline />

            {company.website_url && (
              <Typography variant="body2">
                <Box component="span" sx={{ color: 'text.secondary', display: 'block', mb: 0.25 }}>
                  Webサイト
                </Box>
                <Box
                  component="a"
                  href={company.website_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  sx={{ color: 'primary.main', wordBreak: 'break-all' }}
                >
                  {company.website_url}
                </Box>
              </Typography>
            )}
          </Box>
        )}
      </Box>

      <Box sx={{ p: 2, pt: 1, borderTop: '1px solid #eee', flexShrink: 0 }}>
        <Button
          component={Link}
          href={buildCompanyDetailPath(companyId)}
          variant="contained"
          fullWidth
          endIcon={<OpenInNewIcon />}
        >
          詳細ページを開く
        </Button>
      </Box>
    </Box>
  )
}

function InfoRow({
  label,
  value,
  multiline = false,
}: {
  label: string
  value?: string
  multiline?: boolean
}) {
  const display = value?.trim() ? value.trim() : '—'
  return (
    <Box>
      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 0.25 }}>
        {label}
      </Typography>
      <Typography
        variant="body2"
        sx={{
          whiteSpace: multiline ? 'pre-wrap' : 'normal',
          wordBreak: 'break-word',
          color: display === '—' ? 'text.disabled' : 'text.primary',
        }}
      >
        {display}
      </Typography>
    </Box>
  )
}
