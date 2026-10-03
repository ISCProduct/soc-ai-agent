/**
 * うなずき・まばたき・口が実際に動くことを固定する。
 *
 * 「動かない」が本番で起きていた機能なので、動くことをテストで留める。
 * 乱数は固定値を注入して決定的にする。
 */
import * as THREE from 'three'
import { AvatarMotion, DEFAULT_MOTION, nodCurve, pulse } from '@/lib/interview/avatar-motion'
import { inspectAvatar } from '@/lib/interview/avatar-capabilities'

function rigged(opts: { withJaw?: boolean; withMouth?: boolean } = {}) {
  const { withJaw = false, withMouth = true } = opts
  const root = new THREE.Group()
  const neck = new THREE.Bone(); neck.name = 'Neck'
  const head = new THREE.Bone(); head.name = 'Head'
  neck.add(head)
  const bones = [neck, head]
  if (withJaw) {
    const jaw = new THREE.Bone(); jaw.name = 'Jaw'
    head.add(jaw); bones.push(jaw)
  }
  root.add(neck)
  const mesh = new THREE.SkinnedMesh(new THREE.BoxGeometry(), new THREE.MeshStandardMaterial())
  const dict: Record<string, number> = { Blink_L: 0, Blink_R: 1 }
  let n = 2
  if (withMouth) dict['aa'] = n++
  mesh.morphTargetDictionary = dict
  mesh.morphTargetInfluences = new Array(n).fill(0)
  mesh.bind(new THREE.Skeleton(bones))
  root.add(mesh)
  return root
}

/** 乱数は常に中央値。間隔が区間の真ん中に固定される。 */
const mid = () => 0.5

describe('pulse / nodCurve', () => {
  it('両端は0で、途中は正の値になる', () => {
    for (const f of [pulse, nodCurve]) {
      expect(f(0)).toBe(0)
      expect(f(1)).toBe(0)
      expect(f(0.5)).toBeGreaterThan(0)
    }
  })

  it('うなずきは前半で最も深くなる（左右対称でない）', () => {
    // 機械的に見えないよう下げを速く戻りを遅くしている
    expect(nodCurve(0.35)).toBeGreaterThan(nodCurve(0.65))
  })
})

describe('AvatarMotion', () => {
  it('listening に入ると頭が動く（うなずく）', () => {
    const root = rigged()
    const caps = inspectAvatar(root)
    const rest = caps.headBone!.rotation.x
    const m = new AvatarMotion(DEFAULT_MOTION, mid)

    m.setState('listening', 0)
    // うなずきの山の途中を見る
    m.update(caps, DEFAULT_MOTION.nodDurationSec * 0.35, 0)
    const nodded = caps.headBone!.rotation.x

    expect(Math.abs(nodded - rest)).toBeGreaterThan(DEFAULT_MOTION.nodDepthRad * 0.5)
  })

  it('speaking 中はうなずかない', () => {
    const caps = inspectAvatar(rigged())
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('speaking', 0)

    const samples: number[] = []
    for (let t = 0; t < 5; t += 0.05) {
      m.update(caps, t, 0)
      samples.push(caps.headBone!.rotation.x)
    }
    // 呼吸の微動（±0.012）だけで、うなずきの深さには達しない
    const span = Math.max(...samples) - Math.min(...samples)
    expect(span).toBeLessThan(DEFAULT_MOTION.nodDepthRad * 0.5)
  })

  it('聞いている間は繰り返しうなずく', () => {
    const caps = inspectAvatar(rigged())
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('listening', 0)

    let peaks = 0
    let prev = 0
    let rising = false
    for (let t = 0; t < 20; t += 0.02) {
      m.update(caps, t, 0)
      const v = Math.abs(caps.headBone!.rotation.x)
      if (v > prev + 1e-6) rising = true
      else if (rising && v < prev - 1e-6) { peaks++; rising = false }
      prev = v
    }
    // 20秒で、間隔の中央値（約3秒）どおりなら複数回出る
    expect(peaks).toBeGreaterThanOrEqual(3)
  })

  it('acknowledge で相槌を1回入れられる', () => {
    const caps = inspectAvatar(rigged())
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('listening', 0)
    // 最初のうなずきを終わらせる
    m.update(caps, DEFAULT_MOTION.nodDurationSec + 0.01, 0)
    const rest = caps.headBone!.rotation.x

    m.acknowledge(10)
    m.update(caps, 10 + DEFAULT_MOTION.nodDurationSec * 0.35, 0)
    expect(Math.abs(caps.headBone!.rotation.x - rest)).toBeGreaterThan(DEFAULT_MOTION.nodDepthRad * 0.5)
  })

  it('まばたきする', () => {
    const caps = inspectAvatar(rigged())
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('idle', 0)

    let maxBlink = 0
    for (let t = 0; t < 15; t += 0.01) {
      m.update(caps, t, 0)
      maxBlink = Math.max(maxBlink, caps.blinkTargets[0].mesh.morphTargetInfluences![0])
    }
    expect(maxBlink).toBeGreaterThan(0.9)
  })

  it('speaking のとき口が音声振幅で開く', () => {
    const caps = inspectAvatar(rigged())
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    const idx = caps.mouthTargets[0].index

    m.setState('speaking', 0)
    // 補間するので1フレームでは到達しない。十分な回数回して近づくことを見る。
    for (let i = 0; i < 80; i++) m.update(caps, 0.1 + i * 0.016, 0.8)
    expect(caps.mouthTargets[0].mesh.morphTargetInfluences![idx]).toBeCloseTo(0.8, 2)

    // 聞いているときは閉じ切る（学生の声で口が動いたら不自然）
    m.setState('listening', 2)
    for (let i = 0; i < 120; i++) m.update(caps, 2 + i * 0.016, 0.8)
    expect(caps.mouthTargets[0].mesh.morphTargetInfluences![idx]).toBe(0)
  })

  it('口のモーフが無いモデルでは顎ボーンで代替する', () => {
    const caps = inspectAvatar(rigged({ withMouth: false, withJaw: true }))
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('speaking', 0)
    for (let i = 0; i < 40; i++) m.update(caps, 0.1 + i * 0.016, 1.0)
    expect(caps.jawBone!.rotation.x).toBeGreaterThan(0)
  })

  it('部位が無いモデルでも例外を投げない（面接を止めない）', () => {
    const staticRoot = new THREE.Group()
    staticRoot.add(new THREE.Mesh(new THREE.BoxGeometry(), new THREE.MeshStandardMaterial()))
    const caps = inspectAvatar(staticRoot)
    const m = new AvatarMotion(DEFAULT_MOTION, mid)
    m.setState('listening', 0)
    expect(() => {
      for (let t = 0; t < 3; t += 0.1) m.update(caps, t, 0.5)
    }).not.toThrow()
  })
})
