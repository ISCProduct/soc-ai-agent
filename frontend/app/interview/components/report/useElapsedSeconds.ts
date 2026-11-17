import { useEffect, useState } from 'react'

/** 基準時刻（ms）からの経過秒。基準が null または active=false の間は止まる */
export function useElapsedSeconds(startedAtMs: number | null | undefined, active: boolean): number {
  const [now, setNow] = useState(() => Date.now())
  useEffect(() => {
    if (!startedAtMs || !active) return
    setNow(Date.now())
    const timer = setInterval(() => setNow(Date.now()), 1000)
    return () => clearInterval(timer)
  }, [startedAtMs, active])
  return startedAtMs ? Math.max(0, Math.floor((now - startedAtMs) / 1000)) : 0
}
