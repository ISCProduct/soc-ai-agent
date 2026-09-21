'use client'

import { useCallback, useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Grid,
  Stack,
  Typography,
} from '@mui/material'
import { PageContainer } from '@/components/admin/PageContainer'
import { PageLoading } from '@/components/common/PageLoading'
import { companyAuthService } from '@/lib/company/auth'
import { companyApplicationService, type CompanyDashboard } from '@/lib/company/applications'

// 集計カード。0件でも「何もない」と分かることが重要なので、
// 件数は常に出し、次の行動への導線を添える（#1320）。
function StatCard({
  label,
  value,
  hint,
  actionLabel,
  onClick,
}: {
  label: string
  value: number
  hint: string
  actionLabel: string
  onClick: () => void
}) {
  return (
    <Card
      elevation={0}
      sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', height: '100%' }}
    >
      <CardActionArea onClick={onClick} sx={{ height: '100%' }}>
        <CardContent>
          <Typography variant="body2" color="text.secondary">
            {label}
          </Typography>
          <Typography variant="h3" fontWeight="bold" sx={{ my: 1 }}>
            {value}
          </Typography>
          <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 1 }}>
            {hint}
          </Typography>
          <Typography variant="body2" color="primary">
            {actionLabel}
          </Typography>
        </CardContent>
      </CardActionArea>
    </Card>
  )
}

export default function CompanyPortalDashboardPage() {
  const router = useRouter()
  const [loading, setLoading] = useState(true)
  const [userName, setUserName] = useState('')
  const [dashboard, setDashboard] = useState<CompanyDashboard | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    const stored = companyAuthService.getStoredUser()
    if (!stored) {
      router.replace('/company-portal/sign-in')
      return
    }
    try {
      const me = await companyAuthService.fetchMe()
      setUserName(me.name)
    } catch {
      companyAuthService.logout()
      router.replace('/company-portal/sign-in')
      return
    }

    try {
      setDashboard(await companyApplicationService.fetchDashboard())
      setError('')
    } catch {
      // 集計が取れなくても導線は出す。ここで画面ごと落とすと何もできなくなる。
      setError('集計を取得できませんでした。時間をおいて再度お試しください。')
    } finally {
      setLoading(false)
    }
  }, [router])

  useEffect(() => {
    void load()
  }, [load])

  if (loading) {
    return <PageLoading message="ダッシュボードを読み込んでいます..." />
  }

  const pending = dashboard?.pending_applications ?? 0
  const jobs = dashboard?.published_jobs ?? 0
  const candidates = dashboard?.new_candidates ?? 0
  const windowDays = dashboard?.new_candidate_window_days ?? 7

  return (
    <PageContainer maxWidth={960}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        企業ポータル
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        ようこそ、{userName} さん
      </Typography>

      {error && (
        <Alert severity="warning" sx={{ mb: 3 }}>
          {error}
        </Alert>
      )}

      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label="未対応の応募"
            value={pending}
            hint={pending === 0 ? 'まだ応募はありません' : '選考を進めましょう'}
            actionLabel="応募者を見る"
            onClick={() => router.push('/company-portal/applications')}
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label="公開中の求人"
            value={jobs}
            hint={jobs === 0 ? '求人がないと応募は集まりません' : '学生に公開されています'}
            actionLabel="求人を管理する"
            onClick={() => router.push('/company-portal/jobs')}
          />
        </Grid>
        <Grid size={{ xs: 12, md: 4 }}>
          <StatCard
            label={`新着の候補者（過去${windowDays}日）`}
            value={candidates}
            hint="スカウト公開に同意した学生のみ"
            actionLabel="学生を探す"
            onClick={() => router.push('/company-portal/students')}
          />
        </Grid>
      </Grid>

      {pending === 0 && jobs === 0 && (
        <Card
          elevation={0}
          sx={{ border: '1px solid', borderColor: 'divider', borderRadius: '10px', mb: 3 }}
        >
          <CardContent>
            <Typography variant="h6" gutterBottom>
              まずは求人を用意しましょう
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
              公開中の求人がないため、学生からの応募は発生しません。
              求人を作成するか、先に学生を探してスカウトの準備を進められます。
            </Typography>
            <Stack direction="row" spacing={1}>
              <Button variant="contained" onClick={() => router.push('/company-portal/jobs')}>
                求人を作る
              </Button>
              <Button variant="outlined" onClick={() => router.push('/company-portal/students')}>
                学生を探す
              </Button>
            </Stack>
          </CardContent>
        </Card>
      )}

      <Stack direction="row" spacing={1}>
        <Button variant="outlined" onClick={() => router.push('/company-portal/settings')}>
          設定
        </Button>
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
    </PageContainer>
  )
}
