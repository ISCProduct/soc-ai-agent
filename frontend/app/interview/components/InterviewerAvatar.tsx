'use client'

import { Component, useState, type ReactNode } from 'react'
import dynamic from 'next/dynamic'

import Avatar2D from './Avatar2D'

// Live2DAvatar は WebGL(window)前提のため、Avatar2D と違い ssr:false が必須。
const Live2DAvatar = dynamic(() => import('./Live2DAvatar'), { ssr: false })

// 資産（Cubism Core・モデル）が未配置の環境でも、毎セッション必ず404するリクエストを
// 試みない・SSR直後に一瞬アバターが空白になるのを避けるため、明示的なフラグで止める。
// 置いて試すときは .env.local で NEXT_PUBLIC_LIVE2D_ENABLED=true にする。
//
// 読み取りを関数にしているのはモジュール読み込み時の値で固定しないため。
// Next は NEXT_PUBLIC_* をビルド時に埋めるので、本番の結果は定数と同じ。
function live2dEnabled(): boolean {
  return process.env.NEXT_PUBLIC_LIVE2D_ENABLED === 'true'
}

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
/**
 * Live2D の読み込み・描画で投げられた例外をここで止め、Avatar2D へ切り替える。
 *
 * `onUnavailable` は Live2DAvatar が自分で異常を見つけたときだけ呼ばれる。
 * next/dynamic のチャンク取得失敗やレンダー中の例外はそこを通らず、境界が無いと
 * app/error.tsx の ServiceErrorView が面接ページ全体を置き換えてしまう。
 * アバターが出ないだけなら面接は続けられるので、ここで受け止める。
 */
class Live2DBoundary extends Component<{ onError: () => void; children: ReactNode }, { failed: boolean }> {
  state = { failed: false }

  static getDerivedStateFromError() {
    return { failed: true }
  }

  componentDidCatch(error: unknown) {
    // 切り替わった理由を追えるように残す（面接自体は止めない）。
    console.error('[InterviewerAvatar] Live2D の描画に失敗したため Avatar2D に切り替えます', error)
    this.props.onError()
  }

  render() {
    // 失敗したフレームで壊れた子を再描画しない。親が avatar2d へ切り替えるとこの境界は外れる。
    return this.state.failed ? null : this.props.children
  }
}

export default function InterviewerAvatar(props: InterviewerAvatarProps) {
  const [renderer, setRenderer] = useState<'live2d' | 'avatar2d'>(
    live2dEnabled() ? 'live2d' : 'avatar2d',
  )

  if (renderer === 'avatar2d') {
    return <Avatar2D {...props} />
  }
  const fallback = () => setRenderer('avatar2d')
  return (
    <Live2DBoundary onError={fallback}>
      <Live2DAvatar {...props} onUnavailable={fallback} />
    </Live2DBoundary>
  )
}
