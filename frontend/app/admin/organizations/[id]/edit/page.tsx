'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import {
  Box,
  Button,
  CircularProgress,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

export default function AdminOrganizationEditPage() {
  const router = useRouter()
  const { id } = useParams<{ id: string }>()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [status, setStatus] = useState('active')
  const [plan, setPlan] = useState('free')
  const [contractStartDate, setContractStartDate] = useState('')
  const [contractEndDate, setContractEndDate] = useState('')

  useEffect(() => {
    if (!id) return
    fetch(`/api/admin/organizations/${id}`, { headers: authService.getAdminFetchHeaders() })
      .then((r) => r.json())
      .then((org) => {
        if (org?.id) {
          setName(org.name || '')
          setSlug(org.slug || '')
          setStatus(org.status || 'active')
          setPlan(org.plan || 'free')
          setContractStartDate(org.contract_start_date ? org.contract_start_date.slice(0, 10) : '')
          setContractEndDate(org.contract_end_date ? org.contract_end_date.slice(0, 10) : '')
        }
        setLoading(false)
      })
      .catch(() => setLoading(false))
  }, [id])

  const handleUpdate = async () => {
    setError('')
    const res = await fetch(`/api/admin/organizations/${id}`, {
      method: 'PUT',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        status,
        plan,
        contract_start_date: contractStartDate,
        contract_end_date: contractEndDate,
      }),
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '更新に失敗しました')
      return
    }
    router.push('/admin/organizations')
  }

  if (loading) {
    return (
      <PageContainer maxWidth={ADMIN_PAGE_WIDTH.form}>
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
          <CircularProgress />
        </Box>
      </PageContainer>
    )
  }

  return (
    <AdminFormContainer
      title="学園(組織)の編集"
      maxWidth={700}
      backHref="/admin/organizations"
      backLabel="一覧に戻る"
    >
      <ErrorAlert error={error} />
      <Stack spacing={2}>
        <TextField label="学園名" value={name} onChange={(e) => setName(e.target.value)} required />
        <TextField
          label="サブドメイン (slug)"
          value={slug}
          disabled
          helperText="サブドメインは登録後に変更できません"
        />
        <TextField select label="状態" value={status} onChange={(e) => setStatus(e.target.value)}>
          <MenuItem value="active">有効</MenuItem>
          <MenuItem value="disabled">無効</MenuItem>
        </TextField>
        <TextField select label="契約プラン" value={plan} onChange={(e) => setPlan(e.target.value)}>
          <MenuItem value="free">Free</MenuItem>
          <MenuItem value="standard">Standard</MenuItem>
          <MenuItem value="pro">Pro</MenuItem>
        </TextField>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
          <TextField
            label="契約開始日"
            type="date"
            value={contractStartDate}
            onChange={(e) => setContractStartDate(e.target.value)}
            slotProps={{ inputLabel: { shrink: true } }}
            fullWidth
          />
          <TextField
            label="契約終了日"
            type="date"
            value={contractEndDate}
            onChange={(e) => setContractEndDate(e.target.value)}
            slotProps={{ inputLabel: { shrink: true } }}
            fullWidth
          />
        </Stack>
        <Typography variant="caption" color="text.secondary">
          日付欄を空にすると契約期間の設定を解除します。
        </Typography>
        <Button variant="contained" onClick={handleUpdate} disabled={!name}>
          更新する
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
