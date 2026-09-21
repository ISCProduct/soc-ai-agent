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

  const schoolOpsCards: HubCard[] = [
    {
      title: '企業情報',
      description: '学生に見せる企業の登録・確認・公開を行います。',
      href: '/admin/companies',
      cta: '企業情報の管理へ',
      chip: companyCount === null ? '読み込み中' : `登録数 ${companyCount}`,
    },
    {
      title: '求人管理',
      description: '企業に紐づく求人ポジションの登録・確認を行います。',
      href: '/admin/job-positions',
      cta: '求人管理へ',
    },
    {
      title: '応募・選考管理',
      description: '学生の応募一覧を確認し、選考ステータスを更新します。',
      href: '/admin/applications',
      cta: '応募管理へ',
    },
    {
      title: '卒業生の就職情報',
      description: '卒業生の就職先情報の登録・確認を行います。',
      href: '/admin/graduate-employments',
      cta: '就職情報管理へ',
    },
    {
      title: 'ユーザー管理',
      description: '担当校のユーザー情報を確認します。',
      href: '/admin/users',
      cta: 'ユーザー管理へ',
    },
  ]

  const teacherCards: HubCard[] = [
    {
      title: '面接管理',
      description: '担当校の面接セッションと録画動画を確認できます。',
      href: '/admin/interviews',
      cta: '面接管理へ',
    },
    {
      title: 'スコアダッシュボード',
      description: 'ユーザー別の練習回数・平均スコア・スコア推移を一覧比較します。',
      href: '/admin/dashboard',
      cta: 'ダッシュボードへ',
    },
    {
      title: '生徒の傾向分析',
      description: '担当する生徒のタイプと向いている業界を一覧比較します（参考情報）。',
      href: '/admin/student-insights',
      cta: '傾向分析へ',
    },
  ]

  const systemCards: HubCard[] = [
    {
      title: '学園(組織)管理',
      description: '学園サブドメインの登録・契約プラン・契約期間を管理します。',
      href: '/admin/organizations',
      cta: '学園管理へ',
    },
    {
      title: '監査ログ',
      description: '管理者操作の履歴を確認できます。',
      href: '/admin/audit-logs',
      cta: '監査ログへ',
    },
    {
      title: 'APIコストモニタリング',
      description: 'OpenAI APIの日次・月次コストとモデル別内訳を可視化します。',
      href: '/admin/costs',
      cta: 'コスト管理へ',
    },
    {
      title: 'ベクトルDB / RAG運用',
      description: 'Chroma のインデックス状況確認と企業単位の再埋め込みを行います。',
      href: '/admin/vector-db',
      cta: 'ベクトルDB管理へ',
    },
    {
      title: 'スコア精度検証',
      description: '相関分析・フェーズ別メトリクス・A/Bテスト・キャリブレーションを管理します。',
      href: '/admin/score-validation',
      cta: 'スコア精度検証へ',
    },
    {
      title: 'プロファイル再計算',
      description: '企業マッチングプロファイルを一括または個別に再計算します。',
      href: '/admin/profile-recalculation',
      cta: 'プロファイル再計算へ',
    },
    {
      title: '集合知サマリー再構築',
      description: '全企業の行動サマリーをバッチ再集計します。',
      href: '/admin/collective-insights',
      cta: '集合知管理へ',
    },
    {
      title: 'スクレイパーセッション',
      description: 'クローリングに使用するサイトごとのセッション（Cookie）を管理します。',
      href: '/admin/scraper-sessions',
      cta: 'セッション管理へ',
    },
  ]

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

      {isPlatform && <HubSection heading="システム管理" cards={systemCards} />}
      <HubSection heading="学校運営" cards={schoolOpsCards} />
      <HubSection heading="教員業務" cards={teacherCards} />
    </PageContainer>
  )
}
