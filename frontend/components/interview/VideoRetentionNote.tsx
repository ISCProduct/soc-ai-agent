'use client'

import { useEffect, useState } from 'react'
import { Typography } from '@mui/material'
import { fetchVideoRetention, videoRetentionSentence, type VideoRetention } from '@/lib/interview/video-retention'

export function useVideoRetentionSentence(): string {
  const [status, setStatus] = useState<'loading' | 'error' | 'ready'>('loading')
  const [retention, setRetention] = useState<VideoRetention | null>(null)

  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const data = await fetchVideoRetention()
        if (cancelled) return
        setRetention(data)
        setStatus('ready')
      } catch {
        if (cancelled) return
        setStatus('error')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return videoRetentionSentence(retention, status)
}

/** 面接結果に、学校が選んだ録画の保持期間を出す（SOCAIAGENT-671）。 */
export function VideoRetentionNote() {
  const sentence = useVideoRetentionSentence()
  return (
    <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
      {sentence}
    </Typography>
  )
}
