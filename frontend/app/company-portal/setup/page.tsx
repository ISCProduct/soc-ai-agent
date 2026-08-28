'use client'

import { FormEvent, Suspense, useMemo, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  CircularProgress,
  TextField,
  Typography,
} from '@mui/material'
import { companyAuthService } from '@/lib/company-auth'

function SetupContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = useMemo(() => searchParams.get('token') || '', [searchParams])
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!token) {
      setError('招待トークンが見つかりません')
      return
    }
    setError('')
    setLoading(true)
    try {
      await companyAuthService.acceptInvite(token, password, name || undefined)
      router.push('/company-portal')
    } catch {
      setError('招待の受諾に失敗しました。リンクの有効期限を確認してください')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Box
      sx={{
        display: 'flex',
        justifyContent: 'center',
        alignItems: 'center',
        minHeight: '100vh',
        bgcolor: 'background.default',
        p: 2,
      }}
    >
      <Card sx={{ maxWidth: 450, width: '100%' }}>
        <CardContent sx={{ p: { xs: 2, sm: 4 } }}>
          <Typography variant="h5" fontWeight="bold" gutterBottom>
            企業アカウントの有効化
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            パスワードを設定してログインを完了してください。
          </Typography>

          {!token && <Alert severity="error" sx={{ mb: 2 }}>招待リンクが無効です</Alert>}
          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

          <Box component="form" onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="お名前（任意）"
              value={name}
              onChange={(e) => setName(e.target.value)}
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="パスワード（8文字以上）"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              sx={{ mb: 3 }}
            />
            <Button
              type="submit"
              fullWidth
              variant="contained"
              size="large"
              disabled={loading || !token}
            >
              {loading ? <CircularProgress size={24} /> : '有効化してログイン'}
            </Button>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}

export default function CompanyPortalSetupPage() {
  return (
    <Suspense
      fallback={(
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
          <CircularProgress />
        </Box>
      )}
    >
      <SetupContent />
    </Suspense>
  )
}
