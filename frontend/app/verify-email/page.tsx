'use client'

import { useEffect, useState } from 'react'
import { Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Box, Button, CircularProgress, Paper, Typography } from '@mui/material'
import CheckCircleIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Error'
import { BACKEND_URL } from '@/lib/backend-url'

function VerifyEmailContent() {
  const searchParams = useSearchParams()
  const router = useRouter()
  const [status, setStatus] = useState<'loading' | 'success' | 'error'>('loading')
  const [message, setMessage] = useState('')

  useEffect(() => {
    const token = searchParams.get('token')
    if (!token) {
      setStatus('error')
      setMessage('認証トークンが見つかりません。')
      return
    }

    fetch(`${BACKEND_URL}/api/auth/verify-email?token=${encodeURIComponent(token)}`)
      .then(async (res) => {
        const data = await res.json()
        if (res.ok) {
          setStatus('success')
          setMessage(data.message || 'メールアドレスを確認しました。')
        } else {
          setStatus('error')
          setMessage(data || '認証に失敗しました。リンクが期限切れか無効です。')
        }
      })
      .catch(() => {
        setStatus('error')
        setMessage('通信エラーが発生しました。')
      })
  }, [searchParams])

  return (
    <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#f5f5f5' }}>
      <Paper sx={{ p: 4, maxWidth: 440, width: '100%', textAlign: 'center', borderRadius: 2 }}>
        {status === 'loading' && (
          <>
            <CircularProgress sx={{ mb: 2 }} />
            <Typography>認証中...</Typography>
          </>
        )}
        {status === 'success' && (
          <>
            <CheckCircleIcon sx={{ fontSize: 56, color: '#34a853', mb: 2 }} />
            <Typography variant="h6" fontWeight="bold" sx={{ mb: 1 }}>
              認証完了
            </Typography>
            <Typography color="text.secondary" sx={{ mb: 3 }}>{message}</Typography>
            <Button variant="contained" onClick={() => router.push('/login')} sx={{ bgcolor: '#1976D2' }}>
              ログインする
            </Button>
          </>
        )}
        {status === 'error' && (
          <>
            <ErrorIcon sx={{ fontSize: 56, color: '#ea4335', mb: 2 }} />
            <Typography variant="h6" fontWeight="bold" sx={{ mb: 1 }}>
              認証エラー
            </Typography>
            <Typography color="text.secondary" sx={{ mb: 3 }}>{message}</Typography>
            <Button variant="outlined" onClick={() => router.push('/login')}>
              ログインページへ
            </Button>
          </>
        )}
      </Paper>
    </Box>
  )
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <Box sx={{ minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center', bgcolor: '#f5f5f5' }}>
          <Paper sx={{ p: 4, maxWidth: 440, width: '100%', textAlign: 'center', borderRadius: 2 }}>
            <CircularProgress sx={{ mb: 2 }} />
            <Typography>認証中...</Typography>
          </Paper>
        </Box>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  )
}
