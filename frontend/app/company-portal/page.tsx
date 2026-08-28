'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Box,
  Button,
  Card,
  CardContent,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company-auth'

export default function CompanyPortalDashboardPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [userName, setUserName] = useState('')
  const [companyId, setCompanyId] = useState<number | null>(null)

  useEffect(() => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    companyAuthService.fetchMe()
      .then((me) => {
        setUserName(me.name)
        setCompanyId(me.company_id)
      })
      .catch(() => {
        companyAuthService.logout()
        router.replace('/company-portal/sign-in')
      })
      .finally(() => setLoading(false))
  }, [router])

  if (loading) {
    return <PageLoading message="ダッシュボードを読み込んでいます..." />
  }

  return (
    <PageContainer maxWidth={720}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        企業ポータル
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        採用担当者向けの管理画面です。
      </Typography>

      <Card elevation={0} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px' }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            ようこそ、{userName} さん
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            自社ID: {companyId}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            スカウト送信や学生検索などの機能は今後追加予定です。
          </Typography>
        </CardContent>
      </Card>

      <Box sx={{ mt: 3 }}>
        <Button
          variant="outlined"
          onClick={() => {
            companyAuthService.logout()
            router.push('/company-portal/sign-in')
          }}
        >
          ログアウト
        </Button>
      </Box>
    </PageContainer>
  )
}
