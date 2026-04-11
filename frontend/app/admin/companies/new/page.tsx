'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import { authService } from '@/lib/auth'

export default function AdminCompanyNewPage() {
  const router = useRouter()

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const [error, setError] = useState('')
  const [name, setName] = useState('')
  const [industry, setIndustry] = useState('')
  const [location, setLocation] = useState('')
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [sourceType, setSourceType] = useState('manual')
  const [sourceUrl, setSourceUrl] = useState('')
  const [dataStatus, setDataStatus] = useState('draft')
  const [isProvisional, setIsProvisional] = useState(true)

  const handleCreate = async () => {
    setError('')
    const admin = authService.getStoredUser()
    const res = await fetch('/api/admin/companies', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAdminFetchHeaders(),
      },
      body: JSON.stringify({
        name,
        industry,
        location,
        website_url: websiteUrl,
        source_type: sourceType,
        source_url: sourceUrl,
        is_provisional: isProvisional,
        data_status: dataStatus,
      }),
    })
    const data = await res.json()
    if (!res.ok) {
      setError(data?.error || '企業の作成に失敗しました')
      return
    }
    router.push('/admin/companies')
  }

  return (
    <Box sx={{ p: 4, maxWidth: 700, mx: 'auto' }}>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 3 }}>
        <Typography variant="h4" fontWeight="bold">
          企業の追加
        </Typography>
        <Button variant="outlined" size="small" onClick={() => router.push('/admin/companies')}>
          一覧に戻る
        </Button>
      </Stack>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Card>
        <CardContent>
          <Stack spacing={2}>
            <TextField label="企業名" value={name} onChange={(e) => setName(e.target.value)} required />
            <TextField label="業種" value={industry} onChange={(e) => setIndustry(e.target.value)} />
            <TextField label="所在地" value={location} onChange={(e) => setLocation(e.target.value)} />
            <TextField label="公式サイトURL" value={websiteUrl} onChange={(e) => setWebsiteUrl(e.target.value)} />
            <TextField select label="出典タイプ" value={sourceType} onChange={(e) => setSourceType(e.target.value)}>
              <MenuItem value="official">公式サイト</MenuItem>
              <MenuItem value="job_site">就活/転職サイト</MenuItem>
              <MenuItem value="manual">手入力</MenuItem>
            </TextField>
            <TextField label="出典URL" value={sourceUrl} onChange={(e) => setSourceUrl(e.target.value)} />
            <TextField select label="ステータス" value={dataStatus} onChange={(e) => setDataStatus(e.target.value)}>
              <MenuItem value="draft">下書き</MenuItem>
              <MenuItem value="published">公開</MenuItem>
            </TextField>
            <TextField
              select
              label="暫定データ"
              value={isProvisional ? 'yes' : 'no'}
              onChange={(e) => setIsProvisional(e.target.value === 'yes')}
            >
              <MenuItem value="yes">暫定</MenuItem>
              <MenuItem value="no">確定</MenuItem>
            </TextField>
            <Button variant="contained" onClick={handleCreate} disabled={!name.trim()}>
              追加する
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
