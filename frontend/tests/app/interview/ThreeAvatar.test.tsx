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

jest.mock('three', () => {
  const actual = jest.requireActual('three')
  class FakeRenderer {
    domElement = document.createElement('canvas')
    shadowMap = { enabled: false, type: 0 }
    outputColorSpace = ''
    setPixelRatio() {}
    setSize() {}
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
})
