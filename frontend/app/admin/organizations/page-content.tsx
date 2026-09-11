'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Box,
  Button,
  Chip,
  Stack,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { AdminListCard } from '@/components/admin/AdminListCard'
import { ErrorAlert } from '@/components/common/ErrorAlert'

type Organization = {
  id: number
  name: string
  slug: string
  status: 'active' | 'disabled'
  plan: 'free' | 'standard' | 'pro'
  contract_start_date?: string
  contract_end_date?: string
}

const PLAN_LABEL: Record<Organization['plan'], string> = {
  free: 'Free',
  standard: 'Standard',
  pro: 'Pro',
}

function formatDate(value?: string): string {
  if (!value) return '未設定'
  return value.slice(0, 10)
}

export default function PageContent() {
  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [organizations, setOrganizations] = useState<Organization[]>([])
  const [error, setError] = useState('')

  useEffect(() => {
    fetch('/api/admin/organizations?limit=100', {
      headers: authService.getAdminFetchHeaders(),
    })
      .then(async (r) => {
        const data = await r.json()
        if (!r.ok) {
          setError(data?.error || '学園一覧の取得に失敗しました')
          return
        }
        setOrganizations(data?.organizations || [])
      })
      .catch(() => setError('学園一覧の取得に失敗しました'))
  }, [])

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="学園(組織)管理"
        description="学園サブドメイン(<slug>.shukatsu-ai.jp)ごとの契約プラン・契約期間を管理します。"
        backHref="/admin"
        actions={
          <Button
            variant="contained"
            size="small"
            component={Link}
            href="/admin/organizations/new"
            disableElevation
          >
            + 新規登録
          </Button>
        }
      />

      <AdminPanel title="学園一覧">
        <AdminPanelBody>
          <ErrorAlert error={error} />
          <Stack spacing={1}>
            {organizations.length === 0 ? (
              <Typography variant="body2" color="text.secondary">
                まだ学園が登録されていません。
              </Typography>
            ) : (
              organizations.map((org) => (
                <AdminListCard key={org.id}>
                  <Stack direction="row" alignItems="center" justifyContent="space-between">
                    <Box>
                      <Stack direction="row" spacing={1} alignItems="center">
                        <Typography variant="subtitle2" fontWeight="bold">
                          {org.name}
                        </Typography>
                        <Chip
                          label={org.status === 'active' ? '有効' : '無効'}
                          size="small"
                          color={org.status === 'active' ? 'success' : 'default'}
                        />
                        <Chip label={PLAN_LABEL[org.plan] ?? org.plan} size="small" variant="outlined" />
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        {org.slug}.shukatsu-ai.jp
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        契約期間: {formatDate(org.contract_start_date)} 〜 {formatDate(org.contract_end_date)}
                      </Typography>
                    </Box>
                    <Button
                      size="small"
                      variant="outlined"
                      component={Link}
                      href={`/admin/organizations/${org.id}/edit`}
                    >
                      編集
                    </Button>
                  </Stack>
                </AdminListCard>
              ))
            )}
          </Stack>
        </AdminPanelBody>
      </AdminPanel>
    </PageContainer>
  )
}
