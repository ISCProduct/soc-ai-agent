'use client'

import { useEffect, useState, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Box, CircularProgress, Typography, Alert } from '@mui/material'
import { authService } from '@/lib/auth'
import { BACKEND_URL } from '@/lib/backend-url'

function OAuthCallbackContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [error, setError] = useState('')

  useEffect(() => {
    const handleCallback = async () => {
      const errorParam = searchParams.get('error')
      if (errorParam) {
        setError(decodeURIComponent(errorParam))
        return
      }

      const userParam = searchParams.get('user')
      const provider = searchParams.get('provider')
      
      if (!userParam) {
        setError('ユーザー情報が見つかりません')
        return
      }

      try {
        // Base64デコードしてユーザー情報を取得
        // Properly handle URL-safe Base64 and UTF-8
        const normalized = userParam.replace(/-/g, '+').replace(/_/g, '/')
        const binary = atob(normalized)
        const bytes = Uint8Array.from(binary, c => c.charCodeAt(0))
        const userDataString = new TextDecoder('utf-8').decode(bytes)
        const userDataRaw = JSON.parse(userDataString)
        // Fallback repair for mojibake in name
        const fixMojibake = (s: string) => /[Ãå][^\s]/.test(s) ? decodeURIComponent(escape(s)) : s
        let userData = { ...userDataRaw, name: fixMojibake(userDataRaw.name) }
        try {
          const fresh = await authService.getUser(
            typeof userData.user_id === 'string' ? Number(userData.user_id) : userData.user_id,
          )
          userData = { ...userData, ...fresh }
        } catch {
          // ignore and fall back to callback payload
        }

        // ローカルストレージに保存
        authService.saveAuth(userData)
        localStorage.removeItem('oauth_state')

        // 名前が未設定の場合のみオンボーディングへ（初回登録ユーザー）
        if (!userData.name) {
          router.push('/onboarding')
          return
        }

        router.push('/')
      } catch (err: any) {
        setError('認証データの処理に失敗しました: ' + err.message)
      }
    }

    handleCallback()
  }, [searchParams, router])

  if (error) {
    return (
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          p: 3,
        }}
      >
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
        <Typography
          variant="body2"
          color="primary"
          sx={{ cursor: 'pointer' }}
          onClick={() => router.push('/')}
        >
          ホームに戻る
        </Typography>
      </Box>
    )
  }

  return (
    <Box
      sx={{
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        minHeight: '100vh',
        gap: 2,
      }}
    >
      <CircularProgress />
      <Typography variant="body1">認証中...</Typography>
    </Box>
  )
}

export default function OAuthCallback() {
  return (
    <Suspense fallback={
      <Box
        sx={{
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          minHeight: '100vh',
          gap: 2,
        }}
      >
        <CircularProgress />
        <Typography variant="body1">読み込み中...</Typography>
      </Box>
    }>
      <OAuthCallbackContent />
    </Suspense>
  )
}
