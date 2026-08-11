'use client'

import { useParams, useRouter } from 'next/navigation'
import { useEffect, useState, useCallback } from 'react'
import dynamic from 'next/dynamic'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Stack,
  Tab,
  Tabs,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import BusinessIcon from '@mui/icons-material/Business'
import PlaceIcon from '@mui/icons-material/Place'
import PeopleIcon from '@mui/icons-material/People'
import CalendarMonthIcon from '@mui/icons-material/CalendarMonth'
import LanguageIcon from '@mui/icons-material/Language'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import StarIcon from '@mui/icons-material/Star'
import CodeIcon from '@mui/icons-material/Code'
import WorkIcon from '@mui/icons-material/Work'
import FavoriteIcon from '@mui/icons-material/Favorite'
import EmojiEventsIcon from '@mui/icons-material/EmojiEvents'
import TrendingUpIcon from '@mui/icons-material/TrendingUp'
import AccountTreeIcon from '@mui/icons-material/AccountTree'
import HubIcon from '@mui/icons-material/Hub'
import {
  mapCompanyApiToViewModel,
  parseJsonArray,
  type CompanyDetailViewModel,
} from './companyDetailUtils'

const CompanyDiagram = dynamic(() => import('@/components/company-diagram'), {
  ssr: false,
  loading: () => (
    <Box sx={{ py: 4, textAlign: 'center', color: 'text.secondary' }}>
      相関図を読み込み中...
    </Box>
  ),
})

type JobPosition = {
  id: number
  title: string
  employment_type?: string
  work_location?: string
  description?: string
  job_category?: { name: string }
}

type LoadStatus = 'loading' | 'ok' | 'not_found' | 'error'

export default function CompanyDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [company, setCompany] = useState<CompanyDetailViewModel | null>(null)
  const [loadStatus, setLoadStatus] = useState<LoadStatus>('loading')
  const [activeTab, setActiveTab] = useState<'capital' | 'business'>('capital')
  const [jobPositions, setJobPositions] = useState<JobPosition[]>([])

  const fetchCompanyDetail = useCallback(async () => {
    const companyId = String(params.id)
    setLoadStatus('loading')
    try {
      const response = await fetch(`/api/companies/${companyId}`)
      if (response.status === 404) {
        setCompany(null)
        setLoadStatus('not_found')
        return
      }
      if (!response.ok) {
        setCompany(null)
        setLoadStatus('error')
        return
      }
      const raw: unknown = await response.json()
      setCompany(mapCompanyApiToViewModel(raw))
      setLoadStatus('ok')
    } catch (error) {
      console.error('Failed to fetch company details:', error)
      setCompany(null)
      setLoadStatus('error')
    }
  }, [params.id])

  useEffect(() => {
    const companyId = String(params.id)

    const fetchJobPositions = async () => {
      try {
        const res = await fetch(`/api/companies/${companyId}/job-positions`)
        if (!res.ok) return
        const raw: unknown = await res.json()
        const payload =
          raw && typeof raw === 'object' && !Array.isArray(raw)
            ? (raw as { data?: unknown; positions?: JobPosition[] })
            : null
        const positions = Array.isArray(payload?.positions)
          ? payload.positions
          : payload?.data && typeof payload.data === 'object' && !Array.isArray(payload.data)
            ? (payload.data as { positions?: JobPosition[] }).positions
            : undefined
        if (Array.isArray(positions)) setJobPositions(positions)
      } catch {
        // job positions are optional
      }
    }

    void fetchCompanyDetail()
    void fetchJobPositions()
  }, [params.id, fetchCompanyDetail])

  if (loadStatus === 'loading') {
    return (
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#f5f7fa' }}>
        <Typography color="text.secondary">読み込み中...</Typography>
      </Box>
    )
  }

  if (loadStatus === 'error') {
    return (
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Stack spacing={2} alignItems="center">
          <Typography variant="h5" fontWeight={700}>企業情報の取得に失敗しました</Typography>
          <Typography color="text.secondary">ネットワークまたはサーバーエラーが発生しました</Typography>
          <Stack direction="row" spacing={1}>
            <Button variant="contained" onClick={() => void fetchCompanyDetail()}>
              再試行
            </Button>
            <Button startIcon={<ArrowBackIcon />} variant="outlined" onClick={() => router.back()}>
              戻る
            </Button>
          </Stack>
        </Stack>
      </Box>
    )
  }

  if (loadStatus === 'not_found' || !company) {
    return (
      <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
        <Stack spacing={2} alignItems="center">
          <Typography variant="h5" fontWeight={700}>企業が見つかりません</Typography>
          <Button startIcon={<ArrowBackIcon />} variant="contained" onClick={() => router.back()}>
            戻る
          </Button>
        </Stack>
      </Box>
    )
  }

  const infra = parseJsonArray(company.infra_stack)
  const cicd = parseJsonArray(company.cicd_tools)

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: '#f5f7fa' }}>
      <Box sx={{ borderBottom: '1px solid', borderColor: 'divider', bgcolor: 'background.paper' }}>
        <Box sx={{ maxWidth: 1000, mx: 'auto', px: 2, py: 1.5 }}>
          <Button startIcon={<ArrowBackIcon />} onClick={() => router.back()}>
            一覧に戻る
          </Button>
        </Box>
      </Box>

      <Box sx={{ maxWidth: 1000, mx: 'auto', px: 2, py: 4 }}>
        <Card sx={{ mb: 3, borderRadius: 2 }}>
          <CardContent sx={{ p: { xs: 2.5, md: 3 } }}>
            <Stack
              direction={{ xs: 'column', sm: 'row' }}
              justifyContent="space-between"
              alignItems={{ xs: 'flex-start', sm: 'flex-start' }}
              spacing={2}
              sx={{ mb: 2 }}
            >
              <Box sx={{ flex: 1, minWidth: 0 }}>
                <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 0.5 }}>
                  <BusinessIcon color="primary" sx={{ fontSize: 36 }} />
                  <Typography variant="h4" fontWeight={700} sx={{ wordBreak: 'break-word' }}>
                    {company.name}
                  </Typography>
                </Stack>
                <Typography color="text.secondary">{company.industry || '業種未登録'}</Typography>
              </Box>
              <Box sx={{ textAlign: { xs: 'left', sm: 'right' } }}>
                <Stack direction="row" spacing={0.75} alignItems="center" justifyContent={{ sm: 'flex-end' }}>
                  <StarIcon sx={{ color: '#f59e0b' }} />
                  <Typography variant="h5" fontWeight={700} color="primary.main">
                    {company.matchScore}%
                  </Typography>
                </Stack>
                <Typography variant="body2" color="text.secondary">マッチ度</Typography>
              </Box>
            </Stack>

            <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} sx={{ mb: 2.5 }}>
              <Stack direction="row" spacing={1} alignItems="center">
                <PlaceIcon fontSize="small" color="action" />
                <Typography variant="body2">{company.location || '—'}</Typography>
              </Stack>
              <Stack direction="row" spacing={1} alignItems="center">
                <PeopleIcon fontSize="small" color="action" />
                <Typography variant="body2">{company.employees || '—'}</Typography>
              </Stack>
              <Stack direction="row" spacing={1} alignItems="center">
                <CalendarMonthIcon fontSize="small" color="action" />
                <Typography variant="body2">設立: {company.founded || '—'}</Typography>
              </Stack>
            </Stack>

            {company.tags.length > 0 && (
              <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mb: 2 }}>
                {company.tags.map((tag) => (
                  <Chip key={tag} label={tag} size="small" />
                ))}
              </Stack>
            )}

            <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5}>
              {company.website ? (
                <Button
                  variant="outlined"
                  href={company.website}
                  target="_blank"
                  rel="noopener noreferrer"
                  startIcon={<LanguageIcon />}
                  endIcon={<OpenInNewIcon sx={{ fontSize: 14 }} />}
                >
                  公式サイトを見る
                </Button>
              ) : null}
              <Button
                variant="contained"
                startIcon={<WorkIcon />}
                onClick={() => router.push(`/interview?company_id=${company.id}`)}
              >
                この企業で面接練習
              </Button>
            </Stack>
          </CardContent>
        </Card>

        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
            gap: 2.5,
          }}
        >
          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <BusinessIcon fontSize="small" />
                <Typography fontWeight={700}>企業概要</Typography>
              </Stack>
              <Typography variant="body2" sx={{ lineHeight: 1.8, whiteSpace: 'pre-wrap' }}>
                {company.description || '概要未登録'}
              </Typography>
            </CardContent>
          </Card>

          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <CodeIcon fontSize="small" />
                <Typography fontWeight={700}>技術スタック</Typography>
              </Stack>
              <Stack spacing={1.5}>
                {company.techStack.length > 0 && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">言語・フレームワーク</Typography>
                    <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mt: 0.5 }}>
                      {company.techStack.map((tech) => (
                        <Chip key={tech} label={tech} size="small" variant="outlined" />
                      ))}
                    </Stack>
                  </Box>
                )}
                {infra.length > 0 && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">インフラ</Typography>
                    <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mt: 0.5 }}>
                      {infra.map((item) => (
                        <Chip key={item} label={item} size="small" />
                      ))}
                    </Stack>
                  </Box>
                )}
                {cicd.length > 0 && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">CI/CD</Typography>
                    <Stack direction="row" flexWrap="wrap" gap={1} sx={{ mt: 0.5 }}>
                      {cicd.map((item) => (
                        <Chip key={item} label={item} size="small" />
                      ))}
                    </Stack>
                  </Box>
                )}
                {company.development_style && (
                  <Box>
                    <Typography variant="caption" color="text.secondary">開発手法</Typography>
                    <Box sx={{ mt: 0.5 }}>
                      <Chip label={company.development_style} size="small" color="primary" />
                    </Box>
                  </Box>
                )}
                {company.techStack.length === 0 && infra.length === 0 && cicd.length === 0 && !company.development_style && (
                  <Typography variant="body2" color="text.secondary">技術情報未登録</Typography>
                )}
              </Stack>
            </CardContent>
          </Card>

          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <HubIcon fontSize="small" />
                <Typography fontWeight={700}>プロジェクトタイプ</Typography>
              </Stack>
              {company.projectTypes.length === 0 ? (
                <Typography variant="body2" color="text.secondary">未登録</Typography>
              ) : (
                <Stack spacing={1}>
                  {company.projectTypes.map((type) => (
                    <Typography key={type} variant="body2">• {type}</Typography>
                  ))}
                </Stack>
              )}
            </CardContent>
          </Card>

          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <TrendingUpIcon fontSize="small" />
                <Typography fontWeight={700}>給与・待遇</Typography>
              </Stack>
              <Typography fontWeight={600} sx={{ mb: 1 }}>
                {company.salary || '給与情報未登録'}
              </Typography>
              <Stack spacing={0.75}>
                {company.benefits.map((benefit) => (
                  <Stack key={benefit} direction="row" spacing={1} alignItems="center">
                    <FavoriteIcon sx={{ fontSize: 14, color: 'primary.main' }} />
                    <Typography variant="body2">{benefit}</Typography>
                  </Stack>
                ))}
              </Stack>
            </CardContent>
          </Card>

          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <EmojiEventsIcon fontSize="small" />
                <Typography fontWeight={700}>企業文化・働き方</Typography>
              </Stack>
              {company.culture.length === 0 ? (
                <Typography variant="body2" color="text.secondary">未登録</Typography>
              ) : (
                <Stack spacing={1}>
                  {company.culture.map((item) => (
                    <Typography key={item} variant="body2">• {item}</Typography>
                  ))}
                </Stack>
              )}
            </CardContent>
          </Card>

          <Card sx={{ borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1.5 }}>
                <TrendingUpIcon fontSize="small" />
                <Typography fontWeight={700}>企業規模・関連情報</Typography>
              </Stack>
              <Typography variant="body2" fontWeight={600} sx={{ mb: 0.5 }}>企業規模</Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                {company.size || '—'}
              </Typography>
              {company.parentCompany && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="body2" fontWeight={600}>親会社</Typography>
                  <Typography variant="body2" color="text.secondary">{company.parentCompany}</Typography>
                </Box>
              )}
              {company.subsidiaries && company.subsidiaries.length > 0 && (
                <Box sx={{ mb: 1.5 }}>
                  <Typography variant="body2" fontWeight={600}>子会社</Typography>
                  {company.subsidiaries.map((sub) => (
                    <Typography key={sub} variant="body2" color="text.secondary">• {sub}</Typography>
                  ))}
                </Box>
              )}
              {company.partnerships && company.partnerships.length > 0 && (
                <Box>
                  <Typography variant="body2" fontWeight={600}>主要パートナー</Typography>
                  {company.partnerships.map((partner) => (
                    <Typography key={partner} variant="body2" color="text.secondary">• {partner}</Typography>
                  ))}
                </Box>
              )}
            </CardContent>
          </Card>
        </Box>

        {jobPositions.length > 0 && (
          <Card sx={{ mt: 3, borderRadius: 2 }}>
            <CardContent>
              <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
                <WorkIcon fontSize="small" />
                <Typography fontWeight={700}>募集職種</Typography>
              </Stack>
              <Stack spacing={1.5}>
                {jobPositions.map((pos) => (
                  <Box key={pos.id} sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 2 }}>
                    <Typography fontWeight={600}>{pos.title}</Typography>
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                      {[pos.job_category?.name, pos.employment_type, pos.work_location].filter(Boolean).join(' / ')}
                    </Typography>
                    {pos.description && (
                      <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                        {pos.description}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Stack>
            </CardContent>
          </Card>
        )}

        <Card sx={{ mt: 3, borderRadius: 2 }}>
          <CardContent>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 2 }}>
              <AccountTreeIcon fontSize="small" />
              <Typography fontWeight={700}>企業関連図</Typography>
            </Stack>

            <Tabs
              value={activeTab}
              onChange={(_, v: 'capital' | 'business') => setActiveTab(v)}
              sx={{ borderBottom: 1, borderColor: 'divider', mb: 2 }}
            >
              <Tab value="capital" icon={<AccountTreeIcon />} iconPosition="start" label="資本関連図" />
              <Tab value="business" icon={<HubIcon />} iconPosition="start" label="ビジネス関連図" />
            </Tabs>

            {activeTab === 'capital' ? (
              <Box>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
                  • 黄色でハイライトされた企業が現在表示中の企業です
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                  • 実線：子会社（出資比率50%以上）、破線：関連会社（出資比率50%未満）
                </Typography>
                <CompanyDiagram companyId={company.id} diagramType="capital" />
              </Box>
            ) : (
              <Box>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 0.5 }}>
                  • 黄色でハイライトされた企業が現在表示中の企業です
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                  • 青い矢印：ビジネス取引関係、灰色の点線：資本関係（親会社）
                </Typography>
                <CompanyDiagram companyId={company.id} diagramType="business" />
              </Box>
            )}
          </CardContent>
        </Card>

        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2} justifyContent="center" sx={{ mt: 4 }}>
          <Button size="large" variant="contained" startIcon={<ArrowBackIcon />} onClick={() => router.back()}>
            一覧に戻る
          </Button>
          {company.website ? (
            <Button
              size="large"
              variant="outlined"
              href={company.website}
              target="_blank"
              rel="noopener noreferrer"
              startIcon={<LanguageIcon />}
              endIcon={<OpenInNewIcon sx={{ fontSize: 14 }} />}
            >
              公式サイトへ
            </Button>
          ) : null}
        </Stack>
      </Box>
    </Box>
  )
}
