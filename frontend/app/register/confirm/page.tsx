'use client'

import { useState, useEffect, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import {
  Box,
  Card,
  CardContent,
  TextField,
  Button,
  Typography,
  Alert,
  CircularProgress,
} from '@mui/material'
import { authService } from '@/lib/auth'

function RegisterConfirmForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = searchParams.get('token') || ''

  const [email, setEmail] = useState('')
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const [tokenError, setTokenError] = useState('')

  useEffect(() => {
    if (!token) {
      setTokenError('無効な登録リンクです。')
      setLoading(false)
      return
    }
    authService.verifyRegistration(token)
      .then(({ email }) => {
        setEmail(email)
      })
      .catch((err: any) => {
        setTokenError(err.message || '無効または期限切れの登録リンクです。')
      })
      .finally(() => setLoading(false))
  }, [token])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')

    if (password.length < 8) {
      setError('パスワードは8文字以上で入力してください。')
      return
    }
    if (password !== confirmPassword) {
      setError('パスワードが一致しません。')
      return
    }

    setSubmitting(true)
    try {
      const response = await authService.register(email, password, name, '新卒', '', '', token)
      authService.saveAuth(response)
      router.push('/')
    } catch (err: any) {
      setError(err.message || '登録に失敗しました。')
    } finally {
      setSubmitting(false)
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
            会員登録の完了
          </Typography>

          {loading ? (
            <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
              <CircularProgress />
            </Box>
          ) : tokenError ? (
            <Alert severity="error">{tokenError}</Alert>
          ) : (
            <>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                {email} での登録を完了します。お名前とパスワードを設定してください。
              </Typography>

              {error && (
                <Alert severity="error" sx={{ mb: 2 }}>
                  {error}
                </Alert>
              )}

              <Box component="form" onSubmit={handleSubmit}>
                <TextField
                  fullWidth
                  label="お名前"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                  sx={{ mb: 2 }}
                />
                <TextField
                  fullWidth
                  label="パスワード"
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
                  disabled={submitting}
                >
                  登録を完了する
                </Button>
              </Box>
            </>
          )}
        </CardContent>
      </Card>
    </Box>
  )
}

export default function RegisterConfirmPage() {
  return (
    <Suspense fallback={<Box sx={{ display: 'flex', justifyContent: 'center', mt: 10 }}><CircularProgress /></Box>}>
      <RegisterConfirmForm />
    </Suspense>
  )
}
