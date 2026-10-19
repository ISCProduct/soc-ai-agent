'use client'

import { useEffect, useRef, useState } from 'react'
import { Box } from '@mui/material'

export interface Avatar2DProps {
  gender: 'male' | 'female'
  /** AI発話の音声振幅 0-1。speaking中の口の開き具合に使う。 */
  level: number
  speaking: boolean
  /** 面接が始まっているか。falseの間はまばたきだけの待機表示にする。 */
  active: boolean
  /** 面接官が次の質問を用意している最中か。 */
  pending?: boolean
  /** 学生の発話の区切りで増える値。変わるたびに相槌(うなずき)を1回する。 */
  acknowledgeSignal?: number
}

const BLINK_INTERVAL_MS: [number, number] = [2500, 6000]
const BLINK_DURATION_MS = 120
const NOD_DURATION_MS = 450

function randomBetween([lo, hi]: [number, number]): number {
  return lo + Math.random() * (hi - lo)
}

// 肌色は性別と無関係な属性なので固定にし、髪色・服色のみをgenderで変える（レビュー指摘）。
const SKIN = '#e8b38c'
const PALETTE = {
  male: { skin: SKIN, hair: '#1f1f1f', outfit: '#2d3a4f' },
  female: { skin: SKIN, hair: '#3a2a22', outfit: '#4a2d3a' },
} as const

/**
 * 2Dイラストの面接官アバター。3D(ThreeAvatar)からの移行(SOCAIAGENT-2D)。
 *
 * 実運用の3Dモデル(male/female-avatar.glb)は骨格・モーフターゲットを
 * 持たない静止メッシュで、うなずき・口パクが実質機能していなかった
 * (avatar-capabilities.tsのコメント参照)。素直な図形の差分アニメーションで
 * 同等以上の「喋っている感」を、three.js/GLTF依存無しで出す。
 */
export default function Avatar2D({ gender, level, speaking, active, pending = false, acknowledgeSignal = 0 }: Avatar2DProps) {
  const [blinking, setBlinking] = useState(false)
  const [nodding, setNodding] = useState(false)
  const prevAckRef = useRef(acknowledgeSignal)

  useEffect(() => {
    let cancelled = false
    let blinkTimer: ReturnType<typeof setTimeout>
    let closeTimer: ReturnType<typeof setTimeout>

    const scheduleBlink = () => {
      blinkTimer = setTimeout(() => {
        if (cancelled) return
        setBlinking(true)
        closeTimer = setTimeout(() => {
          if (!cancelled) setBlinking(false)
        }, BLINK_DURATION_MS)
        scheduleBlink()
      }, randomBetween(BLINK_INTERVAL_MS))
    }
    scheduleBlink()

    return () => {
      cancelled = true
      clearTimeout(blinkTimer)
      clearTimeout(closeTimer)
    }
  }, [])

  useEffect(() => {
    if (acknowledgeSignal === prevAckRef.current) return
    prevAckRef.current = acknowledgeSignal
    setNodding(true)
    const t = setTimeout(() => setNodding(false), NOD_DURATION_MS)
    return () => clearTimeout(t)
  }, [acknowledgeSignal])

  const mouthOpenPx = speaking ? 6 + Math.max(0.15, Math.min(1, level)) * 20 : 6
  const palette = PALETTE[gender]
  const eyeRy = blinking ? 0.5 : 6

  return (
    <Box
      sx={{
        width: '100%', height: '100%', position: 'relative',
        display: 'flex', alignItems: 'center', justifyContent: 'center',
        opacity: active || pending ? 1 : 0.75,
        transition: 'opacity 0.3s ease',
      }}
    >
      <svg
        viewBox="0 0 200 200"
        aria-hidden="true"
        style={{
          width: '74%',
          height: '74%',
          transform: nodding ? 'translateY(4px) rotate(1.5deg)' : 'translateY(0) rotate(0deg)',
          transition: 'transform 0.18s ease-out',
        }}
      >
        {/* 肩・服 */}
        <path d="M30 200 Q100 150 170 200 L170 200 L30 200 Z" fill={palette.outfit} />
        {/* 後ろ髪 */}
        <circle cx="100" cy="92" r="62" fill={palette.hair} />
        {/* 顔 */}
        <circle cx="100" cy="100" r="54" fill={palette.skin} />
        {/* 前髪 */}
        <path d="M46 88 Q100 40 154 88 L154 68 Q100 28 46 68 Z" fill={palette.hair} />
        {/* 目 */}
        <ellipse cx="78" cy="98" rx="6" ry={eyeRy} fill="#2b2b2b" style={{ transition: 'ry 0.08s ease' }} />
        <ellipse cx="122" cy="98" rx="6" ry={eyeRy} fill="#2b2b2b" style={{ transition: 'ry 0.08s ease' }} />
        {/* 口 */}
        <rect
          x="84" y={131 - mouthOpenPx / 2} width="32" height={mouthOpenPx} rx="6"
          fill="#8a4a42"
          style={{ transition: speaking ? 'none' : 'height 0.15s ease, y 0.15s ease' }}
        />
      </svg>
    </Box>
  )
}
