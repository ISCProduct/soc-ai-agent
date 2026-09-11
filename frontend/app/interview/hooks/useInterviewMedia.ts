'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import type { InterviewStatus } from '../types'
import { shouldStartInterviewMediaPreview } from './mediaPreviewGate'

/** getUserMedia 共通制約（ロビープレビュー / ensureStream で共有） */
export const MEDIA_CONSTRAINTS: MediaStreamConstraints = {
  audio: { sampleRate: 48000, channelCount: 1, echoCancellation: true, noiseSuppression: true },
  video: true,
}

type UseInterviewMediaArgs = {
  loading: boolean
  status: InterviewStatus
}

/**
 * カメラ/マイクストリームと video 要素 refs を一箇所で所有する。
 * sessionVideoCallbackRef は status 更新後に video がマウントされるケース用。
 */
export function useInterviewMedia({ loading, status }: UseInterviewMediaArgs) {
  const [micEnabled, setMicEnabled] = useState(true)
  const [cameraEnabled, setCameraEnabled] = useState(true)
  const [lobbyPermissionError, setLobbyPermissionError] = useState<string | null>(null)

  const streamRef = useRef<MediaStream | null>(null)
  const lobbyVideoRef = useRef<HTMLVideoElement | null>(null)
  const sessionVideoRef = useRef<HTMLVideoElement | null>(null)
  // video 要素がマウントした瞬間にストリームをアタッチするための callback ref。
  // useEffect([status]) では DOM コミット前に status が更新されるため srcObject が設定されないことがある。
  const sessionVideoCallbackRef = useCallback((node: HTMLVideoElement | null) => {
    sessionVideoRef.current = node
    if (node && streamRef.current) {
      node.srcObject = streamRef.current
      node.play().catch(() => undefined)
    }
  }, [])

  const videoRecorderRef = useRef<MediaRecorder | null>(null)
  const videoChunksRef = useRef<Blob[]>([])

  // Lobby camera preview — 選択画面では getUserMedia しない（遷移直後の遅延を避ける）
  useEffect(() => {
    if (!shouldStartInterviewMediaPreview({ loading, status })) return
    let cancelled = false
    const startPreview = async () => {
      try {
        let stream = streamRef.current
        const needsNew =
          !stream || stream.getTracks().length === 0 || stream.getTracks().every((t) => t.readyState === 'ended')
        if (needsNew) {
          stream = await navigator.mediaDevices.getUserMedia(MEDIA_CONSTRAINTS)
          if (cancelled) {
            stream.getTracks().forEach((t) => t.stop())
            return
          }
          streamRef.current = stream
        }
        if (lobbyVideoRef.current && stream) {
          lobbyVideoRef.current.srcObject = stream
          lobbyVideoRef.current.play().catch(() => undefined)
        }
        setLobbyPermissionError(null)
      } catch (err: unknown) {
        if (cancelled) return
        const name = err instanceof DOMException ? err.name : err instanceof Error ? err.name : ''
        if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
          const blocked: string[] = []
          await navigator.mediaDevices
            .getUserMedia({ video: true })
            .then((s) => s.getTracks().forEach((t) => t.stop()))
            .catch(() => {
              blocked.push('カメラ')
            })
          await navigator.mediaDevices
            .getUserMedia({ audio: true })
            .then((s) => s.getTracks().forEach((t) => t.stop()))
            .catch(() => {
              blocked.push('マイク')
            })
          const target = blocked.length > 0 ? blocked.join('と') : 'マイクとカメラ'
          setLobbyPermissionError(`${target}へのアクセスが拒否されました。`)
        } else if (name === 'NotFoundError') {
          setLobbyPermissionError('マイクまたはカメラが見つかりません。デバイスを確認してください。')
        } else {
          setLobbyPermissionError('カメラの起動に失敗しました。')
        }
      }
    }
    void startPreview()
    return () => {
      cancelled = true
      // stream は面接中再利用のため streamRef に残す
    }
  }, [loading, status])

  // Attach stream to session video when connected
  useEffect(() => {
    if (status === 'connected' && sessionVideoRef.current && streamRef.current) {
      sessionVideoRef.current.srcObject = streamRef.current
      sessionVideoRef.current.play().catch(() => undefined)
    }
  }, [status])

  const classifyMediaError = async (err: unknown): Promise<string> => {
    const name = err instanceof DOMException ? err.name : err instanceof Error ? err.name : ''
    if (name === 'NotAllowedError' || name === 'PermissionDeniedError') {
      const blocked: string[] = []
      await navigator.mediaDevices
        .getUserMedia({ video: true })
        .then((s) => s.getTracks().forEach((t) => t.stop()))
        .catch(() => {
          blocked.push('カメラ')
        })
      await navigator.mediaDevices
        .getUserMedia({ audio: true })
        .then((s) => s.getTracks().forEach((t) => t.stop()))
        .catch(() => {
          blocked.push('マイク')
        })
      const target = blocked.length > 0 ? blocked.join('と') : 'マイクとカメラ'
      return `${target}へのアクセスが拒否されました。`
    }
    if (name === 'NotFoundError') {
      return 'マイクまたはカメラが見つかりません。デバイスを確認してください。'
    }
    return 'カメラの起動に失敗しました。'
  }

  /** ページリロードせず getUserMedia を再実行する（会社選択状態を維持）。 */
  const retryLobbyPreview = async () => {
    setLobbyPermissionError(null)
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((t) => t.stop())
      streamRef.current = null
    }
    try {
      const stream = await navigator.mediaDevices.getUserMedia(MEDIA_CONSTRAINTS)
      streamRef.current = stream
      // エラー UI 解除後に video がマウントされるのを待つ
      await new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
      })
      if (lobbyVideoRef.current) {
        lobbyVideoRef.current.srcObject = stream
        await lobbyVideoRef.current.play().catch(() => undefined)
      }
    } catch (err: unknown) {
      setLobbyPermissionError(await classifyMediaError(err))
    }
  }

  const ensureStream = async (): Promise<MediaStream> => {
    let stream = streamRef.current
    if (!stream || stream.getTracks().every((t) => t.readyState === 'ended')) {
      try {
        stream = await navigator.mediaDevices.getUserMedia(MEDIA_CONSTRAINTS)
      } catch (err: unknown) {
        const name = err instanceof DOMException ? err.name : err instanceof Error ? err.name : ''
        if (name === 'NotAllowedError' || name === 'PermissionDeniedError') throw new Error('NotAllowedError')
        if (name === 'NotFoundError') throw new Error('NotFoundError')
        throw err
      }
      streamRef.current = stream
    }
    return stream
  }

  const stopStream = () => {
    if (streamRef.current) {
      streamRef.current.getTracks().forEach(t => t.stop())
      streamRef.current = null
    }
  }

  const startVideoRecording = (stream: MediaStream) => {
    try {
      const mimeType = MediaRecorder.isTypeSupported('video/webm;codecs=vp9') ? 'video/webm;codecs=vp9' : 'video/webm'
      videoChunksRef.current = []
      // ビットレートを制限して 10 分録画でも約 25 MB 以内に収める
      // video: 300 kbps + audio: 32 kbps ≈ 332 kbps → 10 分 ≈ 24.9 MB
      const vr = new MediaRecorder(stream, {
        mimeType,
        videoBitsPerSecond: 300_000,
        audioBitsPerSecond: 32_000,
      })
      vr.ondataavailable = (e) => { if (e.data.size > 0) videoChunksRef.current.push(e.data) }
      vr.start(1000)
      videoRecorderRef.current = vr
    } catch { /* camera unavailable — skip recording */ }
  }

  const stopAndCollectVideoBlob = async (): Promise<Blob | null> => {
    const vr = videoRecorderRef.current
    let videoBlob: Blob | null = null
    if (vr && vr.state !== 'inactive') {
      await new Promise<void>((resolve) => {
        vr.onstop = () => resolve()
        vr.stop()
      })
      if (videoChunksRef.current.length > 0) {
        videoBlob = new Blob(videoChunksRef.current, { type: 'video/webm' })
      }
      videoRecorderRef.current = null
      videoChunksRef.current = []
    }
    return videoBlob
  }

  const toggleMic = () => {
    if (!streamRef.current) return
    const next = !micEnabled
    streamRef.current.getAudioTracks().forEach(t => { t.enabled = next })
    setMicEnabled(next)
  }

  const toggleCamera = () => {
    if (!streamRef.current) return
    const next = !cameraEnabled
    streamRef.current.getVideoTracks().forEach(t => { t.enabled = next })
    setCameraEnabled(next)
  }

  return {
    streamRef,
    lobbyVideoRef,
    sessionVideoRef,
    sessionVideoCallbackRef,
    micEnabled,
    cameraEnabled,
    setMicEnabled,
    setCameraEnabled,
    lobbyPermissionError,
    setLobbyPermissionError,
    toggleMic,
    toggleCamera,
    ensureStream,
    stopStream,
    startVideoRecording,
    stopAndCollectVideoBlob,
    retryLobbyPreview,
  }
}

export type InterviewMedia = ReturnType<typeof useInterviewMedia>
