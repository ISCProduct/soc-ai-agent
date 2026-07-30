'use client'

import Link from 'next/link'
import { Button, Stack } from '@mui/material'
import { resolveIndustryFieldProfile } from '@/lib/admin-company-field-profile'

export type CompanyAspect = 'info' | 'tech' | 'relations'

const ASPECT_PATHS: Record<CompanyAspect, (id: string | number) => string> = {
  info: (id) => `/admin/companies/${id}/info`,
  tech: (id) => `/admin/companies/${id}/edit`,
  relations: (id) => `/admin/companies/${id}/relations`,
}

type CompanyAspectTabsProps = {
  companyId: string | number
  active: CompanyAspect
  /** 業界名。省略時は技術タブも表示（従来互換） */
  industry?: string | null
}

/** 企業詳細の画面切替（会社概要 / 技術・設備 / 関連企業）。業界により技術タブの表示・ラベルが変わる。 */
export function CompanyAspectTabs({ companyId, active, industry }: CompanyAspectTabsProps) {
  const profile = resolveIndustryFieldProfile(industry)
  const aspects: { key: CompanyAspect; label: string }[] = [
    { key: 'info', label: '会社概要' },
  ]
  if (profile.techAspectEnabled) {
    aspects.push({ key: 'tech', label: profile.techAspectLabel })
  }
  aspects.push({ key: 'relations', label: '関連企業' })

  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mb: 2 }}>
      {aspects.map((aspect) => {
        const selected = aspect.key === active
        return (
          <Button
            key={aspect.key}
            component={Link}
            href={ASPECT_PATHS[aspect.key](companyId)}
            size="small"
            variant={selected ? 'contained' : 'outlined'}
            color={selected ? 'primary' : 'inherit'}
            disableElevation
            sx={{ textTransform: 'none' }}
          >
            {aspect.label}
          </Button>
        )
      })}
    </Stack>
  )
}

export function companyAspectHref(companyId: string | number, aspect: CompanyAspect): string {
  return ASPECT_PATHS[aspect](companyId)
}
