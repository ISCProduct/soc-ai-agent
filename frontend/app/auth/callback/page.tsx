'use client'

import { useEffect, useState, Suspense } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { Box, CircularProgress, Typography, Alert } from '@mui/material'
import { authService } from '@/lib/auth'

type OAuthUser = {
  user_id: number | string
  email: string
  name: string
  token?: string
  user_token?: string
  is_guest: boolean
  target_level?: string
  is_admin?: boolean
  oauth_provider?: string
  avatar_url?: string
}

function validateOAuthPayload(raw: unknown): OAuthUser {
  if (!raw || typeof raw !== 'object') throw new Error('Invalid OAuth payload structure')
  const d = raw as Record<string, unknown>
  if ((typeof d.user_id !== 'number' && typeof d.user_id !== 'string') || !d.user_id)
    throw new Error('Invalid OAuth payload structure')
  if (typeof d.email !== 'string' || !d.email.includes('@'))
    throw new Error('Invalid OAuth payload structure')
  if (typeof d.name !== 'string')
    throw new Error('Invalid OAuth payload structure')
  return {
    user_id: d.user_id as number | string,
    email: d.email,
    name: d.name,
    token: typeof d.token === 'string' ? d.token : undefined,
    user_token: typeof d.user_token === 'string' ? d.user_token : undefined,
    is_guest: typeof d.is_guest === 'boolean' ? d.is_guest : false,
    target_level: typeof d.target_level === 'string' ? d.target_level : '',
    is_admin: typeof d.is_admin === 'boolean' ? d.is_admin : undefined,
    oauth_provider: typeof d.oauth_provider === 'string' ? d.oauth_provider : undefined,
    avatar_url: typeof d.avatar_url === 'string' ? d.avatar_url : undefined,
  }
}

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
        // スキーマ検証: 不正な構造のペイロードを拒否
        const validatedData = validateOAuthPayload(userDataRaw)
        // Fallback repair for mojibake in name
        const fixMojibake = (s: string) => /[Ãå][^\s]/.test(s) ? decodeURIComponent(escape(s)) : s
        let userData = { ...validatedData, name: fixMojibake(validatedData.name) }
        try {
          const fresh = await authService.getUser()
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
