'use client'

import { FormEvent, useState } from 'react'
import { useRouter } from 'next/navigation'
import Link from 'next/link'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  TextField,
  Typography,
} from '@mui/material'
import { StudentThemeToggle } from '@/components/StudentThemeToggle'
import {
  COMPANY_PASSWORD_MIN_LENGTH,
  companyAuthService,
  validateNewPassword,
} from '@/lib/company/auth'

export default function CompanyPortalRegisterPage() {
  const router = useRouter()
  const [companyName, setCompanyName] = useState('')
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    setError('')

    const passwordError = validateNewPassword(password, confirmPassword)
    if (passwordError) {
      setError(passwordError)
      return
    }
    if (!companyName.trim() || !name.trim() || !email.trim()) {
      setError('企業名・担当者名・メールアドレスを入力してください')
      return
    }

    setLoading(true)
    try {
      await companyAuthService.register({
        companyName: companyName.trim(),
        name: name.trim(),
        email: email.trim(),
        password,
      })
      router.push('/company-portal')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'アカウントの作成に失敗しました')
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
        p: { xs: 2, sm: 3 },
      }}
    >
      <Card sx={{ maxWidth: 520, width: '100%' }}>
        <CardContent sx={{ p: { xs: 3, sm: 5 } }}>
          <Typography variant="h4" component="h1" align="center" gutterBottom fontWeight="bold">
            IT業界キャリアエージェント
          </Typography>
          <Typography variant="body2" align="center" color="text.secondary" sx={{ mb: 3 }}>
            企業担当者アカウントの作成
          </Typography>

          {error && (
            <Alert severity="error" sx={{ mb: 2 }}>
              <Typography variant="subtitle2" fontWeight="bold" gutterBottom>
                アカウントを作成できませんでした
              </Typography>
              <Typography variant="body2">{error}</Typography>
            </Alert>
          )}

          <Box component="form" onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="企業名"
              value={companyName}
              onChange={(e) => setCompanyName(e.target.value)}
              required
              autoComplete="organization"
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="担当者名"
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              autoComplete="name"
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="メールアドレス"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              autoComplete="username"
              slotProps={{ htmlInput: { inputMode: 'email' } }}
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label={`パスワード（${COMPANY_PASSWORD_MIN_LENGTH}文字以上）`}
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              autoComplete="new-password"
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              label="パスワード（確認）"
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              autoComplete="new-password"
              sx={{ mb: 3 }}
            />
            <Button type="submit" fullWidth variant="contained" size="large" disabled={loading}>
              アカウントを作成
            </Button>
          </Box>

          <Box sx={{ textAlign: 'center', mt: 2 }}>
            <Link href="/company-portal/sign-in" className="auth-link">
              すでにアカウントをお持ちの方はログイン
            </Link>
          </Box>

          <Box sx={{ mt: 3, display: 'flex', justifyContent: 'center' }}>
            <StudentThemeToggle />
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}
