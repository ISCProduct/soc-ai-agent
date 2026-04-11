'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Collapse,
  Divider,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import { authService } from '@/lib/auth'

type JobPosition = {
  id: number
  company_id: number
  title: string
  description?: string
  employment_type?: string
  work_location?: string
  remote_option?: boolean
  min_salary?: number
  max_salary?: number
  required_skills?: string
  preferred_skills?: string
  data_status?: string
  created_at?: string
  company?: {
    id: number
    name: string
    source_url?: string
    source_type?: string
    source_fetched_at?: string
    is_provisional?: boolean
  }
  job_category?: { id: number; name: string }
}

const statusBadge = (status?: string) => {
  if (status === 'published') return <Chip label="公開" color="success" size="small" />
  if (status === 'rejected') return <Chip label="却下" color="error" size="small" />
  return <Chip label="審査中" color="warning" size="small" />
}

const formatDate = (dateStr?: string) => {
  if (!dateStr) return null
  return new Date(dateStr).toLocaleDateString('ja-JP', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  })
}

const parsedSkills = (json?: string): string[] => {
  if (!json) return []
  try {
    const parsed = JSON.parse(json)
    if (Array.isArray(parsed)) return parsed.map(String)
  } catch {}
  return json.split(/[,、]/).map((s) => s.trim()).filter(Boolean)
}

function JobPositionCard({
  position,
  onPublish,
  onReject,
}: {
  position: JobPosition
  onPublish: (id: number) => void
  onReject: (id: number) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const reqSkills = parsedSkills(position.required_skills)
  const prefSkills = parsedSkills(position.preferred_skills)
  const hasCrawlInfo = !!(position.company?.source_url || position.company?.source_fetched_at || position.description || position.min_salary || reqSkills.length || prefSkills.length)

  return (
    <Box sx={{ border: '1px solid #e0e0e0', borderRadius: 1, p: 2 }}>
      <Stack direction="row" alignItems="flex-start" justifyContent="space-between" spacing={1}>
        <Box flex={1} minWidth={0}>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" sx={{ mb: 0.5 }}>
            <Typography variant="subtitle1" fontWeight="bold">
              {position.title}
            </Typography>
            {statusBadge(position.data_status)}
            {position.company?.is_provisional && (
              <Chip label="仮登録" size="small" variant="outlined" color="warning" />
            )}
          </Stack>
          <Typography variant="body2" color="text.secondary">
            {position.company?.name || `企業ID ${position.company_id}`}
            {position.job_category?.name ? ` / ${position.job_category.name}` : ''}
            {position.employment_type ? ` / ${position.employment_type}` : ''}
            {position.work_location ? ` / ${position.work_location}` : ''}
            {position.remote_option ? ' / リモート可' : ''}
            {position.min_salary || position.max_salary ? (
              ` / ${position.min_salary ? position.min_salary + '万' : ''}〜${position.max_salary ? position.max_salary + '万円' : ''}`
            ) : ''}
          </Typography>
          {position.created_at && (
            <Typography variant="caption" color="text.secondary">
              取得日: {formatDate(position.created_at)}
            </Typography>
          )}
        </Box>
        <Stack direction="row" alignItems="center" spacing={1} flexShrink={0}>
          {(position.data_status || 'draft') !== 'published' && (
            <>
              <Button
                variant="contained"
                color="success"
                size="small"
                onClick={() => onPublish(position.id)}
              >
                承認
              </Button>
              {(position.data_status || 'draft') !== 'rejected' && (
                <Button
                  variant="outlined"
                  color="error"
                  size="small"
                  onClick={() => onReject(position.id)}
                >
                  却下
                </Button>
              )}
            </>
          )}
          {hasCrawlInfo && (
            <Tooltip title={expanded ? '詳細を閉じる' : 'クロール情報を表示'}>
              <IconButton size="small" onClick={() => setExpanded(!expanded)}>
                {expanded ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
              </IconButton>
            </Tooltip>
          )}
        </Stack>
      </Stack>

      <Collapse in={expanded}>
        <Divider sx={{ my: 1.5 }} />
        <Stack spacing={1.5}>
          {/* クロール元情報 */}
          {(position.company?.source_url || position.company?.source_fetched_at || position.company?.source_type) && (
            <Box>
              <Typography variant="caption" fontWeight="bold" color="text.secondary" display="block" sx={{ mb: 0.5 }}>
                クロール元情報
              </Typography>
              <Stack spacing={0.5}>
                {position.company?.source_url && (
                  <Stack direction="row" alignItems="center" spacing={0.5}>
                    <Typography variant="body2" color="text.secondary">URL:</Typography>
                    <Link
                      href={position.company.source_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ display: 'flex', alignItems: 'center', gap: 2, fontSize: 13 }}
                    >
                      {position.company.source_url.length > 60
                        ? position.company.source_url.slice(0, 60) + '…'
                        : position.company.source_url}
                      <OpenInNewIcon sx={{ fontSize: 13, ml: 0.25 }} />
                    </Link>
                  </Stack>
                )}
                {position.company?.source_type && (
                  <Typography variant="body2" color="text.secondary">
                    ソースタイプ: {position.company.source_type}
                  </Typography>
                )}
                {position.company?.source_fetched_at && (
                  <Typography variant="body2" color="text.secondary">
                    取得日時: {formatDate(position.company.source_fetched_at)}
                  </Typography>
                )}
              </Stack>
            </Box>
          )}

          {/* 職務内容 */}
          {position.description && (
            <Box>
              <Typography variant="caption" fontWeight="bold" color="text.secondary" display="block" sx={{ mb: 0.5 }}>
                職務内容
              </Typography>
              <Typography variant="body2" sx={{ whiteSpace: 'pre-line' }}>
                {position.description.length > 300
                  ? position.description.slice(0, 300) + '…'
                  : position.description}
              </Typography>
            </Box>
          )}

          {/* 必須スキル */}
          {reqSkills.length > 0 && (
            <Box>
              <Typography variant="caption" fontWeight="bold" color="text.secondary" display="block" sx={{ mb: 0.5 }}>
                必須スキル
              </Typography>
              <Stack direction="row" flexWrap="wrap" gap={0.5}>
                {reqSkills.map((s, i) => (
                  <Chip key={i} label={s} size="small" color="primary" variant="outlined" />
                ))}
              </Stack>
            </Box>
          )}

          {/* 歓迎スキル */}
          {prefSkills.length > 0 && (
            <Box>
              <Typography variant="caption" fontWeight="bold" color="text.secondary" display="block" sx={{ mb: 0.5 }}>
                歓迎スキル
              </Typography>
              <Stack direction="row" flexWrap="wrap" gap={0.5}>
                {prefSkills.map((s, i) => (
                  <Chip key={i} label={s} size="small" variant="outlined" />
                ))}
              </Stack>
            </Box>
          )}
        </Stack>
      </Collapse>
    </Box>
  )
}

export default function AdminJobPositionsPage() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [jobPositions, setJobPositions] = useState<JobPosition[]>([])
  const [error, setError] = useState('')
  const [filterStatus, setFilterStatus] = useState<'all' | 'draft' | 'published' | 'rejected'>('all')

  const fetchJobPositions = async () => {
    const res = await fetch('/api/admin/job-positions?limit=100', {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await res.json()
    if (res.ok) setJobPositions(data?.positions || [])
  }

  useEffect(() => {
    fetchJobPositions()
  }, [])

  const handlePublish = async (id: number) => {
    const res = await fetch(`/api/admin/job-positions/${id}/publish`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json()
      setError(data?.error || '承認に失敗しました')
      return
    }
    fetchJobPositions()
  }

  const handleReject = async (id: number) => {
    const res = await fetch(`/api/admin/job-positions/${id}/reject`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json()
      setError(data?.error || '却下に失敗しました')
      return
    }
    fetchJobPositions()
  }

  const filtered = jobPositions.filter((p) => {
    if (filterStatus === 'all') return true
    return (p.data_status || 'draft') === filterStatus
  })

  return (
    <Box sx={{ p: 4, maxWidth: 1000, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Typography variant="h4" fontWeight="bold">
          求人管理
        </Typography>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" size="small" component={Link} href="/admin/companies">
            企業管理
          </Button>
          <Button variant="outlined" size="small" component={Link} href="/admin/graduate-employments">
            就職情報管理
          </Button>
          <Button variant="outlined" size="small" component={Link} href="/admin/crawling">
            クローリング管理
          </Button>
        </Stack>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        クローリングで取得した求人情報を審査・公開します。各求人の▼ボタンでクロール元URL・スキル・職務内容を確認できます。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Card>
        <CardContent>
          <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
            <Typography variant="h6">
              求人一覧
              <Typography component="span" variant="body2" color="text.secondary" sx={{ ml: 1 }}>
                ({filtered.length}件)
              </Typography>
            </Typography>
            <Stack direction="row" spacing={1}>
              {(['all', 'draft', 'published', 'rejected'] as const).map((s) => (
                <Chip
                  key={s}
                  label={s === 'all' ? 'すべて' : s === 'draft' ? '審査中' : s === 'published' ? '公開' : '却下'}
                  variant={filterStatus === s ? 'filled' : 'outlined'}
                  color={s === 'published' ? 'success' : s === 'rejected' ? 'error' : s === 'draft' ? 'warning' : 'default'}
                  onClick={() => setFilterStatus(s)}
                  clickable
                />
              ))}
            </Stack>
          </Stack>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={1}>
            {filtered.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                該当する求人がありません。
              </Typography>
            ) : (
              filtered.map((position) => (
                <JobPositionCard
                  key={position.id}
                  position={position}
                  onPublish={handlePublish}
                  onReject={handleReject}
                />
              ))
            )}
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
