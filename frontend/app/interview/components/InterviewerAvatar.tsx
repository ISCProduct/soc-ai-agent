'use client'

import { useState } from 'react'
import dynamic from 'next/dynamic'

import Avatar2D from './Avatar2D'

// Live2DAvatar は WebGL(window)前提のため、Avatar2D と違い ssr:false が必須。
const Live2DAvatar = dynamic(() => import('./Live2DAvatar'), { ssr: false })

// 資産（Cubism Core・モデル）が未配置の環境でも、毎セッション必ず404するリクエストを
// 試みない・SSR直後に一瞬アバターが空白になるのを避けるため、明示的なフラグで止める。
// 置いて試すときは .env.local で NEXT_PUBLIC_LIVE2D_ENABLED=true にする。
const LIVE2D_ENABLED = process.env.NEXT_PUBLIC_LIVE2D_ENABLED === 'true'

export interface InterviewerAvatarProps {
  gender: 'male' | 'female'
  level: number
  speaking: boolean
  active: boolean
  pending?: boolean
  acknowledgeSignal?: number
}

/**
 * 面接官アバターの描画エンジンを選ぶ入口。
 *
 * `NEXT_PUBLIC_LIVE2D_ENABLED=true` のときだけ Live2D を試す。Core/モデルが
 * 未配置、または置かれたモデルが面接官として要件（口・うなずき）を満たさない
 * 場合は `onUnavailable` が呼ばれ、現行の Avatar2D（2Dイラスト）へ自動で切り替わる。
 * フラグが無い（既定）ときは Avatar2D を直接返すので、本番の挙動・SSR結果は
 * このコンポーネントを入れる前と完全に同じになる。
 *
 * Live2D化を本採用するかどうかは別issueの判断（Avatar2D.tsx のコメント参照:
 * 3D(ThreeAvatar)は骨格・モーフを持たない静止メッシュ止まりで廃止した経緯がある。
 * Live2Dモデルを実際に用意できたときにどちらを残すか決める）。
 */
export default function InterviewerAvatar(props: InterviewerAvatarProps) {
  const [renderer, setRenderer] = useState<'live2d' | 'avatar2d'>(
    LIVE2D_ENABLED ? 'live2d' : 'avatar2d',
  )

  if (renderer === 'avatar2d') {
    return <Avatar2D {...props} />
  }
  return <Live2DAvatar {...props} onUnavailable={() => setRenderer('avatar2d')} />
}
