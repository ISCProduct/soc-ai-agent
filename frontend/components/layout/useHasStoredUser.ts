'use client'

import { useEffect, useState } from 'react'
import { authService } from '@/lib/auth'

/**
 * ストレージにログイン中の利用者があるか。学生用のナビ（下部ナビ・メニューボタン）を未ログインの訪問者に出さないために使う（#1653・#2222）。
 *
 * セッション Cookie は httpOnly でクライアントから読めないので、ストレージを手掛かりにする。
 * サーバー側（RootLayout）で Cookie を見ると全ルートが動的レンダリングになるため使わない（StudentBottomNav の説明を参照）。
 * 描画の最初（mounted が false の間）は判定しない（サーバーの描画と食い違わないように、どちらも出さない）。
 */
export function useHasStoredUser(): { mounted: boolean; hasStoredUser: boolean } {
  const [mounted, setMounted] = useState(false)
  const [hasStoredUser, setHasStoredUser] = useState(false)
  useEffect(() => {
    setHasStoredUser(authService.getStoredUser() !== null)
    setMounted(true)
  }, [])
  return { mounted, hasStoredUser }
}
