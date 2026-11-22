'use client'

import { useEffect, useRef, useState } from 'react'
import { Alert, Box, Button, CircularProgress, Paper, Stack, Typography } from '@mui/material'
import { interviewApi, type InterviewPlayback, type StudentVideo } from '@/lib/interview'
import { UI } from '@/lib/design/tokens'

type ReviewPlayerProps = {
  sessionId: number
  playback: InterviewPlayback | null
  loading: boolean
  error: string | null
  onRetry: () => void
  seekUtteranceId: number | null
  seekNonce: number
}

function playableVideo(videos: StudentVideo[] | undefined): StudentVideo | null {
  const done = (videos ?? []).filter((video) => video.status === 'done')
  if (done.length === 0) return null
  return done[done.length - 1]
}

export default function ReviewPlayer({
  sessionId,
  playback,
  loading,
  error,
  onRetry,
  seekUtteranceId,
  seekNonce,
}: ReviewPlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const requestGen = useRef(0)
  const signedRef = useRef<{ url: string; expiresAtMs: number } | null>(null)
  const [playError, setPlayError] = useState<string | null>(null)
  const [preparing, setPreparing] = useState(false)

  const videos = playback && Array.isArray(playback.videos) ? playback.videos : undefined
  const cues = playback && Array.isArray(playback.cues) ? playback.cues : []
  const video = playableVideo(videos)
  const playbackRef = useRef(playback)
  playbackRef.current = playback

  useEffect(() => {
    setPlayError(null)
    signedRef.current = null
    if (videoRef.current) {
      videoRef.current.removeAttribute('src')
      videoRef.current.load()
    }
  }, [sessionId, video?.id])

  const playAt = async (offsetSeconds: number | null) => {
    if (!video) return
    const gen = requestGen.current + 1
    requestGen.current = gen
    setPreparing(true)
    setPlayError(null)
    try {
      const player = videoRef.current
      if (!player) return
      // 署名URLは15分で切れる。切れたあとも同じ src のままだと、発話位置への再生が失敗したまま戻らない。
      const signed = signedRef.current
      const stillValid = signed != null && signed.expiresAtMs - Date.now() > 15_000
      if (!player.src || !stillValid) {
        const fresh = await interviewApi.videoPlaybackURL(sessionId, video.id)
        if (requestGen.current !== gen) return
        const expiresAtMs = Date.parse(fresh.expires_at)
        signedRef.current = { url: fresh.url, expiresAtMs: Number.isNaN(expiresAtMs) ? 0 : expiresAtMs }
        player.src = fresh.url
      }
      if (offsetSeconds != null) player.currentTime = offsetSeconds
      await player.play()
    } catch {
      if (requestGen.current !== gen) return
      setPlayError('録画を再生できませんでした')
    } finally {
      if (requestGen.current === gen) setPreparing(false)
    }
  }

  useEffect(() => {
    if (seekNonce === 0 || seekUtteranceId == null) return
    const current = playbackRef.current
    if (!current) return
    const cue = (current.cues ?? []).find((item) => item.utterance_id === seekUtteranceId)
    if (!cue || cue.offset_seconds == null) {
      setPlayError('この発話の録画位置は特定できません')
      return
    }
    void playAt(cue.offset_seconds)
  }, [seekNonce, seekUtteranceId])

  return (
    <Paper elevation={0} sx={{ p: 2.5, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
      <Typography component="h2" sx={{ fontWeight: 700, mb: 0.5 }}>録画で振り返る</Typography>
      <Typography sx={{ fontSize: 13, color: UI.inkSoft, mb: 1.5 }}>
        評価の根拠になった発話を、自分の録画から再生します。時刻は面接開始からの目安で、確定した切り出し位置ではありません。
      </Typography>
      {loading ? (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
          <CircularProgress size={28} sx={{ color: UI.mark }} aria-label="録画を読み込み中" />
        </Box>
      ) : error ? (
        <Stack spacing={1} alignItems="flex-start">
          <Alert severity="error">{error}</Alert>
          <Button size="small" variant="outlined" onClick={onRetry}>再試行</Button>
        </Stack>
      ) : !videos || videos.length === 0 ? (
        <Typography color="text.secondary" fontSize={14}>
          この面接の録画はありません。カメラをオンにして練習すると、ここに残ります。
        </Typography>
      ) : !video ? (
        <Typography color="text.secondary" fontSize={14}>録画を保存しています。完了するとここから再生できます。</Typography>
      ) : (
        <Stack spacing={1.5}>
          <Box
            component="video"
            ref={videoRef}
            controls
            playsInline
            sx={{ width: '100%', borderRadius: 1, bgcolor: '#111', maxHeight: 280 }}
          />
          <Button variant="contained" disabled={preparing} onClick={() => void playAt(null)} sx={{ alignSelf: 'flex-start', minHeight: 44 }}>
            {preparing ? '準備しています' : '最初から再生'}
          </Button>
          {playError ? <Alert severity="error">{playError}</Alert> : null}
          <Typography sx={{ fontSize: 12, color: UI.inkFaint }}>
            {cues.some((cue) => cue.offset_seconds != null)
              ? 'レポートの「録画のこのあたりを再生」を押すと、その発話のあたりから再生します。'
              : '面接の開始時刻が無いため、発話の位置までは飛べません。最初から再生できます。'}
          </Typography>
        </Stack>
      )}
    </Paper>
  )
}
