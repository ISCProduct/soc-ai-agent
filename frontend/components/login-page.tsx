'use client'

import { useState, useEffect } from 'react'
import {
  Box,
  Card,
  CardContent,
  TextField,
  Button,
  Typography,
  Tabs,
  Tab,
  Divider,
  Alert,
} from '@mui/material'
import GitHubIcon from '@mui/icons-material/GitHub'
import GoogleIcon from '@mui/icons-material/Google'
import Link from 'next/link'
import { authService, AuthResponse } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/backend-url'
import { GUEST_LIMITATIONS } from '@/lib/guest-limits'
import { StudentThemeToggle } from '@/components/student-theme-toggle'
import { extractTenantSlug } from '@/lib/tenant'

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

interface LoginPageProps {
  onAuthSuccess: (authResponse: AuthResponse) => void
  /** 0: ログイン, 1: 新規登録 */
  initialTab?: number
}

export function LoginPage({ onAuthSuccess, initialTab = 0 }: LoginPageProps) {
  const [tabValue, setTabValue] = useState(initialTab)
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [emailTouched, setEmailTouched] = useState(false)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  // 新規登録: メール送信完了状態
  const [registrationEmailSent, setRegistrationEmailSent] = useState(false)

  const emailInvalid = emailTouched && email !== '' && !EMAIL_PATTERN.test(email)

  useEffect(() => {
    setTabValue(initialTab)
  }, [initialTab])

  const handleLogin = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      const response = await authService.login(email, password)
      authService.saveAuth(response)
      onAuthSuccess(response)
    } catch (err: any) {
      const msg = err.message || ''
      if (msg === 'email_not_verified') {
        setError('メールアドレスが未確認です。登録時に送信された確認メールをご確認ください。')
      } else if (msg === 're_verification_required') {
        setError('10日以上ログインがなかったため、再認証メールを送信しました。メールをご確認ください。')
      } else {
        setError(msg)
      }
    } finally {
      setLoading(false)
    }
  }

  const handleRequestRegistration = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await authService.requestRegistration(email)
      setRegistrationEmailSent(true)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleGuestLogin = async () => {
    setError('')
    setLoading(true)
    try {
      const response = await authService.createGuest()
      authService.saveAuth(response)
      onAuthSuccess(response)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const handleOAuthLogin = (provider: 'google' | 'github') => {
    // バックエンドのOAuthエンドポイントへ直接遷移する
    // fetch経由ではなく直接ナビゲートすることでクッキーが同一オリジンで正しく設定される
    const slug = extractTenantSlug(window.location.hostname)
    const tenantParam = slug ? `?tenant=${encodeURIComponent(slug)}` : ''
    window.location.href = `${BACKEND_URL}/api/auth/${provider}${tenantParam}`
  }

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        bgcolor: 'background.default',
        p: { xs: 2, sm: 3 },
      }}
    >
      <Card sx={{ maxWidth: 520, width: '100%' }}>
        <CardContent sx={{ p: { xs: 3, sm: 5 } }}>
          <Typography variant="h4" align="center" gutterBottom fontWeight="bold">
            IT業界キャリアエージェント
          </Typography>
          <Typography variant="body2" align="center" color="text.secondary" sx={{ mb: 3 }}>
            4万社から最適な企業を選定
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              {error}
            </Alert>
          )}

          <Tabs value={tabValue} onChange={(_, v) => setTabValue(v)} sx={{ mb: 3 }}>
            <Tab label="ログイン" />
            <Tab label="新規登録" />
          </Tabs>

          {tabValue === 0 ? (
            <Box component="form" onSubmit={handleLogin}>
              <TextField
                fullWidth
                label="メールアドレス"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={() => setEmailTouched(true)}
                error={emailInvalid}
                helperText={emailInvalid ? 'メールアドレスの形式が正しくありません' : ' '}
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
                sx={{ mb: 1 }}
              />
              <Box sx={{ textAlign: 'right', mt: -1, mb: 2 }}>
                <Link href="/forgot-password" style={{ fontSize: '0.875rem' }}>
                  パスワードをお忘れですか？
                </Link>
              </Box>
              <Button
                type="submit"
                fullWidth
                variant="contained"
                size="large"
                disabled={loading || emailInvalid}
              >
                ログイン
              </Button>
            </Box>
          ) : registrationEmailSent ? (
            <Alert severity="success">
              <strong>確認メールを送信しました</strong><br />
              {email} 宛に確認リンクを送りました。メールを確認して登録を完了してください。
            </Alert>
          ) : (
            <Box component="form" onSubmit={handleRequestRegistration}>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                まずメールアドレスを入力してください。確認リンクをお送りします。
              </Typography>
              <TextField
                fullWidth
                label="メールアドレス"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                onBlur={() => setEmailTouched(true)}
                error={emailInvalid}
                helperText={emailInvalid ? 'メールアドレスの形式が正しくありません' : ' '}
                required
                sx={{ mb: 3 }}
              />
              <Button
                type="submit"
                fullWidth
                variant="contained"
                size="large"
                disabled={loading || emailInvalid}
              >
                確認メールを送る
              </Button>
            </Box>
          )}

          <Divider sx={{ my: 3 }}>または</Divider>

          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
            <Button
              fullWidth
              variant="outlined"
              startIcon={<GoogleIcon />}
              onClick={() => handleOAuthLogin('google')}
              disabled={loading}
            >
              Googleでログイン
            </Button>
            <Button
              fullWidth
              variant="outlined"
              startIcon={<GitHubIcon />}
              onClick={() => handleOAuthLogin('github')}
              disabled={loading}
            >
              GitHubでログイン
            </Button>
            <Button
              fullWidth
              variant="text"
              onClick={handleGuestLogin}
              disabled={loading}
            >
              ゲストとして続ける
            </Button>
            <Typography variant="caption" color="text.secondary" sx={{ display: 'block', px: 0.5 }}>
              ゲストでは以下が利用できません: {GUEST_LIMITATIONS.join(' / ')}。
              あとからアカウント登録すると解放されます。
            </Typography>
          </Box>

          <Divider sx={{ my: 3 }} />
          <StudentThemeToggle />
        </CardContent>
      </Card>
    </Box>
  )
}
