'use client'

import type { ReactNode } from 'react'
import { Button, Card, CardContent } from '@mui/material'
import { PageContainer, ADMIN_PAGE_WIDTH } from './PageContainer'
import { AdminPageHeader } from './AdminPageHeader'

type AdminFormContainerProps = {
  title: string
  description?: string
  children: ReactNode
  maxWidth?: number
  backHref?: string
  backLabel?: string
  onBack?: () => void
  actions?: ReactNode
}

/**
 * 管理画面のフォームページ共通シェル。
 */
export function AdminFormContainer({
  title,
  description,
  children,
  maxWidth = ADMIN_PAGE_WIDTH.form,
  backHref,
  backLabel = '一覧に戻る',
  onBack,
  actions,
}: AdminFormContainerProps) {
  return (
    <PageContainer maxWidth={maxWidth}>
      <AdminPageHeader
        title={title}
        description={description}
        backHref={onBack ? undefined : backHref}
        backAriaLabel={backLabel}
        actions={
          <>
            {onBack ? (
              <Button variant="outlined" size="small" onClick={onBack}>
                {backLabel}
              </Button>
            ) : null}
            {actions}
          </>
        }
      />
      <Card
        elevation={0}
        sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}
      >
        <CardContent sx={{ p: { xs: 2, sm: 3 } }}>{children}</CardContent>
      </Card>
    </PageContainer>
  )
}
