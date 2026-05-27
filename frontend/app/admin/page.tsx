'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Stack,
  Typography,
} from '@mui/material'
import Grid from '@mui/material/GridLegacy'
import { authService } from '@/lib/auth'

export default function AdminDashboardPage() {
  const [companyCount, setCompanyCount] = useState<number | null>(null)
  const [crawlCount, setCrawlCount] = useState<number | null>(null)

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  useEffect(() => {
    const loadCounts = async () => {
      const headers = authService.getAdminFetchHeaders()
      try {
        const [companiesRes, crawlRes] = await Promise.all([
          fetch('/api/admin/companies', { headers }),
          fetch('/api/admin/crawl-sources', { headers }),
        ])
        const companiesData = await companiesRes.json()
        const crawlData = await crawlRes.json()
        setCompanyCount(companiesData?.companies?.length ?? 0)
        setCrawlCount(crawlData?.sources?.length ?? 0)
      } catch {
        setCompanyCount(null)
        setCrawlCount(null)
      }
    }
    loadCounts()
  }, [])

  return (
    <Box sx={{ p: 4, maxWidth: 960, mx: 'auto' }}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        管理者ダッシュボード
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        管理者向けの操作メニューです。権限がない場合は表示されません。
      </Typography>

      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                企業データ
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                企業プロフィールの追加・更新、公開ステータスの管理を行います。
              </Typography>
              <Chip
                label={companyCount === null ? '読み込み中' : `登録数 ${companyCount}`}
                size="small"
                sx={{ mb: 2 }}
              />
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/companies">
                企業管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                自動クローリング
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                週次・月次のスケジュールを設定し、企業データを自動更新します。
              </Typography>
              <Chip
                label={crawlCount === null ? '読み込み中' : `スケジュール ${crawlCount}`}
                size="small"
                sx={{ mb: 2 }}
              />
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/crawling">
                クローリング管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                求人管理
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                企業に紐づく求人ポジションの登録・確認を行います。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/job-positions">
                求人管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                卒業生の就職情報
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                卒業生の就職先情報の登録・確認を行います。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/graduate-employments">
                就職情報管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                ユーザー管理
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                ユーザー情報の確認と管理者権限の付与を行います。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/users">
                ユーザー管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                監査ログ
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                管理者操作の履歴を確認できます。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/audit-logs">
                監査ログへ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                面接管理
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                全ユーザーの面接セッションと録画動画を確認できます。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/interviews">
                面接管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                APIコストモニタリング
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                OpenAI APIの日次・月次コストとモデル別内訳を可視化します。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/costs">
                コスト管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                スコアダッシュボード
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                ユーザー別の練習回数・平均スコア・スコア推移を一覧比較します。CSV エクスポート対応。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/dashboard">
                ダッシュボードへ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={6}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                スコア精度検証
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                相関分析・フェーズ別メトリクス・A/Bテスト・キャリブレーションを管理します。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/score-validation">
                スコア精度検証へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <Stack spacing={2}>
        <Card>
          <CardContent>
            <Typography variant="h6" gutterBottom>
              近日追加予定
            </Typography>
            <Typography variant="body2" color="text.secondary">
              ユーザー権限管理、監査ログ、通知設定などを順次追加します。
            </Typography>
          </CardContent>
        </Card>
      </Stack>
    </Box>
  )
}
