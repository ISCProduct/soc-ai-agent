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
} from '@mui/material'
import Link from 'next/link'
import { authService } from '@/lib/auth'

function ResetPasswordForm() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const token = searchParams.get('token') || ''

  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [loading, setLoading] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!token) {
      setError('無効なリセットリンクです。')
    }
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

    setLoading(true)
    try {
      await authService.resetPassword(token, password)
      setSuccess(true)
      setTimeout(() => {
        router.push('/login')
      }, 3000)
    } catch (err: any) {
      setError(err.message || 'パスワードのリセットに失敗しました。')
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

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          {success ? (
            <>
              <Alert severity="success" sx={{ mb: 3 }}>
                パスワードをリセットしました。ログインしてください。
              </Alert>
              <Typography variant="body2" align="center" color="text.secondary">
                3秒後にログインページへ移動します...
              </Typography>
            </>
          ) : (
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
                パスワードをリセットする
              </Button>
            </Box>
          )}

          <Box sx={{ textAlign: 'center', mt: 2 }}>
            <Link href="/login" style={{ fontSize: '0.875rem', color: '#1976D2' }}>
              ログインページへ戻る
            </Link>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}

export default function ResetPasswordPage() {
  return (
    <Suspense fallback={<Box sx={{ display: 'flex', justifyContent: 'center', mt: 10 }}><Typography>読み込み中...</Typography></Box>}>
      <ResetPasswordForm />
    </Suspense>
  )
}
