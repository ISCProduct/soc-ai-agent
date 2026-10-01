'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Alert,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Typography,
} from '@mui/material'
import Grid from '@mui/material/Grid'
import { authService } from '@/lib/auth'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'
import { visibleAdminNav } from '@/lib/admin-nav'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

const cardSx = {
  height: '100%',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: '10px',
} as const

type HubCard = {
  title: string
  description: string
  href: string
  cta: string
  chip?: string | null
}

function HubSection({
  heading,
  cards,
}: {
  heading: string
  cards: HubCard[]
}) {
  if (cards.length === 0) return null
  return (
    <>
      <Typography variant="h6" sx={{ mt: 1, mb: 1.5 }}>
        {heading}
      </Typography>
      <Grid container spacing={2} sx={{ mb: 3 }}>
        {cards.map((card) => (
          <Grid key={card.href + card.title} size={{ xs: 12, md: 6 }}>
            <Card elevation={0} sx={cardSx}>
              <CardContent>
                <Typography variant="h6" gutterBottom>
                  {card.title}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                  {card.description}
                </Typography>
                {card.chip != null && (
                  <Chip label={card.chip} size="small" sx={{ mb: 2 }} />
                )}
                <Divider sx={{ mb: 2 }} />
                <Button variant="contained" component={Link} href={card.href}>
                  {card.cta}
                </Button>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>
    </>
  )
}

export default function PageContent() {
  const [companyCount, setCompanyCount] = useState<number | null>(null)
  const [restricted, setRestricted] = useState<boolean | null>(null)
  const [accessError, setAccessError] = useState<string | null>(null)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  useEffect(() => {
    const load = async () => {
      try {
        const access = await getAdminSchoolAccess()
        setRestricted(access.restricted)
      } catch {
        setAccessError('権限情報の取得に失敗しました')
        setRestricted(true)
      }
      try {
        const companiesRes = await fetch('/api/admin/companies', {
          headers: authService.getAdminFetchHeaders(),
        })
        const companiesData = await companiesRes.json()
        setCompanyCount(companiesData?.companies?.length ?? 0)
      } catch {
        setCompanyCount(null)
      }
    }
    void load()
  }, [])

  const isPlatform = restricted === false
  const title = isPlatform ? 'システム管理メニュー' : '学校運営・教員メニュー'
  const description = isPlatform
    ? 'プラットフォーム横断の設定・監視・バッチ操作を行います。'
    : '担当校の学生対応と学校運営（企業・求人・選考）を行います。'

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader title={title} description={description} backHref="/" />

      {accessError && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          {accessError}
        </Alert>
      )}

      {restricted === null && !accessError && (
        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
          権限を確認しています…
        </Typography>
      )}

      {visibleAdminNav(restricted === null ? null : isPlatform).map((group) => (
        <HubSection
          key={group.heading}
          heading={group.heading}
          cards={group.items.map((item) => ({
            ...item,
            // 企業情報だけ登録数を添える。読み込み中は件数の代わりにその旨を出す。
            chip:
              item.href === '/admin/companies'
                ? companyCount === null
                  ? '読み込み中'
                  : `登録数 ${companyCount}`
                : null,
          }))}
        />
      ))}
    </PageContainer>
  )
}
