/**
 * 実際にシップする GLB の幾何を見る。
 *
 * avatar-motion.test.ts / avatar-capabilities.test.ts は合成のリグに対しては
 * 正しかったが、実物のモデルとは一度も突き合わせていなかった。そのため
 * 「向き補正をジョイントの rotation に焼き込んだせいで head.rotation.x が
 * うなずき(ピッチ)ではなく首かしげ(ロール)になる」不具合を1件も捕まえられて
 * いなかった。head.rotation.x を足したときに実際の頂点がどちらへ動くかを見る。
 */
import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { readGlb, skinnedPositions, landmarks } from '@/scripts/measure-avatar.mjs'

/** AvatarMotion の nodDepthRad と同じ（6.3度）。 */
const NOD_RAD = 0.11

const MODELS = [
  ['male', 'public/avatars/male-avatar.glb'],
  ['female', 'public/avatars/female-avatar.glb'],
] as const

describe.each(MODELS)('%s-avatar.glb の幾何', (_name, rel) => {
  const path = join(process.cwd(), rel)
  let rest: Float64Array
  let nodded: Float64Array
  let mark: { crown: number; nose: number; height: number }

  beforeAll(() => {
    if (!existsSync(path)) throw new Error(`${rel} がありません`)
    const glb = readGlb(path)
    rest = skinnedPositions(glb)
    nodded = skinnedPositions(glb, { headRotX: NOD_RAD })
    mark = landmarks(rest)
  }, 120_000)

  const delta = (i: number) => ({
    x: nodded[i * 3] - rest[i * 3],
    y: nodded[i * 3 + 1] - rest[i * 3 + 1],
    z: nodded[i * 3 + 2] - rest[i * 3 + 2],
  })

  it('顔が +Z を向いている（ThreeAvatar は骨があると向き補正をかけない）', () => {
    expect(rest[mark.nose * 3 + 2]).toBeGreaterThan(mark.height * 0.1)
  })

  it('うなずくと鼻が下がって前へ出る', () => {
    const d = delta(mark.nose)
    expect(d.y).toBeLessThan(-mark.height * 0.005)
    expect(d.z).toBeGreaterThan(mark.height * 0.005)
  })

  it('うなずきで頭が横へ倒れない（首かしげとの取り違えを防ぐ）', () => {
    for (const i of [mark.nose, mark.crown]) {
      const d = delta(i)
      // 横(X)のずれが上下(Y)・前後(Z)に比べて無視できることを見る。
      // 向き補正がジョイントの rotation に入っていると、ここが主成分になる。
      expect(Math.abs(d.x)).toBeLessThan(Math.max(Math.abs(d.y), Math.abs(d.z)) * 0.05)
    }
  })

  it('うなずきで頭頂が前へ出る（真上や真後ろではない）', () => {
    expect(delta(mark.crown).z).toBeGreaterThan(0)
  })
})
