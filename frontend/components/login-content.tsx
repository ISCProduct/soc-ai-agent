'use client'

import { useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { LoginPage } from '@/components/login-page'
import { authService, AuthResponse } from '@/lib/auth'
import { isLoginRegisterTab } from '@/lib/guest-limits'

export function LoginContent() {
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
