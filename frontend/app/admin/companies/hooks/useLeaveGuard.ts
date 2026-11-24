'use client'

import { useEffect } from 'react'

/** 取得・公開の処理中は、ページの再読み込みとリンクでの移動を止める（途中結果の表示が消えるため）。 */
export function useLeaveGuard(busy: boolean) {
  useEffect(() => {
    if (!busy) return
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault()
      e.returnValue = ''
    }
    const onClick = (e: MouseEvent) => {
      const el = e.target
      if (!(el instanceof Element)) return
      if (!el.closest('a[href]')) return
      e.preventDefault()
      e.stopPropagation()
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    document.addEventListener('click', onClick, true)
    return () => {
      window.removeEventListener('beforeunload', onBeforeUnload)
      document.removeEventListener('click', onClick, true)
    }
  }, [busy])
}
