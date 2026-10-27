'use client'

import { useEffect, useRef } from 'react'
import { Box } from '@mui/material'
import * as PIXI from 'pixi.js'
import { Live2DModel } from 'pixi-live2d-display/cubism4'

import {
  ensureCubismCore,
  toLive2DModelInfo,
  DEFAULT_MODEL_URL,
  type Live2DModelLike,
} from '@/lib/interview/live2d-loader'
import {
  inspectLive2DModel,
  isUsableAsInterviewer,
  mouthOpenValue,
  nodAngleY,
  PARAM,
  DEFICIENCY_REASON,
  type Live2DCapabilities,
} from '@/lib/interview/live2d-params'
import { Live2DMotion, pulse } from '@/lib/interview/live2d-motion'

declare global {
  interface Window {
    PIXI?: typeof PIXI
  }
}

/** pixi-live2d-display は `Live2DModel.registerTicker` かグローバル `window.PIXI` を見て自動更新する。一度でよい。 */
let tickerRegistered = false

type PixiLive2DModel = PIXI.Container & Live2DModelLike & {
  width: number
  height: number
  scale: { set(x: number, y?: number): void }
  anchor: { set(x: number, y?: number): void }
  position: { set(x: number, y?: number): void }
  destroy(options?: { children?: boolean; texture?: boolean; baseTexture?: boolean }): void
}

interface CubismCoreModel {
  setParameterValueById(id: string, value: number, weight?: number): void
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

/**
 * Live2Dの面接官アバター。
 *
 * `gender` は Avatar2D との props 互換のために受け取るが、v1では使わない
 * （Live2Dモデルは性別ごとに用意していない。配色の出し分けはモデル差し替え時に
 * 検討する）。口は顔パーツのビセームではなく `level`/`speaking` の音声振幅で
 * 駆動する（Avatar2D と同じ入力源）。
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
  const levelRef = useRef(level)
  const speakingRef = useRef(speaking)
  const motionRef = useRef<Live2DMotion | null>(null)
  const capsRef = useRef<Live2DCapabilities | null>(null)
  const onUnavailableRef = useRef(onUnavailable)
  onUnavailableRef.current = onUnavailable

  useEffect(() => { levelRef.current = level }, [level])
  useEffect(() => { speakingRef.current = speaking }, [speaking])
  useEffect(() => {
    if (acknowledgeSignal > 0) motionRef.current?.acknowledge(performance.now() / 1000)
  }, [acknowledgeSignal])

  useEffect(() => {
    if (!containerRef.current) return
    const container = containerRef.current
    let disposed = false
    let app: PIXI.Application | null = null
    let pixiModel: PixiLive2DModel | null = null
    let natural: { width: number; height: number } | null = null

    const bail = (reason: string, err?: unknown) => {
      if (err !== undefined) console.warn(`[Live2DAvatar] ${reason}`, err)
      else console.warn(`[Live2DAvatar] ${reason}`)
      if (!disposed) onUnavailableRef.current()
    }

    const layout = () => {
      if (!app || !pixiModel || !natural) return
      const w = container.clientWidth
      const h = container.clientHeight
      if (w === 0 || h === 0) return
      app.renderer.resize(w, h)
      const scale = (Math.min(w / natural.width, h / natural.height) || 1) * 0.98
      pixiModel.scale.set(scale)
      pixiModel.position.set(w / 2, h / 2)
    }

    const setup = async () => {
      try {
        await ensureCubismCore()
      } catch (err) {
        bail('Cubism Coreを読み込めなかった（public/live2d/core/ に未配置の可能性）', err)
        return
      }
      if (disposed) return

      window.PIXI = PIXI
      if (!tickerRegistered) {
        Live2DModel.registerTicker(PIXI.Ticker)
        tickerRegistered = true
      }

      let loaded: Live2DModelLike
      try {
        loaded = (await Live2DModel.from(DEFAULT_MODEL_URL)) as unknown as Live2DModelLike
      } catch (err) {
        bail('モデルを読み込めなかった（public/live2d/model/ に未配置の可能性）', err)
        return
      }
      if (disposed) return

      const caps = inspectLive2DModel(toLive2DModelInfo(loaded))
      if (!isUsableAsInterviewer(caps)) {
        bail(`モデルが面接官として使えない: ${caps.deficiencies.map((d) => DEFICIENCY_REASON[d]).join(' / ')}`)
        return
      }
      if (caps.deficiencies.length > 0) {
        console.warn(`[Live2DAvatar] ${caps.deficiencies.map((d) => DEFICIENCY_REASON[d]).join(' / ')}`)
      }
      capsRef.current = caps
      motionRef.current = new Live2DMotion()

      pixiModel = loaded as unknown as PixiLive2DModel
      natural = { width: pixiModel.width || 1, height: pixiModel.height || 1 }
      pixiModel.anchor.set(0.5, 0.5)

      const width = container.clientWidth
      const height = container.clientHeight
      app = new PIXI.Application({
        width,
        height,
        backgroundAlpha: 0,
        antialias: true,
        resolution: Math.min(window.devicePixelRatio, 2),
        autoDensity: true,
      })
      container.appendChild(app.view as unknown as Node)
      app.stage.addChild(pixiModel)
      layout()

      app.ticker.add(() => {
        if (disposed) return
        const motion = motionRef.current
        const c = capsRef.current
        if (!motion || !c) return
        const now = performance.now() / 1000

        const frame = motion.update(now)
        const core = loaded.internalModel.coreModel as unknown as CubismCoreModel

        const mouthValue = mouthOpenValue(levelRef.current, speakingRef.current)
        for (const id of c.mouthParamIds) core.setParameterValueById(id, mouthValue)

        if (c.canNod) {
          const angle = frame.nodProgress !== null ? nodAngleY(frame.nodProgress) : 0
          core.setParameterValueById(PARAM.angleY, angle)
        }

        if (c.blinkParamIds.length > 0) {
          const openness = frame.blinkProgress !== null ? 1 - pulse(frame.blinkProgress) : 1
          for (const id of c.blinkParamIds) core.setParameterValueById(id, openness)
        }
      })
    }

    setup()

    window.addEventListener('resize', layout)

    return () => {
      disposed = true
      window.removeEventListener('resize', layout)
      pixiModel?.destroy({ children: true, texture: true, baseTexture: true })
      if (app) {
        const view = app.view as unknown as Node
        app.destroy(false, { children: true, texture: true, baseTexture: true })
        if (view.parentNode === container) container.removeChild(view)
      }
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
    />
  )
}
