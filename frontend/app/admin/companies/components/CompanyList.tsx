'use client'

import Link from 'next/link'
import { Box, Button, Chip, Stack, Typography } from '@mui/material'
import { EmptyState } from '@/components/common/EmptyState'
import type { Company } from '../company-list-model'
import { CompanyRow, type CompanyRowProps } from './CompanyRow'

type Props = {
  companies: Company[]
  companyGroups: { key: string; label: string; items: Company[] }[]
  hasActiveFilters: boolean
  resetFilters: () => void
  /** 各行へそのまま渡す（company 以外） */
  rowProps: Omit<CompanyRowProps, 'company'>
}

/** 企業一覧の本体。0件なら空の状態、業界別表示なら業界ごとの見出しを付ける。 */
export function CompanyList({ companies, companyGroups, hasActiveFilters, resetFilters, rowProps }: Props) {
  const { isPlatform, groupByIndustry } = rowProps
  return (
    <Stack divider={<Box sx={{ borderBottom: '1px solid', borderColor: 'divider' }} />}>
      {companies.length === 0 && (
        <EmptyState
          title={hasActiveFilters ? '条件に一致する企業がありません' : 'まだ企業が登録されていません'}
          description={
            hasActiveFilters
              ? '絞り込みを変えるか、解除してもう一度お試しください。'
              : isPlatform
                ? '企業を追加すると、学生に見せる情報の取得を始められます。'
                : '担当校に公開できる企業がまだありません。追加はシステム管理者が行います。'
          }
          action={
            hasActiveFilters ? (
              <Button variant="outlined" size="small" onClick={resetFilters}>
                絞り込みを解除
              </Button>
            ) : isPlatform ? (
              <Button component={Link} href="/admin/companies/new" variant="outlined" size="small">
                最初の企業を追加
              </Button>
            ) : null
          }
        />
      )}

      {companyGroups.map((group) => (
        <Box key={group.key}>
          {groupByIndustry && group.label ? (
            <Box
              sx={{
                px: 2.5,
                py: 1,
                bgcolor: 'grey.100',
                borderBottom: '1px solid',
                borderColor: 'divider',
                position: 'sticky',
                top: 0,
                zIndex: 1,
              }}
            >
              <Stack direction="row" spacing={1} alignItems="center">
                <Typography variant="subtitle2" fontWeight={700}>
                  {group.label}
                </Typography>
                <Chip size="small" label={`${group.items.length} 社`} variant="outlined" />
              </Stack>
            </Box>
          ) : null}

          <Stack divider={<Box sx={{ borderBottom: '1px solid', borderColor: 'divider' }} />}>
            {group.items.map((company) => (
              <CompanyRow key={company.id} company={company} {...rowProps} />
            ))}
          </Stack>
        </Box>
      ))}
    </Stack>
  )
}
