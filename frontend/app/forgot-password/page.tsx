'use client'

import { useState } from 'react'
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

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [loading, setLoading] = useState(false)
  const [sent, setSent] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await authService.requestPasswordReset(email)
      setSent(true)
    } catch (err: any) {
      setError(err.message || 'エラーが発生しました')
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
            パスワードをお忘れですか？
          </Typography>
          <Typography variant="body2" align="center" color="text.secondary" sx={{ mb: 3 }}>
            登録済みのメールアドレスを入力してください。パスワードリセット用のリンクをお送りします。
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          {sent ? (
            <Alert severity="success" sx={{ mb: 3 }}>
              <strong>メールを送信しました</strong>
              <br />
              {email} 宛にパスワードリセット用のリンクを送りました。メールをご確認ください。
            </Alert>
          ) : (
            <Box component="form" onSubmit={handleSubmit}>
              <TextField
                fullWidth
                label="メールアドレス"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                sx={{ mb: 3 }}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                size="large"
                disabled={loading}
                sx={{ mb: 2 }}
              >
                リセットメールを送る
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
