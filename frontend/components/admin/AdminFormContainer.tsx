import Link from 'next/link'
import { Button, Card, CardContent, Stack, Typography } from '@mui/material'
import { type ReactNode } from 'react'
import { PageContainer } from './PageContainer'

type AdminFormContainerProps = {
  title: string
  children: ReactNode
  maxWidth?: number
  backHref?: string
  backLabel?: string
  onBack?: () => void
}

export function AdminFormContainer({
  title,
  children,
  maxWidth = 700,
  backHref,
  backLabel = '一覧に戻る',
  onBack,
}: AdminFormContainerProps) {
  return (
    <PageContainer maxWidth={maxWidth}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 3 }}>
        <Typography variant="h4" fontWeight="bold">
          {title}
        </Typography>
        {onBack ? (
          <Button variant="outlined" size="small" onClick={onBack}>
            {backLabel}
          </Button>
        ) : (
          backHref && (
            <Button variant="outlined" size="small" component={Link} href={backHref}>
              {backLabel}
            </Button>
          )
        )}
      </Stack>
      <Card>
        <CardContent>{children}</CardContent>
      </Card>
    </PageContainer>
  )
}

