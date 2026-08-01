'use client'

import { Suspense, useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { LoginPage } from '@/components/login-page'
import { authService, AuthResponse } from '@/lib/auth'
import { isLoginRegisterTab } from '@/lib/guest-limits'
import { PageLoading } from '@/components/common/PageLoading'

function LoginContent() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const wantRegister = isLoginRegisterTab(searchParams.get('tab'))

  useEffect(() => {
    const storedUser = authService.getStoredUser()
    if (!storedUser) return

    // ゲストが登録CTAから来た場合はセッションをクリアして登録画面を表示する
    if (wantRegister && storedUser.is_guest) {
      authService.logout()
      return
    }

    router.replace('/')
  }, [router, wantRegister])

  const handleAuthSuccess = (_authResponse: AuthResponse) => {
    router.push('/')
  }

  return <LoginPage onAuthSuccess={handleAuthSuccess} initialTab={wantRegister ? 1 : 0} />
}

export default function Login() {
  return (
    <Suspense fallback={<PageLoading message="ログイン画面を準備しています..." />}>
      <LoginContent />
    </Suspense>
  )
}
