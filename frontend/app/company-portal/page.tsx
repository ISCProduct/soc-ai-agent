'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Button, CircularProgress, Stack, Typography } from '@mui/material'
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
    return <CircularProgress />
  }

  return (
    <Stack spacing={3}>
      <Typography variant="h4">ダッシュボード</Typography>
      <Typography>
        {userName} さん、ようこそ。自社ID: {companyId}
      </Typography>
      <Typography color="text.secondary">
        スカウト送信や学生検索などの機能は今後のIssueで追加されます。
      </Typography>
      <Button
        variant="outlined"
        onClick={() => {
          companyAuthService.logout()
          router.push('/company-portal/sign-in')
        }}
      >
        ログアウト
      </Button>
    </Stack>
  )
}
