'use client'

import Link from 'next/link'
import { useEffect, useState } from 'react'
import {
  Button,
  Card,
  CardContent,
  Chip,
  Divider,
  Typography,
} from '@mui/material'
import Grid from '@mui/material/Grid'
import { authService } from '@/lib/auth'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'

const cardSx = {
  height: '100%',
  border: '1px solid',
  borderColor: 'divider',
  borderRadius: '10px',
} as const

export default function AdminDashboardPage() {
  const [companyCount, setCompanyCount] = useState<number | null>(null)

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
        const companiesRes = await fetch('/api/admin/companies', { headers })
        const companiesData = await companiesRes.json()
        setCompanyCount(companiesData?.companies?.length ?? 0)
      } catch {
        setCompanyCount(null)
      }
    }
    loadCounts()
  }, [])

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="管理者ダッシュボード"
        description="管理者向けの操作メニューです。権限がない場合は表示されません。"
        backHref="/"
      />

      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                ベクトルDB
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                Chroma のインデックス状況確認と企業単位の再埋め込みを行います。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/vector-db">
                ベクトルDB管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                AI/RAG 運用管理
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                RAG ベクトルDB、キャッシュヒット率、コスト削減効果を一元管理します。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/vector-db">
                RAG 運用へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
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
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                プロファイル再計算
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                企業マッチングプロファイルを一括または個別に再計算します。ロールバックも可能です。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/profile-recalculation">
                プロファイル再計算へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                集合知サマリー再構築
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                全企業の行動サマリーをバッチ再集計します。データの整合性を保つ際に使用します。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/collective-insights">
                集合知管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
        <Grid size={{ xs: 12, md: 6 }}>
          <Card elevation={0} sx={cardSx}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                スクレイパーセッション
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                クローリングに使用するサイトごとのセッション（Cookie）を管理します。
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Button variant="contained" component={Link} href="/admin/scraper-sessions">
                セッション管理へ
              </Button>
            </CardContent>
          </Card>
        </Grid>
      </Grid>
    </PageContainer>
  )
}
