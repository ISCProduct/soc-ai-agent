'use client'

import Link from 'next/link'
import { Button, Stack } from '@mui/material'

export type CompanyAspect = 'info' | 'tech' | 'relations'

const ASPECTS: { key: CompanyAspect; label: string; path: (id: string | number) => string }[] = [
  { key: 'info', label: '会社概要', path: (id) => `/admin/companies/${id}/info` },
  { key: 'tech', label: '技術情報', path: (id) => `/admin/companies/${id}/edit` },
  { key: 'relations', label: '関連企業', path: (id) => `/admin/companies/${id}/relations` },
]

type CompanyAspectTabsProps = {
  companyId: string | number
  active: CompanyAspect
}

/** 企業詳細の3画面切替（会社概要 / 技術情報 / 関連企業）。 */
export function CompanyAspectTabs({ companyId, active }: CompanyAspectTabsProps) {
  return (
    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap sx={{ mb: 2 }}>
      {ASPECTS.map((aspect) => {
        const selected = aspect.key === active
        return (
          <Button
            key={aspect.key}
            component={Link}
            href={aspect.path(companyId)}
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
  const found = ASPECTS.find((a) => a.key === aspect)
  return found ? found.path(companyId) : `/admin/companies/${companyId}/info`
}
