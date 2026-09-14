'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import type { InterviewStatus } from '../types'
import { shouldStartInterviewMediaPreview } from './mediaPreviewGate'
import {
  MEDIA_CONSTRAINTS,
  classifyMediaErrorKind,
  isStreamUsable,
  mediaErrorMessage,
} from '../lib/mediaStream'

// 制約は lib/mediaStream.ts に集約した。既存の import 先を壊さないため再輸出する。
export { MEDIA_CONSTRAINTS }

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
  // getUserMedia の同時実行を1本にまとめる。
  // ロビープレビューの effect と handleStart の ensureStream() は
  // status の変化を挟んでほぼ同時に走るため、まとめないと
  // 取り残したストリームがデバイスを掴み続ける。
  const acquiringRef = useRef<Promise<MediaStream> | null>(null)
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

  /**
   * ストリームを1本だけ確保する。
   *
   * 同時に呼ばれても getUserMedia は1回しか走らず、全員が同じ
   * ストリームを受け取る。これが無いと、先に代入されたストリームが
   * stop() されないまま取り残され、デバイスを掴み続ける。
   */
  const acquireStream = useCallback(async (): Promise<MediaStream> => {
    if (isStreamUsable(streamRef.current)) return streamRef.current as MediaStream
    if (acquiringRef.current) return acquiringRef.current

    const pending = (async () => {
      const stream = await navigator.mediaDevices.getUserMedia(MEDIA_CONSTRAINTS)
      // 待っている間に別経路が使えるストリームを確保していたら、
      // 今取得したほうを捨てる。掴みっぱなしを作らない。
      const existing = streamRef.current
      if (existing && existing !== stream && isStreamUsable(existing)) {
        stream.getTracks().forEach((t) => t.stop())
        return existing
      }
      // 死んだストリームが残っていれば明示的に解放してから置き換える。
      if (existing && existing !== stream) {
        existing.getTracks().forEach((t) => t.stop())
      }
      streamRef.current = stream
      return stream
    })()

    acquiringRef.current = pending
    try {
      return await pending
    } finally {
      if (acquiringRef.current === pending) acquiringRef.current = null
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
        // 取得は acquireStream に一本化する。ここで直接 getUserMedia すると
        // handleStart の ensureStream() と二重に走り、片方が取り残される。
        const stream = await acquireStream()
        // cancelled でもストリームは止めない。streamRef が所有しており、
        // 面接中も使い続ける。ここで止めると handleStart 側が死んだストリームを掴む。
        if (cancelled) return
        if (lobbyVideoRef.current && stream) {
          lobbyVideoRef.current.srcObject = stream
          lobbyVideoRef.current.play().catch(() => undefined)
        }
        setLobbyPermissionError(null)
      } catch (err: unknown) {
        if (cancelled) return
        setLobbyPermissionError(await classifyMediaError(err))
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
    const kind = classifyMediaErrorKind(err)
    if (kind !== 'denied') return mediaErrorMessage(kind)

    // 権限拒否と分かっている場合だけ、どちらが拒否されたのかを個別に確かめる。
    // デバイス使用中など他の失敗でここへ来ると、この問い合わせも失敗して
    // 「許可したのに権限エラー」と誤表示する原因になる。
    const blocked: string[] = []
    for (const [label, constraints] of [
      ['カメラ', { video: true }],
      ['マイク', { audio: true }],
    ] as const) {
      try {
        const probe = await navigator.mediaDevices.getUserMedia(constraints)
        probe.getTracks().forEach((t) => t.stop())
      } catch (probeErr) {
        // 拒否されたものだけを挙げる。使用中や未検出は権限の問題ではない。
        if (classifyMediaErrorKind(probeErr) === 'denied') blocked.push(label)
      }
    }
    return mediaErrorMessage('denied', blocked.length > 0 ? blocked.join('と') : undefined)
  }

  /** ページリロードせず getUserMedia を再実行する（会社選択状態を維持）。 */
  const retryLobbyPreview = async () => {
    setLobbyPermissionError(null)
    if (streamRef.current) {
      streamRef.current.getTracks().forEach((t) => t.stop())
      streamRef.current = null
    }
    // 進行中の取得があれば、その結果を掴まないよう捨てる
    acquiringRef.current = null
    try {
      const stream = await acquireStream()
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
    try {
      return await acquireStream()
    } catch (err: unknown) {
      // 呼び出し側 (parseMediaError) が種別で分岐できるよう名前を残す。
      // 使用中 (busy) を権限拒否に丸めない。
      const kind = classifyMediaErrorKind(err)
      if (kind === 'denied') throw new Error('NotAllowedError')
      if (kind === 'notfound') throw new Error('NotFoundError')
      if (kind === 'busy') throw new Error('NotReadableError')
      throw err
    }
  }

  const stopStream = () => {
    // 進行中の取得結果が後から streamRef に入るのを防ぐ。
    // これが無いと、停止したつもりのストリームが残って次回の取得を妨げる。
    const pending = acquiringRef.current
    acquiringRef.current = null
    if (pending) {
      void pending
        .then((s) => {
          s.getTracks().forEach((t) => t.stop())
          // 遅れて代入された参照も外す。残すと次回「使えないストリーム」を
          // 掴んだ状態から始まる。
          if (streamRef.current === s) streamRef.current = null
        })
        .catch(() => {})
    }
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
