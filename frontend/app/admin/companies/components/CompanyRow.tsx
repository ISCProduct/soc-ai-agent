'use client'

import type { Dispatch, SetStateAction } from 'react'
import Link from 'next/link'
import { Box, Button, Checkbox, Chip, CircularProgress, IconButton, Stack, Typography } from '@mui/material'
import MoreVertIcon from '@mui/icons-material/MoreVert'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import { approvalChipState } from '@/lib/admin/company-approval'
import { companyAspectHref } from '@/components/admin/CompanyAspectTabs'
import { FreshnessMark } from '@/components/admin/FreshnessMark'
import { companyFreshness } from '@/lib/admin/company-freshness'
import {
  ASPECT_PAGE,
  aspectLabel,
  canSelectForPublish,
  missingAspects,
  statusLabel,
  subtitleLine,
  type Company,
} from '../company-list-model'
import type { CompanyMenuAnchor } from '../hooks/useCompanyRowActions'

export type CompanyRowProps = {
  company: Company
  isPlatform: boolean
  groupByIndustry: boolean
  busy: boolean
  fetchPrimaryId: number | null
  selectedIds: number[]
  toggleSelect: (companyId: number) => void
  schoolId: number | undefined
  approvedCompanyIds: Set<number> | null
  approvalBusyId: number | null
  toggleCompanyApproval: (companyId: number, approved: boolean) => void
  setFilterIndustry: Dispatch<SetStateAction<string>>
  setPage: Dispatch<SetStateAction<number>>
  handleFetchPrimary: (companyId: number, force?: boolean) => void
  handlePublish: (companyId: number) => void
  setMenuAnchor: Dispatch<SetStateAction<CompanyMenuAnchor | null>>
}

/** 企業一覧の1行。状態チップ・足りない情報・各ページへのリンク・行の操作を出す。 */
export function CompanyRow({
  company,
  isPlatform,
  groupByIndustry,
  busy,
  fetchPrimaryId,
  selectedIds,
  toggleSelect,
  schoolId,
  approvedCompanyIds,
  approvalBusyId,
  toggleCompanyApproval,
  setFilterIndustry,
  setPage,
  handleFetchPrimary,
  handlePublish,
  setMenuAnchor,
}: CompanyRowProps) {
  const missing = missingAspects(company)
  const primaryMissing = missing.filter((k) => k !== 'jobs')
  const ready = primaryMissing.length === 0
  const fetching = fetchPrimaryId === company.id
  const isDraft = company.data_status !== 'published'
  const industryLabel = company.industry?.trim() || ''
  const locationLabel = company.location?.trim() || ''
  const meta =
    groupByIndustry || !industryLabel
      ? subtitleLine(company)
      : locationLabel
  const status = statusLabel(company.data_status)
  const selected = selectedIds.includes(company.id)
  // 欠損だけでなく TTL 超過も行頭で示す。既存表示では取得済みだが古いデータが
  // 見えず、バッチに再取得され続けていても気づけなかった。
  const fresh = companyFreshness(company)

  return (
    <Box
      sx={{
        px: 2.5,
        py: 2,
        bgcolor: selected ? 'action.selected' : undefined,
        '&:hover': { bgcolor: selected ? 'action.selected' : 'action.hover' },
      }}
    >
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={1.5}
        alignItems={{ md: 'center' }}
        justifyContent="space-between"
      >
        <Stack direction="row" spacing={1} alignItems="flex-start" sx={{ minWidth: 0, flex: 1 }}>
          <Box sx={{ pt: '3px', width: 20, mr: 0.5, flexShrink: 0, textAlign: 'center' }}>
            <FreshnessMark freshness={fresh.freshness} detail={fresh.detail} compact />
          </Box>
          {isPlatform && (
            <Checkbox
              checked={selected}
              onChange={() => toggleSelect(company.id)}
              disabled={busy || !canSelectForPublish(company)}
              inputProps={{ 'aria-label': `${company.name}を選択` }}
              sx={{ mt: -0.5 }}
            />
          )}
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Stack
              direction="row"
              spacing={1}
              alignItems="center"
              flexWrap="wrap"
              useFlexGap
              sx={{ mb: 0.5 }}
            >
              <Typography
                component={Link}
                href={companyAspectHref(company.id, 'info')}
                variant="subtitle1"
                fontWeight={700}
                sx={{
                  color: 'text.primary',
                  textDecoration: 'none',
                  '&:hover': { color: 'primary.main' },
                }}
              >
                {company.name}
              </Typography>
              <Chip label={status.label} color={status.color} size="small" />
              {isPlatform ? (
                <Chip
                  size="small"
                  variant="outlined"
                  label={company.is_verified ? '学生検索 審査済' : '学生検索 未審査'}
                />
              ) : null}
              {schoolId !== undefined ? (
                // 取得できていない状態を「未承認」と同じ見た目にしない(#1452)。
                approvalChipState(approvedCompanyIds, company.id) === 'unknown' ? (
                  <Chip label="承認状態 不明" color="warning" variant="outlined" size="small" />
                ) : (
                  (() => {
                    const approved = approvalChipState(approvedCompanyIds, company.id) === 'approved'
                    return (
                      <Chip
                        label={approved ? '承認済み' : '未承認'}
                        color={approved ? 'success' : 'default'}
                        variant={approved ? 'filled' : 'outlined'}
                        size="small"
                        disabled={approvalBusyId === company.id}
                        onClick={() => toggleCompanyApproval(company.id, approved)}
                        sx={{ cursor: 'pointer' }}
                      />
                    )
                  })()
                )
              ) : null}
              {!groupByIndustry && industryLabel ? (
                <Chip
                  size="small"
                  label={industryLabel}
                  variant="outlined"
                  onClick={() => {
                    setFilterIndustry(industryLabel)
                    setPage(1)
                  }}
                  sx={{ cursor: 'pointer' }}
                />
              ) : null}
            </Stack>

            {meta ? (
              <Typography variant="body2" color="text.secondary" sx={{ mb: 0.75 }}>
                {meta}
              </Typography>
            ) : null}

            <Stack
              direction="row"
              spacing={0.75}
              alignItems="center"
              flexWrap="wrap"
              useFlexGap
              sx={{ mb: 1 }}
            >
              {ready ? (
                <Chip
                  size="small"
                  icon={<CheckCircleOutlineIcon />}
                  label="公開の準備ができています"
                  color="success"
                  variant="outlined"
                />
              ) : (
                <>
                  <Chip
                    size="small"
                    icon={<ErrorOutlineIcon />}
                    label="まだ足りない情報があります"
                    color="warning"
                    variant="outlined"
                  />
                  {primaryMissing.map((key) => {
                    const page = ASPECT_PAGE[key]
                    const label = aspectLabel(key, company.industry)
                    if (!page) {
                      return (
                        <Chip key={key} size="small" label={label} variant="outlined" />
                      )
                    }
                    return (
                      <Chip
                        key={key}
                        size="small"
                        label={label}
                        variant="outlined"
                        component={Link}
                        href={companyAspectHref(company.id, page)}
                        clickable
                      />
                    )
                  })}
                </>
              )}
            </Stack>

            <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap>
              <Button
                component={Link}
                href={companyAspectHref(company.id, 'info')}
                size="small"
                variant="text"
              >
                会社概要
              </Button>
              <Button
                component={Link}
                href={companyAspectHref(company.id, 'tech')}
                size="small"
                variant="text"
              >
                技術情報
              </Button>
              <Button
                component={Link}
                href={companyAspectHref(company.id, 'relations')}
                size="small"
                variant="text"
              >
                関連企業
              </Button>
            </Stack>
          </Box>
        </Stack>

        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          justifyContent={{ xs: 'flex-start', md: 'flex-end' }}
          sx={{ flexShrink: 0, pl: { xs: 5, md: 0 } }}
        >
          {!ready ? (
            isPlatform ? (
            <Button
              variant="contained"
              size="small"
              color="secondary"
              onClick={() => handleFetchPrimary(company.id, false)}
              disabled={busy}
              startIcon={fetching ? <CircularProgress size={14} color="inherit" /> : null}
              disableElevation
            >
              {fetching ? '取得中…' : '情報を取得'}
            </Button>
            ) : null
          ) : isDraft && isPlatform ? (
            <Button
              variant="contained"
              size="small"
              color="success"
              onClick={() => handlePublish(company.id)}
              disabled={busy}
              disableElevation
            >
              学生に公開
            </Button>
          ) : null}

          {/* メニューの中身はすべてシステム管理者専用。空のメニューを開かせない */}
          {isPlatform && (
            <IconButton
              size="small"
              aria-label={`${company.name}のその他の操作`}
              disabled={busy && !fetching}
              onClick={(e) => setMenuAnchor({ el: e.currentTarget, company })}
            >
              <MoreVertIcon fontSize="small" />
            </IconButton>
          )}
        </Stack>
      </Stack>
    </Box>
  )
}
