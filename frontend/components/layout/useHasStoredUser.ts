'use client'

import { useEffect, useState } from 'react'
import { usePathname } from 'next/navigation'
import { authService } from '@/lib/auth'

/**
 * ストレージにログイン中の利用者があるか。学生用のナビ（下部ナビ・メニューボタン）を未ログインの訪問者に出さないために使う（#1653・#2222）。
 *
 * セッション Cookie は httpOnly でクライアントから読めないので、ストレージを手掛かりにする。
 * サーバー側（RootLayout）で Cookie を見ると全ルートが動的レンダリングになるため使わない（StudentBottomNav の説明を参照）。
 * 描画の最初（mounted が false の間）は判定しない（サーバーの描画と食い違わないように、どちらも出さない）。
 *
 * この部品は RootLayout にあり、/login でも同じものがマウントされたままになる。ログインは保存の後に
 * router.push で移動する（SPA の遷移）ので、パスが変わるたびに判定し直す（一度だけ読むと、ログインしても
 * 再読み込みまで「未ログイン」のままになる。#2222 のレビュー）。ログアウトも、その後に画面を移るので同じく追える。
 * 利用者の情報は sessionStorage（タブごと）に置くので、別のタブのログイン・ログアウトはこのタブに関係しない。
 */
export function useHasStoredUser(): { mounted: boolean; hasStoredUser: boolean } {
  const pathname = usePathname()
  const [mounted, setMounted] = useState(false)
  const [hasStoredUser, setHasStoredUser] = useState(false)

  useEffect(() => {
    setHasStoredUser(authService.getStoredUser() !== null)
    setMounted(true)
  }, [pathname])

  return { mounted, hasStoredUser }
}
