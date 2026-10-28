'use client'

import { useEffect, useRef } from 'react'
import { Box } from '@mui/material'

import { ensureCubismCore, DEFAULT_MODEL_URL, DEFAULT_SHADER_PATH } from '@/lib/interview/live2d-loader'
import type { Live2DFrameworkModel as Live2DFrameworkModelClass } from '@/lib/interview/live2d-framework-model'
import {
  inspectLive2DModel,
  isUsableAsInterviewer,
  mouthOpenValue,
  smoothMouth,
  valueAt,
  normalizeLevel,
  nodAngleY,
  DEFICIENCY_REASON,
  type Live2DCapabilities,
} from '@/lib/interview/live2d-params'
import { Live2DMotion, pulse } from '@/lib/interview/live2d-motion'
import { getAiAudioMeter } from '@/lib/interview/ai-audio-meter'

// 全身絵の余白を切って顔を大きく見せる。FACE_FOCUS_Y は画像の上端0・下端1とした顔の中心位置。
// 書き出し済み男性モデルの絵に合わせた値なので、モデルを差し替えたら見直すこと。
const FACE_ZOOM = 1.6
const FACE_FOCUS_Y = 0.36

/**
 * 口と声のタイミング微調整(ミリ秒)。正の値で口を遅らせる。出力遅延は自動で補正するが、
 * 機器によるずれが残るときに、ブラウザのコンソールで
 * `localStorage.setItem('lipSyncOffsetMs', '80')` のように指定する（既定 0）。
 */
function lipSyncOffsetSec(): number {
  try {
    const v = Number(localStorage.getItem('lipSyncOffsetMs'))
    return Number.isFinite(v) ? Math.max(-300, Math.min(500, v)) / 1000 : 0
  } catch {
    return 0
  }
}

export interface Live2DAvatarProps {
  gender: 'male' | 'female'
  /** AI発話の音声振幅 0-1。speaking中の口の開き具合に使う（Avatar2D と同じ意味）。 */
  level: number
  speaking: boolean
  /** 面接が始まっているか。 */
  active: boolean
  /** 面接官が次の質問を用意している最中か。 */
  pending?: boolean
  /** 学生の発話の区切りで増える値。変わるたびに相槌を1回打つ。 */
  acknowledgeSignal?: number
  /**
   * Core未配置・モデル未配置・モデルが面接官として要件を満たさない場合に呼ばれる。
   * 親（InterviewerAvatar）はこれを受けて Avatar2D へ切り替える。描画は止めない。
   */
  onUnavailable: () => void
}

function modelDirOf(modelUrl: string): { dir: string; fileName: string } {
  const i = modelUrl.lastIndexOf('/')
  return { dir: modelUrl.slice(0, i + 1), fileName: modelUrl.slice(i + 1) }
}

/**
 * Live2Dの面接官アバター。公式Cubism Web Framework（`lib/interview/live2d-framework-model.ts`）を
 * 直接使って素のcanvas+WebGLに描画する（`pixi-live2d-display`は現行Coreと非互換なため不採用。
 * 詳細はそちらのファイル先頭コメント参照）。
 *
 * `gender` は Avatar2D との props 互換のために受け取るが、v1では使わない
 * （Live2Dモデルは性別ごとに用意していない）。口は顔パーツのビセームではなく
 * `level`/`speaking` の音声振幅で駆動する（Avatar2D と同じ入力源）。
 */
export default function Live2DAvatar({
  level,
  speaking,
  active,
  pending = false,
  acknowledgeSignal = 0,
  onUnavailable,
}: Live2DAvatarProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const levelRef = useRef(level)
  const mouthRef = useRef(0)
  const speakingRef = useRef(speaking)
  const motionRef = useRef<Live2DMotion | null>(null)
  const capsRef = useRef<Live2DCapabilities | null>(null)
  const onUnavailableRef = useRef(onUnavailable)
  onUnavailableRef.current = onUnavailable

  useEffect(() => {
    levelRef.current = level
  }, [level])
  useEffect(() => {
    speakingRef.current = speaking
  }, [speaking])
  useEffect(() => {
    if (acknowledgeSignal > 0) motionRef.current?.acknowledge(performance.now() / 1000)
  }, [acknowledgeSignal])

  useEffect(() => {
    if (!containerRef.current || !canvasRef.current) return
    const container = containerRef.current
    const canvas = canvasRef.current
    let disposed = false
    let rafId = 0
    let lastTime = 0
    let model: Live2DFrameworkModelClass | null = null

    const bail = (reason: string, err?: unknown) => {
      if (err !== undefined) console.warn(`[Live2DAvatar] ${reason}`, err)
      else console.warn(`[Live2DAvatar] ${reason}`)
      if (!disposed) onUnavailableRef.current()
    }

    const resize = () => {
      const w = container.clientWidth
      const h = container.clientHeight
      if (w === 0 || h === 0) return
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      const pxW = Math.round(w * dpr)
      const pxH = Math.round(h * dpr)
      if (canvas.width !== pxW) canvas.width = pxW
      if (canvas.height !== pxH) canvas.height = pxH
      model?.resize(pxW, pxH)
    }

    const setup = async () => {
      try {
        await ensureCubismCore()
      } catch (err) {
        bail('Cubism Coreを読み込めなかった（public/live2d/core/ に未配置の可能性）', err)
        return
      }
      if (disposed) return

      // Framework（vendor/live2d-framework）は一部ファイルがモジュール評価時に
      // `Live2DCubismCore`（Core読込後に生えるグローバル）を参照するため、
      // Core読込より前に静的importすると ReferenceError になる。動的importする。
      const { ensureCubismFramework, Live2DFrameworkModel } = await import('@/lib/interview/live2d-framework-model')
      ensureCubismFramework()

      const gl = canvas.getContext('webgl', { alpha: true, premultipliedAlpha: true }) as WebGLRenderingContext | null
      if (!gl) {
        bail('WebGLコンテキストを取得できなかった')
        return
      }
      gl.clearColor(0, 0, 0, 0)

      resize()
      const { dir, fileName } = modelDirOf(DEFAULT_MODEL_URL)

      let loaded: Live2DFrameworkModelClass
      try {
        loaded = await Live2DFrameworkModel.load(gl, dir, fileName, DEFAULT_SHADER_PATH, canvas.width, canvas.height)
      } catch (err) {
        bail('モデルを読み込めなかった（public/live2d/model/ に未配置の可能性）', err)
        return
      }
      if (disposed) {
        loaded.release()
        return
      }

      const caps = inspectLive2DModel(loaded.getCapabilitiesInfo())
      if (!isUsableAsInterviewer(caps)) {
        bail(`モデルが面接官として使えない: ${caps.deficiencies.map((d) => DEFICIENCY_REASON[d]).join(' / ')}`)
        loaded.release()
        return
      }
      if (caps.deficiencies.length > 0) {
        console.warn(`[Live2DAvatar] ${caps.deficiencies.map((d) => DEFICIENCY_REASON[d]).join(' / ')}`)
      }
      capsRef.current = caps
      motionRef.current = new Live2DMotion()
      model = loaded

      const levelHistory: { t: number; v: number }[] = []
      let levelPeak = 0
      const tick = (nowMs: number) => {
        if (disposed || !model) return
        const now = nowMs / 1000
        const dt = lastTime === 0 ? 0 : Math.min(now - lastTime, 0.1)
        lastTime = now

        const motion = motionRef.current
        const c = capsRef.current
        if (motion && c) {
          const frame = motion.update(now)
          // 音量は状態を経由せず直接読み、再生の出力遅延ぶん遅らせて、聞こえる声に口を合わせる。
          const meter = getAiAudioMeter()
          if (!speakingRef.current) levelPeak = 0
          const rawLevel = meter ? meter.read() : levelRef.current
          const norm = normalizeLevel(rawLevel, levelPeak, dt)
          levelPeak = norm.peak
          levelHistory.push({ t: now, v: norm.level })
          while (levelHistory.length > 1 && levelHistory[1].t < now - 1) levelHistory.shift()
          const delayed = valueAt(levelHistory, now - (meter?.latencySec ?? 0) - lipSyncOffsetSec())
          mouthRef.current = smoothMouth(mouthRef.current, mouthOpenValue(delayed, speakingRef.current), dt)
          const mouthValue = mouthRef.current
          for (const id of c.mouthParamIds) model.setParameterValue(id, mouthValue)

          if (c.nodParamId) {
            model.setParameterValue(c.nodParamId, frame.nodProgress !== null ? nodAngleY(frame.nodProgress) : 0)
          }

          const eyeOpenness = frame.blinkProgress !== null ? 1 - pulse(frame.blinkProgress) : 1
          if (c.blinkParamIds.length > 0) {
            for (const id of c.blinkParamIds) model.setParameterValue(id, eyeOpenness)
          }
          // 差分画像を重ねるタイプのモデル（目閉じ・口開きがパラメータに結び付いていない）用。
          model.setOverlayState(eyeOpenness, mouthValue)
        }

        model.commitFrame(dt)
        gl.viewport(0, 0, canvas.width, canvas.height)
        gl.clear(gl.COLOR_BUFFER_BIT)
        model.draw(gl, canvas.width, canvas.height, 0.98, FACE_ZOOM, FACE_FOCUS_Y)

        rafId = requestAnimationFrame(tick)
      }
      rafId = requestAnimationFrame(tick)
    }

    setup()

    const resizeObserver = new ResizeObserver(resize)
    resizeObserver.observe(container)

    return () => {
      disposed = true
      if (rafId) cancelAnimationFrame(rafId)
      resizeObserver.disconnect()
      model?.release()
      motionRef.current = null
      capsRef.current = null
    }
  }, [])

  return (
    <Box
      ref={containerRef}
      sx={{
        width: '100%',
        height: '100%',
        position: 'relative',
        // Avatar2D と同じ扱い: 面接開始前・質問準備中は少し薄くする。
        opacity: active || pending ? 1 : 0.75,
        transition: 'opacity 0.3s ease',
      }}
    >
      <canvas ref={canvasRef} style={{ width: '100%', height: '100%', display: 'block' }} />
    </Box>
  )
}
