/**
 * @jest-environment jsdom
 */
/**
 * 面接官アバターの切り替え。Live2D が出せない環境でも面接を続けられること。
 *
 * `onUnavailable` は Live2DAvatar が自分で異常を見つけたときだけ呼ばれる。
 * next/dynamic のチャンク取得失敗やレンダー中の例外はそこを通らないので、
 * 境界が無いと app/error.tsx の ServiceErrorView が面接ページ全体を置き換える。
 * アバターが出ないだけなら面接は続けられるので、Avatar2D へ落ちるのが正しい。
 */
import type { ComponentType } from 'react'
import { act, render } from '@testing-library/react'
import InterviewerAvatar from '@/app/interview/components/InterviewerAvatar'

type Live2DProps = { onUnavailable?: () => void }

// 差し替え可能にしておき、テストごとに「正常 / 例外 / onUnavailable」を切り替える。
let live2dImpl: ComponentType<Live2DProps> = () => <div data-testid="live2d" />
let live2dCalls = 0

jest.mock('@/app/interview/components/Live2DAvatar', () => ({
  __esModule: true,
  default: (props: Live2DProps) => {
    live2dCalls += 1
    const Impl = live2dImpl
    return <Impl {...props} />
  },
}))

// Avatar2D が描画されたかは、Avatar2D.test.tsx と同じ目のセレクタで見る。
const EYE = '[cx="79"][cy="98"]'
const avatarProps = { gender: 'female' as const, level: 0, speaking: false, active: true }

async function renderAvatar() {
  let container!: HTMLElement
  await act(async () => {
    container = render(<InterviewerAvatar {...avatarProps} />).container
  })
  return container
}

describe('InterviewerAvatar', () => {
  const origFlag = process.env.NEXT_PUBLIC_LIVE2D_ENABLED
  let consoleError: jest.SpyInstance

  beforeEach(() => {
    live2dCalls = 0
    live2dImpl = () => <div data-testid="live2d" />
    // React は境界で捕まえた例外も console.error に出すため、出力を抑える。
    consoleError = jest.spyOn(console, 'error').mockImplementation(() => {})
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = 'true'
  })
  afterEach(() => {
    consoleError.mockRestore()
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = origFlag
  })

  it('フラグが無ければ Live2D を読み込まず Avatar2D を出す', async () => {
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = 'false'
    const container = await renderAvatar()
    expect(container.querySelector(EYE)).not.toBeNull()
    expect(live2dCalls).toBe(0)
  })

  it('Live2D が動く環境では Avatar2D を出さない（切り替えが常時発動しないこと）', async () => {
    const container = await renderAvatar()
    expect(container.querySelector('[data-testid="live2d"]')).not.toBeNull()
    expect(container.querySelector(EYE)).toBeNull()
  })

  it('Live2D の描画で例外が出たら Avatar2D へ切り替える', async () => {
    live2dImpl = () => {
      throw new Error('WebGL コンテキストを取得できない')
    }
    const container = await renderAvatar()
    expect(container.querySelector(EYE)).not.toBeNull()
  })

  it('チャンク取得失敗のように読み込み段階で投げても Avatar2D へ切り替える', async () => {
    live2dImpl = () => {
      throw new Error('ChunkLoadError: Loading chunk app/interview failed')
    }
    const container = await renderAvatar()
    expect(container.querySelector(EYE)).not.toBeNull()
  })

  it('Live2D が onUnavailable を呼んだら Avatar2D へ切り替える', async () => {
    live2dImpl = ({ onUnavailable }) => {
      onUnavailable?.()
      return <div data-testid="live2d" />
    }
    const container = await renderAvatar()
    expect(container.querySelector(EYE)).not.toBeNull()
    expect(container.querySelector('[data-testid="live2d"]')).toBeNull()
  })
})
