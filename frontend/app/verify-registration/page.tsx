'use client'

import { useEffect, useState } from 'react'
import { Suspense } from 'react'
import { useSearchParams, useRouter } from 'next/navigation'
import {
  Box,
  Card,
  CardContent,
  Typography,
  TextField,
  Button,
  Alert,
  CircularProgress,
  MenuItem,
} from '@mui/material'
import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/backend-url'
import { CERTIFICATION_OPTIONS, joinCertifications } from '@/lib/profile'

function VerifyRegistrationContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const token = searchParams.get('token') ?? ''

  const [verifying, setVerifying] = useState(true)
  const [email, setEmail] = useState('')
  const [tokenError, setTokenError] = useState('')

  // フォーム
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [targetLevel, setTargetLevel] = useState('新卒')
  const [certificationsAcquired, setCertificationsAcquired] = useState<string[]>([])
  const [certificationsInProgress, setCertificationsInProgress] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState('')

  useEffect(() => {
    if (!token) {
      setTokenError('トークンが見つかりません。')
      setVerifying(false)
      return
    }
    fetch(`${BACKEND_URL}/api/auth/verify-registration?token=${encodeURIComponent(token)}`)
      .then(async (res) => {
        if (!res.ok) {
          const text = await res.text()
          throw new Error(text || 'トークンが無効または期限切れです。')
        }
        return res.json() as Promise<{ email: string; token: string }>
      })
      .then((data) => {
        setEmail(data.email)
        setVerifying(false)
      })
      .catch((err: Error) => {
        setTokenError(err.message)
        setVerifying(false)
      })
  }, [token])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError('')
    setSubmitting(true)
    try {
      const response = await authService.register(
        email,
        password,
        name,
        targetLevel,
        joinCertifications(certificationsAcquired),
        certificationsInProgress,
        token,
      )
      authService.saveAuth(response)
      router.push('/')
    } catch (err: any) {
      setSubmitError(err.message)
    } finally {
      setSubmitting(false)
    }
  }

  if (verifying) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <CircularProgress />
      </Box>
    )
  }

  if (tokenError) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', p: 2 }}>
        <Card sx={{ maxWidth: 450, width: '100%' }}>
          <CardContent sx={{ p: 4 }}>
            <Alert severity="error">{tokenError}</Alert>
            <Button sx={{ mt: 2 }} onClick={() => router.push('/login')}>
              ログインページへ
            </Button>
          </CardContent>
        </Card>
      </Box>
    )
  }

  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh', p: 2 }}>
      <Card sx={{ maxWidth: 450, width: '100%' }}>
        <CardContent sx={{ p: 4 }}>
          <Typography variant="h5" fontWeight="bold" gutterBottom>
            アカウント情報の入力
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
            {email} でアカウントを作成します。
          </Typography>

          {submitError && <Alert severity="error" sx={{ mb: 2 }}>{submitError}</Alert>}

          <Box component="form" onSubmit={handleSubmit}>
            <TextField
              fullWidth
              label="名前"
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
              sx={{ mb: 2 }}
            />
            <TextField
              fullWidth
              select
              label="区分"
              value={targetLevel}
              onChange={(e) => setTargetLevel(e.target.value)}
              sx={{ mb: 2 }}
            >
              <MenuItem value="新卒">新卒</MenuItem>
              <MenuItem value="中途">中途</MenuItem>
            </TextField>
            <TextField
              fullWidth
              select
              label="取得資格"
              value={certificationsAcquired}
              onChange={(e) =>
                setCertificationsAcquired(
                  typeof e.target.value === 'string' ? e.target.value.split(',') : e.target.value,
                )
              }
              SelectProps={{
                multiple: true,
                renderValue: (selected) => (selected as string[]).join(', '),
              }}
              helperText="複数選択可"
              sx={{ mb: 2 }}
            >
              {CERTIFICATION_OPTIONS.map((option) => (
                <MenuItem key={option} value={option}>
                  {option}
                </MenuItem>
              ))}
            </TextField>
            <TextField
              fullWidth
              label="勉強中の資格"
              value={certificationsInProgress}
              onChange={(e) => setCertificationsInProgress(e.target.value)}
              placeholder="例: 応用情報技術者、AWS SAA（改行区切り可）"
              multiline
              minRows={2}
              sx={{ mb: 3 }}
            />
            <Button
              type="submit"
              fullWidth
              variant="contained"
              size="large"
              disabled={submitting}
            >
              {submitting ? <CircularProgress size={24} /> : '登録を完了する'}
            </Button>
          </Box>
        </CardContent>
      </Card>
    </Box>
  )
}

export default function VerifyRegistrationPage() {
  return (
    <Suspense
      fallback={
        <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
          <CircularProgress />
        </Box>
      }
    >
      <VerifyRegistrationContent />
    </Suspense>
  )
}
