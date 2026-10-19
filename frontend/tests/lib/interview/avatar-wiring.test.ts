/**
 * ThreeAvatar の結線判断（#1646）。ThreeAvatar はこの関数を通して状態と口の経路を決める。
 */
import * as THREE from 'three'
import { inspectAvatar } from '@/lib/interview/avatar-capabilities'
import { capsOwnsMouth, interviewerStateFor, legacyMouthPaths } from '@/lib/interview/avatar-wiring'

function model(opts: { mouth?: boolean; jaw?: boolean }) {
  const root = new THREE.Group()
  const head = new THREE.Bone(); head.name = 'Head'
  const bones = [head]
  if (opts.jaw) {
    const jaw = new THREE.Bone(); jaw.name = 'Jaw'
    head.add(jaw); bones.push(jaw)
  }
  root.add(head)
  const mesh = new THREE.SkinnedMesh(new THREE.BoxGeometry(), new THREE.MeshStandardMaterial())
  mesh.morphTargetDictionary = opts.mouth ? { aa: 0 } : {}
  mesh.morphTargetInfluences = opts.mouth ? [0] : []
  mesh.bind(new THREE.Skeleton(bones))
  root.add(mesh)
  return root
}

describe('interviewerStateFor', () => {
  it.each([
    [{ speaking: true, active: true, pending: true }, 'speaking'],
    [{ speaking: true, active: false, pending: false }, 'speaking'],
    [{ speaking: false, active: false, pending: true }, 'idle'],
    [{ speaking: false, active: true, pending: true }, 'thinking'],
    [{ speaking: false, active: true, pending: false }, 'listening'],
  ] as const)('%o → %s', (input, want) => {
    expect(interviewerStateFor(input)).toBe(want)
  })
})

describe('口の3経路の排他', () => {
  const all = { morph: true, jaw: true, viseme: true }

  it('caps が口モーフを持つなら旧経路はすべて止める', () => {
    const caps = inspectAvatar(model({ mouth: true }))
    expect(capsOwnsMouth(caps)).toBe(true)
    expect(legacyMouthPaths(caps, all)).toEqual({ morph: false, jaw: false, viseme: false })
  })

  it('caps が顎ボーンだけ持つときも旧経路は止める', () => {
    const caps = inspectAvatar(model({ jaw: true }))
    expect(capsOwnsMouth(caps)).toBe(true)
    expect(legacyMouthPaths(caps, all)).toEqual({ morph: false, jaw: false, viseme: false })
  })

  it('caps が口を持たないなら旧経路のモーフ・顎を使い、ビセームは使わない', () => {
    const caps = inspectAvatar(model({}))
    expect(capsOwnsMouth(caps)).toBe(false)
    expect(legacyMouthPaths(caps, all)).toEqual({ morph: true, jaw: true, viseme: false })
  })

  it('モーフも顎も無いときだけビセームに落ちる', () => {
    expect(legacyMouthPaths(null, { morph: false, jaw: false, viseme: true }))
      .toEqual({ morph: false, jaw: false, viseme: true })
  })
})
