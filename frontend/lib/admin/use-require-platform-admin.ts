'use client'

import { useEffect, useState } from 'react'
import { useRouter } from 'next/navigation'
import { getAdminSchoolAccess } from '@/lib/admin/school-access'

/**
 * システム管理者専用ページ用。担当校つき管理者は /admin へ戻す。
 * API 側の EchoRequirePlatformAdmin が正。ここは UX の防御。
 */
export function useRequirePlatformAdmin() {
  const router = useRouter()
  const [ready, setReady] = useState(false)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const access = await getAdminSchoolAccess()
        if (cancelled) return
        if (access.restricted) {
          router.replace('/admin')
          return
        }
        setReady(true)
      } catch {
        if (!cancelled) router.replace('/admin')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [router])

  return ready
}
