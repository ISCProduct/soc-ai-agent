'use client'

import { useEffect, useState } from 'react'

/**
 * つながっているマイク・スピーカー・カメラの一覧（#2634）。
 * 機器名は許可が取れた後でないと空なので、ストリームを取れてから（ready）読む。
 * 抜き差し（devicechange）でも読み直す。
 */
export function useMediaDevices(ready: boolean) {
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([])

  useEffect(() => {
    const md = typeof navigator === 'undefined' ? undefined : navigator.mediaDevices
    if (!ready || !md?.enumerateDevices) return
    let alive = true
    const load = () => {
      md.enumerateDevices()
        .then((list) => { if (alive) setDevices(list) })
        .catch(() => {})
    }
    load()
    md.addEventListener?.('devicechange', load)
    return () => {
      alive = false
      md.removeEventListener?.('devicechange', load)
    }
  }, [ready])

  return {
    mics: devices.filter((d) => d.kind === 'audioinput'),
    speakers: devices.filter((d) => d.kind === 'audiooutput'),
    cameras: devices.filter((d) => d.kind === 'videoinput'),
  }
}
