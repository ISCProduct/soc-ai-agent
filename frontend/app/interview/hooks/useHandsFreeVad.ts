'use client'

import { useEffect, type MutableRefObject, type RefObject } from 'react'
import type { InterviewStatus } from '../types'

/** 発話検知の音量閾値 (RMS) */
export const VAD_THRESHOLD = 0.015
/** この無音時間が続いたら自動送信（長めに設定して途切れ防止） */
export const SILENCE_MS = 2500
/** 録音開始後この時間は自動停止しない（息継ぎなどで誤停止しない） */
export const MIN_RECORDING_MS = 1000

type UseHandsFreeVadArgs = {
  enabled: boolean
  status: InterviewStatus
  streamRef: RefObject<MediaStream | null>
  isRecordingRef: MutableRefObject<boolean>
  turnPendingRef: MutableRefObject<boolean>
  aiSpeakingRef: MutableRefObject<boolean>
  startRecording: () => void
  stopRecording: () => void
}

/**
 * ハンズフリー VAD: 音声検知で自動録音開始・停止。
 * stream / recording フラグの refs は呼び出し側が所有する。
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
}: UseHandsFreeVadArgs) {
  useEffect(() => {
    if (!enabled || status !== 'connected' || !streamRef.current) return
    const audioCtx = new AudioContext()
    const source = audioCtx.createMediaStreamSource(streamRef.current)
    const analyser = audioCtx.createAnalyser()
    analyser.fftSize = 512
    source.connect(analyser)
    const buf = new Float32Array(analyser.fftSize)
    let silenceStart: number | null = null
    let recordingStartTime: number | null = null
    let rafId: number
    const tick = () => {
      rafId = requestAnimationFrame(tick)
      analyser.getFloatTimeDomainData(buf)
      const rms = Math.sqrt(buf.reduce((s, v) => s + v * v, 0) / buf.length)
      const speaking = rms > VAD_THRESHOLD
      if (speaking) {
        silenceStart = null
        if (!isRecordingRef.current && !turnPendingRef.current && !aiSpeakingRef.current) {
          recordingStartTime = Date.now()
          startRecording()
        }
      } else if (isRecordingRef.current) {
        // 録音開始直後の短い無音（息継ぎ等）では止めない
        const elapsed = recordingStartTime ? Date.now() - recordingStartTime : Infinity
        if (elapsed < MIN_RECORDING_MS) return
        if (silenceStart === null) {
          silenceStart = Date.now()
        } else if (Date.now() - silenceStart > SILENCE_MS) {
          silenceStart = null
          recordingStartTime = null
          stopRecording()
        }
      } else {
        silenceStart = null
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
