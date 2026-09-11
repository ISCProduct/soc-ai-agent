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
import Link from 'next/link'
import { companyAuthService, validateNewPassword } from '@/lib/company-auth'

function ResetPasswordContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = useMemo(() => searchParams.get('token') || '', [searchParams])

  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const validationError = validateNewPassword(password, confirmPassword)
    if (validationError) {
      setError(validationError)
      return
    }

    setError('')
    setLoading(true)
    try {
      await companyAuthService.resetPassword(token, password)
      // 成功時はそのままログイン状態になるためダッシュボードへ遷移する (#1196)
      router.push('/company-portal')
    } catch {
      setError('パスワードの再設定に失敗しました。リンクの有効期限を確認してください')
    } finally {
      setLoading(false)
    }
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        bgcolor: 'background.default',
        p: 2,
      }}
    >
      <Card sx={{ maxWidth: 450, width: '100%' }}>
        <CardContent sx={{ p: { xs: 2, sm: 4 } }}>
          <Typography variant="h5" align="center" gutterBottom fontWeight="bold">
            新しいパスワードを設定
          </Typography>
          <Typography variant="body2" align="center" color="text.secondary" sx={{ mb: 3 }}>
            新しいパスワードを設定すると、そのままログインします。
          </Typography>

          {!token && <Alert severity="error" sx={{ mb: 2 }}>無効なリセットリンクです。</Alert>}
          {error && <Alert severity="error" sx={{ mb: 2 }}>{error}</Alert>}

          <Box component="form" onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="新しいパスワード"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              helperText="8文字以上で入力してください"
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="パスワード（確認）"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              sx={{ mb: 3 }}
            />
            <Button
              type="submit"
              fullWidth
              variant="contained"
              size="large"
              disabled={loading || !token}
              sx={{ mb: 2 }}
            >
              {loading ? <CircularProgress size={24} /> : 'パスワードを再設定する'}
            </Button>
          </Box>

          <Box sx={{ textAlign: 'center', mt: 2 }}>
            <Link href="/company-portal/sign-in" style={{ fontSize: '0.875rem' }}>
              ログインページへ戻る
            </Link>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}

export default function CompanyPortalResetPasswordPage() {
  return (
    <Suspense
      fallback={(
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
          <CircularProgress />
        </Box>
      )}
    >
      <ResetPasswordContent />
    </Suspense>
  )
}
