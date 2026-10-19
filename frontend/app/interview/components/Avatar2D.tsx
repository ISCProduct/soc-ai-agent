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
const SKIN_BASE = '#e7b48c'
const SKIN_HILITE = '#f6d2ae'
const SKIN_SHADOW = '#c88f66'

const PALETTE = {
  male: {
    hairBase: '#2a2320', hairHilite: '#4a3c35',
    outfitBase: '#24344a', outfitHilite: '#3a5272',
    shirt: '#e8edf2', tie: '#38bdf8',
  },
  female: {
    hairBase: '#3c2420', hairHilite: '#5e3a30',
    outfitBase: '#4a2d3f', outfitHilite: '#6b4259',
    shirt: '#f2e9ec', tie: '#f0c9d6',
  },
} as const

/**
 * 2Dイラストの面接官アバター。3D(ThreeAvatar)からの移行(SOCAIAGENT-2D)。
 *
 * 実運用の3Dモデル(male/female-avatar.glb)は骨格・モーフターゲットを
 * 持たない静止メッシュで、うなずき・口パクが実質機能していなかった
 * (avatar-capabilities.tsのコメント参照)。素直な図形の差分アニメーションで
 * 同等以上の「喋っている感」を、three.js/GLTF依存無しで出す。
 *
 * グラデーション・陰影を使ったフラットイラスト調。単色の円だけで描いていた
 * 初版は見た目が安っぽかったため描き直した（ユーザー指摘）。
 */
export default function Avatar2D({ gender, level, speaking, active, pending = false, acknowledgeSignal = 0 }: Avatar2DProps) {
  const [blinking, setBlinking] = useState(false)
  const [nodding, setNodding] = useState(false)
  const prevAckRef = useRef(acknowledgeSignal)
  const uid = useRef(`av2d-${Math.random().toString(36).slice(2, 9)}`).current

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

  const mouthOpen = speaking ? Math.max(0.15, Math.min(1, level)) : 0
  const mouthOpenPx = 3 + mouthOpen * 16
  const palette = PALETTE[gender]
  const eyeRy = blinking ? 0.6 : 7

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
        viewBox="0 0 200 220"
        aria-hidden="true"
        style={{
          width: '82%',
          height: '82%',
          transform: nodding ? 'translateY(3px) rotate(1.2deg)' : 'translateY(0) rotate(0deg)',
          transition: 'transform 0.18s ease-out',
        }}
      >
        <defs>
          <radialGradient id={`${uid}-glow`} cx="50%" cy="38%" r="60%">
            <stop offset="0%" stopColor="#38bdf8" stopOpacity="0.16" />
            <stop offset="100%" stopColor="#38bdf8" stopOpacity="0" />
          </radialGradient>
          <radialGradient id={`${uid}-skin`} cx="42%" cy="34%" r="65%">
            <stop offset="0%" stopColor={SKIN_HILITE} />
            <stop offset="65%" stopColor={SKIN_BASE} />
            <stop offset="100%" stopColor={SKIN_SHADOW} />
          </radialGradient>
          <linearGradient id={`${uid}-hair`} x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor={palette.hairHilite} />
            <stop offset="100%" stopColor={palette.hairBase} />
          </linearGradient>
          <linearGradient id={`${uid}-outfit`} x1="0" y1="0" x2="1" y2="1">
            <stop offset="0%" stopColor={palette.outfitHilite} />
            <stop offset="100%" stopColor={palette.outfitBase} />
          </linearGradient>
        </defs>

        {/* 背景の淡いグロー(テーマのアクセント色と統一感を出す) */}
        <circle cx="100" cy="95" r="98" fill={`url(#${uid}-glow)`} />

        {/* 肩・ブレザー */}
        <path d="M18 220 Q24 158 72 144 L100 160 L128 144 Q176 158 182 220 Z" fill={`url(#${uid}-outfit)`} />
        {/* シャツ襟元 */}
        <path d="M86 150 L100 175 L114 150 L100 158 Z" fill={palette.shirt} />
        {/* ネクタイ/スカーフのアクセント */}
        <path d="M96 160 L104 160 L101 182 L100 188 L99 182 Z" fill={palette.tie} />
        {/* ラペル(襟) */}
        <path d="M72 144 L100 160 L92 172 L58 150 Z" fill={palette.outfitBase} opacity="0.9" />
        <path d="M128 144 L100 160 L108 172 L142 150 Z" fill={palette.outfitBase} opacity="0.9" />

        {/* 後ろ髪(女性は肩までの長さ、男性は短髪)。顔(skin楕円)を後で重ねて描き、
            はみ出た部分だけを髪として見せる(前髪・頬周りの輪郭を顔側で作る)。 */}
        {gender === 'female' ? (
          <path d="M38 108 Q34 48 100 38 Q166 48 162 108 Q166 150 148 168 L142 120 Q142 90 100 86 Q58 90 58 120 L52 168 Q34 150 38 108 Z" fill={`url(#${uid}-hair)`} />
        ) : (
          <ellipse cx="100" cy="82" rx="60" ry="62" fill={`url(#${uid}-hair)`} />
        )}

        {/* 首 */}
        <rect x="88" y="128" width="24" height="26" rx="8" fill={SKIN_SHADOW} />
        <rect x="88" y="128" width="24" height="16" rx="8" fill={SKIN_BASE} />

        {/* 耳 */}
        <ellipse cx="54" cy="104" rx="6" ry="9" fill={SKIN_BASE} />
        <ellipse cx="146" cy="104" rx="6" ry="9" fill={SKIN_BASE} />

        {/* 顔 */}
        <ellipse cx="100" cy="100" rx="46" ry="50" fill={`url(#${uid}-skin)`} />
        {/* 頬の赤み */}
        <ellipse cx="72" cy="114" rx="9" ry="6" fill="#e8847a" opacity="0.18" />
        <ellipse cx="128" cy="114" rx="9" ry="6" fill="#e8847a" opacity="0.18" />

        {/* 前髪 */}
        <path
          d={gender === 'female'
            ? 'M54 82 Q58 44 100 40 Q142 44 146 82 Q136 58 100 56 Q64 58 54 82 Z'
            : 'M56 80 Q58 42 100 38 Q142 42 144 80 Q130 56 100 52 Q70 56 56 80 Z'}
          fill={`url(#${uid}-hair)`}
        />
        {gender === 'female' && (
          <path d="M50 92 Q44 120 52 150 Q60 128 58 96 Z" fill={`url(#${uid}-hair)`} />
        )}
        {gender === 'female' && (
          <path d="M150 92 Q156 120 148 150 Q140 128 142 96 Z" fill={`url(#${uid}-hair)`} />
        )}

        {/* 眉 */}
        <rect x="70" y="80" width="18" height="4" rx="2" fill={palette.hairBase} />
        <rect x="112" y="80" width="18" height="4" rx="2" fill={palette.hairBase} />

        {/* 目 */}
        <ellipse cx="79" cy="98" rx="7" ry={eyeRy} fill="#2b2320" style={{ transition: 'ry 0.08s ease' }} />
        <ellipse cx="121" cy="98" rx="7" ry={eyeRy} fill="#2b2320" style={{ transition: 'ry 0.08s ease' }} />
        {!blinking && (
          <>
            <circle cx="81.5" cy="95.5" r="1.6" fill="#fff" opacity="0.85" />
            <circle cx="123.5" cy="95.5" r="1.6" fill="#fff" opacity="0.85" />
          </>
        )}

        {/* 鼻(控えめ) */}
        <path d="M100 104 Q102 112 100 115 Q97 115 98 112" fill="none" stroke={SKIN_SHADOW} strokeWidth="1.5" strokeLinecap="round" opacity="0.6" />

        {/* 口: 待機中は柔らかい笑み、発話中は音量に応じて開く */}
        {speaking ? (
          <ellipse cx="100" cy="130" rx="11" ry={mouthOpenPx} fill="#7a3b36" style={{ transition: 'none' }} />
        ) : (
          <path d="M88 129 Q100 137 112 129" fill="none" stroke="#8a4a42" strokeWidth="3" strokeLinecap="round" />
        )}
      </svg>
    </Box>
  )
}
