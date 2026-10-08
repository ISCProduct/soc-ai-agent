'use client'

import { useEffect } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { LoginPage } from '@/components/LoginPage'
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

    // localStorage にユーザーが残っていても、httpOnly Cookie のセッションは
    // 切れていることがある。リフレッシュトークンは30日で失効し、middleware が
    // Cookie を消す(#1519)。localStorage は誰も消さないのでそのまま残る。
    //
    // その状態で無条件に / へ送ると、サーバーコンポーネントの
    // requireSessionUser() が Cookie 不在で /login へ戻し、その /login が
    // また localStorage を見て / へ送る。ローディングとログイン画面を
    // 往復し続け、利用者は localStorage を消す方法を知らないので復帰できない。
    // Cookie が生きているか確かめてから送る。
    const controller = new AbortController()
    fetch('/api/auth/session', {
      method: 'GET',
      cache: 'no-store',
      credentials: 'same-origin',
      signal: controller.signal,
    })
      .then((res) => {
        // 職員（教員・キャリア担当）はチャットではなく教員指導画面(/admin)へ送る。
        if (res.ok) router.replace(authService.getStoredUser()?.is_staff ? '/admin' : '/')
        // 401 のときは送らずにこの画面へ留まる。それだけでループは止まる。
        //
        // ここで authService.logout() を呼んでストレージを掃除したくなるが、
        // やってはいけない。401 は「リフレッシュトークンが失効した」ときだけで
        // なく、「アクセストークンが期限切れ + Backendが一時的に落ちている」
        // ときにも返る(middleware が refresh 失敗を unavailable と判定し、
        // 期限切れトークンを後段へ渡さないため)。logout() は Backend の
        // /api/auth/logout を叩いてリフレッシュトークンを失効させるので、
        // 一時障害に当たっただけの利用者が、まだ30日有効なセッションを
        // 失って再ログインを強制される(#1519 が避けた挙動)。
        // 取り残された localStorage は次のログイン成功時に上書きされる。
      })
      .catch(() => {
        // 通信断・中断。生きているか判断できないので送らない。
      })
    return () => controller.abort()
  }, [router, wantRegister])

  const handleAuthSuccess = (authResponse: AuthResponse) => {
    // 職員はログイン直後から教員指導画面(/admin)へ。学生・企業はこれまで通り/へ。
    router.push(authResponse.is_staff ? '/admin' : '/')
  }

  return <LoginPage onAuthSuccess={handleAuthSuccess} initialTab={wantRegister ? 1 : 0} />
}
