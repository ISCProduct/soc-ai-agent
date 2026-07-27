'use client'

import {
  Box,
  Paper,
  Typography,
  Button,
  Card,
  CardContent,
  Stack,
  Chip,
  Tabs,
  Tab,
  CircularProgress,
} from '@mui/material'
import { ArrowBack, LocationOn, People, TrendingUp as TrendingUpIcon } from '@mui/icons-material'
import dynamic from 'next/dynamic'
import type { CapitalRelation, CompanyMarketInfo } from '@/lib/company-data'
import type { Company } from '../types'

const CompanyRelationDiagram = dynamic(() => import('./CompanyRelationDiagram'), {
  ssr: false,
  loading: () => (
    <Box sx={{ height: 360, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
      <CircularProgress size={32} />
    </Box>
  ),
})

export interface CompanyDetailViewProps {
  company: Company
  detailTab: number
  onDetailTabChange: (tab: number) => void
  onBack: () => void
  relations: CapitalRelation[]
  marketInfo: CompanyMarketInfo[]
  diagramLoading: boolean
  diagramError: string | null
}

/**
 * 選択企業の詳細（基本情報 / 資本関連図 / ビジネス関連図）。
 */
export default function CompanyDetailView({
  company,
  detailTab,
  onDetailTabChange,
  onBack,
  relations,
  marketInfo,
  diagramLoading,
  diagramError,
}: CompanyDetailViewProps) {
  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      backgroundColor: '#fff',
    }}>
      {/* ヘッダー */}
      <Box sx={{
        p: { xs: 2, sm: 3 },
        borderBottom: '1px solid #e0e0e0',
        backgroundColor: '#fff',
        flexShrink: 0,
      }}>
        <Button variant="outlined" startIcon={<ArrowBack />} onClick={onBack}>
          企業一覧に戻る
        </Button>
      </Box>

      {/* 企業詳細コンテンツ */}
      <Box sx={{
        flexGrow: 1,
        overflowY: 'auto',
        p: { xs: 2, sm: 3 },
        backgroundColor: '#fafafa',
      }}>
        <Box sx={{ maxWidth: 1200, mx: 'auto' }}>
          <Card elevation={3}>
            <CardContent sx={{ p: { xs: 2, sm: 4 } }}>
              {/* 企業名とマッチスコア */}
              <Box sx={{ display: 'flex', flexDirection: { xs: 'column', sm: 'row' }, justifyContent: 'space-between', alignItems: { xs: 'flex-start', sm: 'flex-start' }, mb: 3, gap: 1 }}>
                <Box>
                  <Typography variant="h4" fontWeight="bold" gutterBottom sx={{ fontSize: { xs: '1.4rem', sm: '2.125rem' } }}>
                    {company.name}
                  </Typography>
                  <Typography variant="h6" color="text.secondary" sx={{ fontSize: { xs: '1rem', sm: '1.25rem' } }}>
                    {company.industry}
                  </Typography>
                </Box>
                <Box sx={{ textAlign: { xs: 'left', sm: 'right' }, display: 'flex', alignItems: 'center', gap: 1, flexDirection: { xs: 'row', sm: 'column' } }}>
                  <Typography variant="h2" color="primary.main" fontWeight="bold" sx={{ fontSize: { xs: '2rem', sm: '3.75rem' } }}>
                    {company.matchScore}
                  </Typography>
                  <Typography variant="body1" color="text.secondary">
                    適合度
                  </Typography>
                </Box>
              </Box>

              {/* タブ */}
              <Box sx={{ borderBottom: 1, borderColor: 'divider', mb: 3 }}>
                <Tabs value={detailTab} onChange={(_e, newValue: number) => onDetailTabChange(newValue)}>
                  <Tab label="基本情報" />
                  <Tab label="資本関連図" />
                  <Tab label="ビジネス関連図" />
                </Tabs>
              </Box>

              {/* タブ0: 基本情報 */}
              {detailTab === 0 && (
                <>
                  <Box sx={{ mb: 4 }}>
                    <Typography variant="h6" fontWeight="bold" gutterBottom>
                      📍 基本情報
                    </Typography>
                    <Stack spacing={2}>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <LocationOn color="action" />
                        <Typography variant="body1">
                          <strong>所在地:</strong> {company.location}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <People color="action" />
                        <Typography variant="body1">
                          <strong>従業員数:</strong> {company.employees}
                        </Typography>
                      </Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <TrendingUpIcon color="action" />
                        <Typography variant="body1">
                          <strong>業種:</strong> {company.industry}
                        </Typography>
                      </Box>
                    </Stack>
                  </Box>

                  <Box sx={{ mb: 4 }}>
                    <Typography variant="h6" fontWeight="bold" gutterBottom>
                      💡 マッチング理由
                    </Typography>
                    <Paper sx={{ p: 2, backgroundColor: '#f5f5f5' }}>
                      <Typography variant="body1">
                        {company.description}
                      </Typography>
                    </Paper>
                  </Box>

                  {company.techStack && company.techStack.length > 0 && (
                    <Box sx={{ mb: 4 }}>
                      <Typography variant="h6" fontWeight="bold" gutterBottom>
                        🛠️ 技術スタック
                      </Typography>
                      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                        {company.techStack.map((tech, i) => (
                          <Chip
                            key={i}
                            label={tech}
                            color="primary"
                            variant="filled"
                            sx={{ fontSize: '0.95rem', py: 2.5 }}
                          />
                        ))}
                      </Stack>
                    </Box>
                  )}

                  {company.tags && company.tags.length > 0 && (
                    <Box sx={{ mb: 4 }}>
                      <Typography variant="h6" fontWeight="bold" gutterBottom>
                        🏷️ 企業タグ
                      </Typography>
                      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                        {company.tags.map((tag, i) => (
                          <Chip key={i} label={tag} variant="outlined" />
                        ))}
                      </Stack>
                    </Box>
                  )}
                </>
              )}

              {/* タブ1: 資本関連図 */}
              {detailTab === 1 && (
                <CompanyRelationDiagram
                  companyId={company.id}
                  type="capital"
                  relations={relations}
                  marketInfo={marketInfo}
                  loading={diagramLoading}
                  error={diagramError}
                />
              )}

              {/* タブ2: ビジネス関連図 */}
              {detailTab === 2 && (
                <CompanyRelationDiagram
                  companyId={company.id}
                  type="business"
                  relations={relations}
                  marketInfo={marketInfo}
                  loading={diagramLoading}
                  error={diagramError}
                />
              )}
            </CardContent>
          </Card>
        </Box>
      </Box>
    </Box>
  )
}
