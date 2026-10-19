'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Box, Button, Paper, Stack, Typography } from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { PipelineBand, pipelineTotal } from '@/components/company-portal/PipelineBand'
import { companyAuthService } from '@/lib/company/auth'
import { companyApplicationService, type CompanyDashboard } from '@/lib/company/applications'
import { nextCompanyAction } from '@/lib/company/next-action'

export default function CompanyPortalDashboardPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [dashboard, setDashboard] = useState<CompanyDashboard | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    try {
      await companyAuthService.fetchMe()
    } catch {
      companyAuthService.logout()
      router.replace('/company-portal/sign-in')
      return
    }

    try {
      setDashboard(await companyApplicationService.fetchDashboard())
      setError('')
    } catch {
      setDashboard(null)
      setError('状況を取得できませんでした。時間をおいて再度お試しください。')
    } finally {
      setLoading(false)
    }
  }, [router])

  useEffect(() => {
    void load()
  }, [load])

  if (loading) {
    return <PageLoading message="ホームを読み込んでいます..." />
  }

  const pending = dashboard?.pending_applications ?? 0
  const jobs = dashboard?.published_jobs ?? 0
  const candidates = dashboard?.new_candidates ?? 0
  const windowDays = dashboard?.new_candidate_window_days ?? 7
  const counts = dashboard?.status_counts ?? {}
  const action = dashboard
    ? nextCompanyAction({ publishedJobs: jobs, pendingApplications: pending })
    : null

  return (
    <PageContainer maxWidth={960}>
      {error && (
        <Alert
          severity="warning"
          sx={{ mb: 2 }}
          action={
            <Button color="inherit" onClick={() => void load()}>
              再読み込み
            </Button>
          }
        >
          {error}
        </Alert>
      )}

      {action && (
        <Paper
          variant="outlined"
          sx={{ p: { xs: 2, sm: 3 }, mb: 3, borderColor: 'divider' }}
        >
          <Typography component="h1" sx={{ fontSize: '1.25rem', fontWeight: 700, mb: 1 }}>
            {action.title}
          </Typography>
          <Typography variant="body1" sx={{ mb: 2, maxWidth: '62ch' }}>
            {action.body}
          </Typography>
          <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
            <Button variant="contained" onClick={() => router.push(action.primary.href)}>
              {action.primary.label}
            </Button>
            {action.secondary && (
              <Button variant="outlined" onClick={() => router.push(action.secondary!.href)}>
                {action.secondary.label}
              </Button>
            )}
          </Stack>
        </Paper>
      )}

      {dashboard && (
        <Box component="section" aria-label="いまの状況" sx={{ mb: 3 }}>
          <Typography component="h2" variant="h6" sx={{ mb: 1 }}>
            いまの状況
          </Typography>
          <Typography component="p" sx={{ m: 0 }}>
            公開中の求人 <strong>{jobs}</strong>件
            <Box component="span" sx={{ mx: 1 }} aria-hidden="true">
              /
            </Box>
            未対応の応募 <strong>{pending}</strong>件
            <Box component="span" sx={{ mx: 1 }} aria-hidden="true">
              /
            </Box>
            新着候補者（過去{windowDays}日） <strong>{candidates}</strong>人
          </Typography>
        </Box>
      )}

      {pipelineTotal(counts) > 0 && (
        <Box component="section" aria-label="選考の進み具合">
          <Typography component="h2" variant="h6" sx={{ mb: 1 }}>
            選考の進み具合
          </Typography>
          <PipelineBand
            counts={counts}
            selected=""
            onSelect={(status) => {
              const query = status ? `?status=${status}` : ''
              router.push(`/company-portal/applications${query}`)
            }}
          />
        </Box>
      )}
    </PageContainer>
  )
}
