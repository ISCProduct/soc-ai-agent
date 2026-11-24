'use client'

import { useRef, useState } from 'react'
import { measureLevel, setAiAudioMeter } from '@/lib/interview/ai-audio-meter'

/**
 * 面接官の応答音声の再生と、リップシンク用の音量（aiLevel）。
 * 停止（cleanupConnection）は useInterviewSession 側で refs を直接操作する。
 */
export function useInterviewAudioPlayback() {
  const [aiLevel, setAiLevel] = useState(0)
  const [aiSpeaking, _setAiSpeaking] = useState(false)
  const aiAudioRef = useRef<HTMLAudioElement | null>(null)
  const aiAudioCtxRef = useRef<AudioContext | null>(null)
  const aiLevelRafRef = useRef<number | null>(null)
  /** playAudioBlob の世代。cleanupConnection で increment し、古い再生を破棄する */
  const audioGenerationRef = useRef(0)
  const aiSpeakingRef = useRef(false)

  // ref と state を常に同期（VAD の stale closure 対策）
  const setAiSpeaking = (v: boolean) => { aiSpeakingRef.current = v; _setAiSpeaking(v) }

  /**
   * ターンの応答音声を再生する。
   * gen は「リクエストを投げた時点」の音声世代で、呼び出し側から引き継ぐ(#1501)。
   * ここで audioGenerationRef を読むと、応答待ちの間に「完了する」で
   * cleanupConnection が世代を進めていても更新後の値を読むため stale 判定を通過し、
   * レポート画面へ遷移した後に面接官の音声が鳴り出す。
   */
  const playAudioBlob = async (blob: Blob, gen: number): Promise<void> => {
    // 終了済み（または次のターンに切り替わった後）なら、再生を始めない
    if (gen !== audioGenerationRef.current) return
    const url = URL.createObjectURL(blob)
    const el = new Audio()
    aiAudioRef.current = el
    el.src = url
    setAiSpeaking(true)

    let rafId: number | null = null

    const cleanup = () => {
      if (rafId !== null) {
        cancelAnimationFrame(rafId)
        rafId = null
      }
      // この要素が現在共有されている再生対象でない場合(=より新しいターンの
      // 音声に切り替わった後の古いクリーンアップ)は、新しい方の再生状態
      // (aiSpeaking/aiLevel/RAF)を巻き戻さないよう、自身のリソース解放のみ行う
      const isCurrent = aiAudioRef.current === el
      if (isCurrent) {
        if (aiLevelRafRef.current !== null) {
          cancelAnimationFrame(aiLevelRafRef.current)
          aiLevelRafRef.current = null
        }
        setAiLevel(0)
        setAiSpeaking(false)
        setAiAudioMeter(null)
        aiAudioRef.current = null
      }
      URL.revokeObjectURL(url)
      el.removeAttribute('src')
      el.load()
    }

    try {
      if (!aiAudioCtxRef.current || aiAudioCtxRef.current.state === 'closed') {
        aiAudioCtxRef.current = new AudioContext()
      }
      const ctx = aiAudioCtxRef.current
      await ctx.resume()
      // await中に次のターンが始まっていれば、この古い音声は再生せず破棄する
      if (gen !== audioGenerationRef.current) { cleanup(); return }

      const source = ctx.createMediaElementSource(el)
      const analyser = ctx.createAnalyser()
      analyser.fftSize = 512
      analyser.smoothingTimeConstant = 0.6
      source.connect(analyser)
      analyser.connect(ctx.destination)

      const timeData = new Uint8Array(analyser.fftSize)
      const trackLevel = () => {
        setAiLevel(measureLevel(analyser, timeData))
        rafId = requestAnimationFrame(trackLevel)
      }
      // Live2D は状態(再描画)を経由せずここから直接読む。出力遅延ぶん音より先に測れてしまうので、
      // 遅れの秒数も渡して、描画側で口を遅らせる。
      const meterBuf = new Uint8Array(analyser.fftSize)
      setAiAudioMeter({
        read: () => measureLevel(analyser, meterBuf),
        latencySec: (ctx.outputLatency || 0) + (ctx.baseLatency || 0),
      })
      rafId = requestAnimationFrame(trackLevel)
      aiLevelRafRef.current = rafId
    } catch (err) {
      // AudioContext 未対応時は Audio 要素のデフォルト出力にフォールバック（リップシンクなし）
      console.warn('[Interview] AudioContext routing unavailable; playing via element output', err)
    }

    // AudioContext設定中に次のターンが始まっていれば、ここでも再生せず破棄する
    if (gen !== audioGenerationRef.current) { cleanup(); return }

    return new Promise<void>((resolve) => {
      el.onended = () => {
        cleanup()
        resolve()
      }
      el.onerror = () => {
        console.error('[Interview] AI audio element error', el.error)
        cleanup()
        resolve()
      }
      el.play().catch((err) => {
        console.error('[Interview] AI audio play() failed (check CSP media-src / autoplay)', err)
        cleanup()
        resolve()
      })
    })
  }

  return {
    aiLevel,
    setAiLevel,
    aiSpeaking,
    setAiSpeaking,
    aiSpeakingRef,
    aiAudioRef,
    aiAudioCtxRef,
    aiLevelRafRef,
    audioGenerationRef,
    playAudioBlob,
  }
}
