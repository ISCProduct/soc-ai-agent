'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Box,
  Button,
  Card,
  CardContent,
  Divider,
  IconButton,
  Stack,
  Typography,
} from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'
import { PageContainer } from '@/components/admin/PageContainer'
import { AdminListCard } from '@/components/admin/AdminListCard'

type GraduateEmployment = {
  id: number
  company_id: number
  job_position_id?: number
  graduate_name?: string
  graduation_year?: number
  school_name?: string
  department?: string
  hired_at?: string
  note?: string
  company?: { id: number; name: string }
  job_position?: { id: number; title: string }
}

export default function AdminGraduateEmploymentsPage() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [graduateEntries, setGraduateEntries] = useState<GraduateEmployment[]>([])

  useEffect(() => {
    fetch('/api/admin/graduate-employments?limit=100', {
      headers: authService.getAdminFetchHeaders(),
    })
      .then((r) => r.json())
      .then((data) => setGraduateEntries(data?.entries || []))
      .catch(() => {})
  }, [])

  return (
    <PageContainer maxWidth={1000}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 1 }}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <IconButton component={Link} href="/admin"><ArrowBackIcon /></IconButton>
          <Typography variant="h4" fontWeight="bold">
            卒業生の就職情報管理
          </Typography>
        </Stack>
        <Stack direction="row" spacing={1}>
          <Button variant="outlined" size="small" component={Link} href="/admin/companies">
            企業管理
          </Button>
          <Button variant="outlined" size="small" component={Link} href="/admin/job-positions">
            求人管理
          </Button>
          <Button variant="contained" size="small" component={Link} href="/admin/graduate-employments/new">
            + 新規登録
          </Button>
        </Stack>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        卒業生の就職先情報を確認・編集します。
      </Typography>

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            就職情報一覧
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={1}>
            {graduateEntries.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                まだ就職情報がありません。
              </Typography>
            ) : (
              graduateEntries.map((entry) => (
                <AdminListCard key={entry.id}>
                  <Stack direction="row" alignItems="center" justifyContent="space-between">
                    <Box>
                      <Typography variant="subtitle2" fontWeight="bold">
                        {entry.company?.name || `企業ID ${entry.company_id}`}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {entry.graduate_name || '氏名未設定'} / {entry.school_name || '学校未設定'}
                        {entry.graduation_year ? ` (${entry.graduation_year}卒)` : ''}
                      </Typography>
                      {entry.job_position?.title && (
                        <Typography variant="caption" color="text.secondary">
                          職種: {entry.job_position.title}
                        </Typography>
                      )}
                    </Box>
                    <Button
                      size="small"
                      variant="outlined"
                      component={Link}
                      href={`/admin/graduate-employments/${entry.id}/edit`}
                    >
                      編集
                    </Button>
                  </Stack>
                </AdminListCard>
              ))
            )}
          </Stack>
        </CardContent>
      </Card>
    </PageContainer>
  )
}
