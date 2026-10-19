/**
 * @jest-environment jsdom
 */
/**
 * ThreeAvatar の結線（#1646）。学生の発話の区切りが来たら AvatarMotion.acknowledge を呼ぶ。
 * WebGL だけ差し替え、モデル読込・AvatarMotion は本番と同じものを通す。
 */
import { render, screen, waitFor } from '@testing-library/react'
import * as THREE from 'three'
import ThreeAvatar from '@/app/interview/components/ThreeAvatar'
import { AvatarMotion } from '@/lib/interview/avatar-motion'

const setSizeSpy = jest.fn()

jest.mock('three', () => {
  const actual = jest.requireActual('three')
  class FakeRenderer {
    domElement = document.createElement('canvas')
    shadowMap = { enabled: false, type: 0 }
    outputColorSpace = ''
    setPixelRatio() {}
    setSize(...args: unknown[]) { setSizeSpy(...args) }
    render() {}
    dispose() {}
  }
  return { ...actual, WebGLRenderer: FakeRenderer }
})

// ESM のままで jest が変換できない。クローンは検証対象ではないのでそのまま返す。
jest.mock('three/examples/jsm/utils/SkeletonUtils.js', () => ({ clone: <T,>(o: T) => o }))

jest.mock('@/lib/avatar-loader', () => ({
  loadAvatar: jest.fn(async () => {
    const root = new THREE.Group()
    const head = new THREE.Bone(); head.name = 'Head'
    root.add(head)
    // SkinnedMesh は skinIndex 属性が無いと境界計算で落ちるので、通常の Mesh で口モーフだけ持たせる
    const mesh = new THREE.Mesh(new THREE.BoxGeometry(), new THREE.MeshStandardMaterial())
    mesh.morphTargetDictionary = { aa: 0 }
    mesh.morphTargetInfluences = [0]
    root.add(mesh)
    return { scene: root }
  }),
}))

describe('ThreeAvatar', () => {
  it('acknowledgeSignal が増えるたびに相槌を1回打つ', async () => {
    const ack = jest.spyOn(AvatarMotion.prototype, 'acknowledge')
    const props = { gender: 'male' as const, audioStream: null, level: 0, speaking: false, active: true }
    const { rerender } = render(<ThreeAvatar {...props} acknowledgeSignal={0} />)
    await waitFor(() => expect(screen.queryByText('アバター読込中...')).toBeNull())
    expect(ack).not.toHaveBeenCalled()

    rerender(<ThreeAvatar {...props} acknowledgeSignal={1} />)
    expect(ack).toHaveBeenCalledTimes(1)
    rerender(<ThreeAvatar {...props} acknowledgeSignal={2} />)
    expect(ack).toHaveBeenCalledTimes(2)
    // 他の props が変わっただけでは打たない
    rerender(<ThreeAvatar {...props} level={0.5} acknowledgeSignal={2} />)
    expect(ack).toHaveBeenCalledTimes(2)
  })

  // パネル全面表示化(#1569)でWebGL描画バッファが肥大化しないよう、
  // 内部解像度には上限(420px)を設けている（レビュー指摘）。
  it('パネルが大きい場合は内部解像度を420pxに上限を設けて描画する', async () => {
    jest.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(873)
    jest.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(776)

    const props = { gender: 'male' as const, audioStream: null, level: 0, speaking: false, active: true }
    render(<ThreeAvatar {...props} />)
    await waitFor(() => expect(screen.queryByText('アバター読込中...')).toBeNull())

    // 第3引数 false = canvasのCSSサイズは変えず、内部解像度(描画バッファ)だけをcapする
    expect(setSizeSpy).toHaveBeenCalledWith(420, 373, false)

    jest.restoreAllMocks()
  })

  // renderer.setSizeの実バッファはpixelRatio倍されるため、devicePixelRatio=2の
  // 環境では論理サイズ自体を420/2=210pxに抑えないと実バッファが420pxを超える
  // （CodeRabbit指摘）。
  it('devicePixelRatioが2の場合は論理サイズをさらに半分に抑える', async () => {
    jest.spyOn(HTMLElement.prototype, 'clientWidth', 'get').mockReturnValue(873)
    jest.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(776)
    const originalDpr = window.devicePixelRatio
    Object.defineProperty(window, 'devicePixelRatio', { value: 2, configurable: true })

    try {
      const props = { gender: 'male' as const, audioStream: null, level: 0, speaking: false, active: true }
      render(<ThreeAvatar {...props} />)
      await waitFor(() => expect(screen.queryByText('アバター読込中...')).toBeNull())

      // 210(論理) x pixelRatio 2 = 実バッファ420px。373の半分(187)も同様。
      expect(setSizeSpy).toHaveBeenCalledWith(210, 187, false)
    } finally {
      Object.defineProperty(window, 'devicePixelRatio', { value: originalDpr, configurable: true })
      jest.restoreAllMocks()
    }
  })
})
