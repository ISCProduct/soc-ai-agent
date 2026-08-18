'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Button,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { AdminFormContainer } from '@/components/admin/AdminFormContainer'
import { ErrorAlert } from '@/components/common/ErrorAlert'

export default function AdminOrganizationNewPage() {
  const router = useRouter()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) window.location.href = '/'
  }, [])

  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [plan, setPlan] = useState('free')
  const [contractStartDate, setContractStartDate] = useState('')
  const [contractEndDate, setContractEndDate] = useState('')
  const [error, setError] = useState('')

  const handleCreate = async () => {
    setError('')
    try {
      const res = await fetch('/api/admin/organizations', {
        method: 'POST',
        headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name,
          slug,
          plan,
          contract_start_date: contractStartDate,
          contract_end_date: contractEndDate,
        }),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || '学園の登録に失敗しました')
        return
      }
      router.push('/admin/organizations')
    } catch {
      setError('学園の登録に失敗しました')
    }
  }

  return (
    <AdminFormContainer
      title="学園(組織)の登録"
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
          onChange={(e) => setSlug(e.target.value)}
          required
          placeholder="my-school"
          helperText={`https://${slug || '<slug>'}.shukatsu-ai.jp でアクセスできるようになります(半角英数字とハイフンのみ)`}
        />
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
          契約期間は未入力でも登録できます(後から編集可能)。
        </Typography>
        <Button variant="contained" onClick={handleCreate} disabled={!name || !slug}>
          登録する
        </Button>
      </Stack>
    </AdminFormContainer>
  )
}
