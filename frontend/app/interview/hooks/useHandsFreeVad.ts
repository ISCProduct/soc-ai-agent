'use client'

import { useEffect, type MutableRefObject, type RefObject } from 'react'
import type { InterviewStatus } from '../types'
import { initialVadState, stepVad, type VadState } from '../lib/vadDecision'

// しきい値・時間定数は lib/vadDecision.ts に集約した。
// 判定を純粋関数にして、実音声なしで境界を検証できるようにしている。
export {
  SILENCE_MS,
  MIN_RECORDING_MS,
  CONFIRM_MS,
  WARMUP_FRAMES,
} from '../lib/vadDecision'

type UseHandsFreeVadArgs = {
  enabled: boolean
  status: InterviewStatus
  streamRef: RefObject<MediaStream | null>
  isRecordingRef: MutableRefObject<boolean>
  turnPendingRef: MutableRefObject<boolean>
  aiSpeakingRef: MutableRefObject<boolean>
  startRecording: () => void
  stopRecording: () => void
  /** 発話が確定しなかった録音を送らずに捨てる。 */
  discardRecording: () => void
}

/**
 * ハンズフリー VAD: 音声検知で自動録音開始・停止。
 * stream / recording フラグの refs は呼び出し側が所有する。
 *
 * 判定そのものは lib/vadDecision.ts の純粋関数が行う。
 * ここは AudioContext から RMS を取り出して結果を実行するだけに留める。
 */
export function useHandsFreeVad({
  enabled,
  status,
  streamRef,
  isRecordingRef,
  turnPendingRef,
  aiSpeakingRef,
  startRecording,
  stopRecording,
  discardRecording,
}: UseHandsFreeVadArgs) {
  useEffect(() => {
    if (!enabled || status !== 'connected' || !streamRef.current) return
    const audioCtx = new AudioContext()
    const source = audioCtx.createMediaStreamSource(streamRef.current)
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 512
    source.connect(analyser)
    const buf = new Float32Array(analyser.fftSize)
    let vad: VadState = initialVadState()
    let rafId: number

    const tick = () => {
      rafId = requestAnimationFrame(tick)
      analyser.getFloatTimeDomainData(buf)
      const rms = Math.sqrt(buf.reduce((s, v) => s + v * v, 0) / buf.length)

      // 送信中・AI発話中は新しい録音を始めない。
      const canStart = !isRecordingRef.current && !turnPendingRef.current && !aiSpeakingRef.current
      const { state, action } = stepVad(vad, { rms, now: Date.now(), canStart })
      vad = state

      switch (action) {
        case 'start':
          startRecording()
          break
        case 'stop':
          stopRecording()
          break
        case 'discard':
          discardRecording()
          break
      }
    }

    rafId = requestAnimationFrame(tick)
    return () => {
      cancelAnimationFrame(rafId)
      source.disconnect()
      audioCtx.close().catch(() => {})
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, status])
}
