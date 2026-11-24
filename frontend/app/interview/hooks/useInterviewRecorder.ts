'use client'

import { useRef, useState } from 'react'
import type { InterviewMedia } from './useInterviewMedia'
import { pickRecorderFormat, type RecorderFormat } from '../lib/recorderFormat'

type UseInterviewRecorderArgs = {
  media: InterviewMedia
  /**
   * 録音が確定したときに呼ぶ送信処理。
   * 録音を始めたレンダーの関数がそのまま呼ばれる（分割前と同じ閉包）。
   */
  sendTurn: () => Promise<void>
}

/**
 * 回答の録音（開始・送信・破棄）と、録音中/応答待ちのフラグ。
 * 録音した音声の送信は sendTurn（useInterviewTurns）が行う。
 */
export function useInterviewRecorder({ media, sendTurn }: UseInterviewRecorderArgs) {
  const [isRecording, _setIsRecording] = useState(false)
  const [turnPending, _setTurnPending] = useState(false)
  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const recorderFormatRef = useRef<RecorderFormat>({ mimeType: '', ext: 'webm' })
  // 発話が確定しなかった録音を送らずに捨てるためのフラグ。
  // MediaRecorder.stop() は非同期で onstop を呼ぶため、
  // 停止要求の時点でどちらの意図か記録しておく必要がある。
  const discardTurnRef = useRef(false)
  const audioChunksRef = useRef<Blob[]>([])
  const isRecordingRef = useRef(false)
  const turnPendingRef = useRef(false)
  // VAD effect は hooks 順序のため先に登録し、実装は ref 経由で呼ぶ
  const startRecordingRef = useRef<() => void>(() => {})
  const stopRecordingRef = useRef<() => void>(() => {})
  const discardRecordingRef = useRef<() => void>(() => {})

  // ref と state を常に同期（VAD の stale closure 対策）
  const setIsRecording = (v: boolean) => { isRecordingRef.current = v; _setIsRecording(v) }
  const setTurnPending = (v: boolean) => { turnPendingRef.current = v; _setTurnPending(v) }

  const startRecording = () => {
    if (!media.streamRef.current || isRecordingRef.current || turnPendingRef.current) return
    const audioTracks = media.streamRef.current.getAudioTracks()
    if (audioTracks.length === 0) return
    const micStream = new MediaStream(audioTracks)
    audioChunksRef.current = []
    discardTurnRef.current = false
    // mimeType はブラウザが実際に出せる形式から選ぶ。'audio/webm' 固定だと
    // Safari では NotSupportedError で例外になり、録音が始まらなかった。
    const format = pickRecorderFormat()
    recorderFormatRef.current = format
    const mr = new MediaRecorder(
      micStream,
      format.mimeType
        ? { mimeType: format.mimeType, audioBitsPerSecond: 128000 }
        : { audioBitsPerSecond: 128000 },
    )
    mr.ondataavailable = (e) => { if (e.data.size > 0) audioChunksRef.current.push(e.data) }
    mr.onstop = () => {
      if (discardTurnRef.current) {
        discardTurnRef.current = false
        audioChunksRef.current = []
        return
      }
      void sendTurn()
    }
    mediaRecorderRef.current = mr
    mr.start()
    setIsRecording(true)
  }
  startRecordingRef.current = startRecording

  const stopRecording = () => {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === 'inactive') return
    mediaRecorderRef.current.stop()
    setIsRecording(false)
    setTurnPending(true)
  }
  stopRecordingRef.current = stopRecording

  /**
   * 発話が確定しなかった録音を破棄する（送信しない）。
   *
   * VAD は頭切れを避けるため低いしきい値で録音を始める。
   * 物音で始まった録音まで送ると、無音や雑音だけの音声で
   * STT を呼ぶことになり、費用と誤認識が増える。
   */
  const discardRecording = () => {
    if (!mediaRecorderRef.current || mediaRecorderRef.current.state === 'inactive') return
    discardTurnRef.current = true
    mediaRecorderRef.current.stop()
    setIsRecording(false)
  }
  discardRecordingRef.current = discardRecording

  return {
    isRecording,
    setIsRecording,
    isRecordingRef,
    turnPending,
    setTurnPending,
    turnPendingRef,
    mediaRecorderRef,
    recorderFormatRef,
    discardTurnRef,
    audioChunksRef,
    startRecordingRef,
    stopRecordingRef,
    discardRecordingRef,
    startRecording,
    stopRecording,
  }
}
