/**
 * 静止メッシュを面接官アバターとして置いたら気付けることを固定する。
 *
 * 実際に本番へ入っていた Tripo 製の GLB は skins 0 / morph 0 / ボーン無しで、
 * ThreeAvatar が口のモーフ検出と顎ボーンのフォールバックを実装していても
 * 全部空振りし、「うなずかない・口が動かない」になっていた。
 * 同じモデルを置いたときに検出できることをテストで留める。
 */
import * as THREE from 'three'
import {
  inspectAvatar,
  avatarDeficiencies,
  describeDeficiencies,
} from '@/lib/interview/avatar-capabilities'

/** 本番に入っていた Tripo モデルと同じ構造（1ノード・1メッシュ・骨も表情も無い）。 */
function staticMesh(): THREE.Object3D {
  const root = new THREE.Group()
  const mesh = new THREE.Mesh(
    new THREE.BoxGeometry(1, 1, 1),
    new THREE.MeshStandardMaterial(),
  )
  mesh.name = 'tripo_node_9c73ccb9'
  root.add(mesh)
  return root
}

/** VRM / Mixamo 相当のリグ済みモデル。 */
function riggedAvatar(opts: {
  headName?: string
  withBlink?: boolean
  withMouth?: boolean
  withJaw?: boolean
} = {}): THREE.Object3D {
  const { headName = 'Head', withBlink = true, withMouth = true, withJaw = false } = opts
  const root = new THREE.Group()
  const hips = new THREE.Bone(); hips.name = 'Hips'
  const neck = new THREE.Bone(); neck.name = 'Neck'
  const head = new THREE.Bone(); head.name = headName
  hips.add(neck); neck.add(head)
  if (withJaw) {
    const jaw = new THREE.Bone(); jaw.name = 'Jaw'
    head.add(jaw)
  }
  root.add(hips)

  const geom = new THREE.BoxGeometry(1, 1, 1)
  const dict: Record<string, number> = {}
  let i = 0
  if (withBlink) { dict['Blink_L'] = i++; dict['Blink_R'] = i++ }
  if (withMouth) { dict['aa'] = i++ }

  const skinned = new THREE.SkinnedMesh(geom, new THREE.MeshStandardMaterial())
  skinned.name = 'Face'
  if (i > 0) {
    skinned.morphTargetDictionary = dict
    skinned.morphTargetInfluences = new Array(i).fill(0)
  }
  skinned.bind(new THREE.Skeleton([hips, neck, head]))
  root.add(skinned)
  return root
}

describe('inspectAvatar / avatarDeficiencies', () => {
  it('静止メッシュは「動かせない」と判定する（本番に入っていたTripoモデル相当）', () => {
    const caps = inspectAvatar(staticMesh())
    expect(caps.skinned).toBe(false)
    expect(caps.headBone).toBeNull()
    expect(caps.jawBone).toBeNull()
    expect(caps.blinkTargets).toHaveLength(0)
    expect(caps.mouthTargets).toHaveLength(0)

    const d = avatarDeficiencies(caps)
    expect(d).toEqual(
      expect.arrayContaining(['no-skeleton', 'no-head-bone', 'no-mouth', 'no-blink']),
    )
    // 原因が1行で分かること。ここが空だと開発者が気付けない。
    expect(describeDeficiencies(d)).toContain('うなずけない')
  })

  it('リグ済みモデルは要件を満たすと判定する', () => {
    const caps = inspectAvatar(riggedAvatar())
    expect(caps.skinned).toBe(true)
    expect(caps.headBone?.name).toBe('Head')
    expect(caps.blinkTargets.length).toBeGreaterThan(0)
    expect(caps.mouthTargets.length).toBeGreaterThan(0)
    expect(avatarDeficiencies(caps)).toEqual([])
    expect(describeDeficiencies([])).toBe('')
  })

  const boneNames: Array<[string, string]> = [
    ['VRM / glTF humanoid', 'Head'],
    ['小文字', 'head'],
    ['Mixamo', 'mixamorig:Head'],
    ['日本語リグ', '頭'],
  ]
  it.each(boneNames)('%s の命名でも頭ボーンを見つける', (_label, name) => {
    const caps = inspectAvatar(riggedAvatar({ headName: name }))
    expect(caps.headBone).not.toBeNull()
    expect(avatarDeficiencies(caps)).not.toContain('no-head-bone')
  })

  it('頭ボーンは完全一致を優先する（HeadTop_End を頭と誤認しない）', () => {
    const root = new THREE.Group()
    const neck = new THREE.Bone(); neck.name = 'Neck'
    const tip = new THREE.Bone(); tip.name = 'HeadTop_End'
    const head = new THREE.Bone(); head.name = 'Head'
    // 部分一致する兄弟を先に走査させる
    neck.add(tip); neck.add(head)
    root.add(neck)
    const skinned = new THREE.SkinnedMesh(new THREE.BoxGeometry(), new THREE.MeshStandardMaterial())
    skinned.bind(new THREE.Skeleton([neck, tip, head]))
    root.add(skinned)

    expect(inspectAvatar(root).headBone?.name).toBe('Head')
  })

  it('口のモーフが無くても顎ボーンがあれば口は動かせる扱いにする', () => {
    const caps = inspectAvatar(riggedAvatar({ withMouth: false, withJaw: true }))
    expect(caps.mouthTargets).toHaveLength(0)
    expect(caps.jawBone?.name).toBe('Jaw')
    expect(avatarDeficiencies(caps)).not.toContain('no-mouth')
  })

  it('口のモーフも顎ボーンも無ければ口は動かせない', () => {
    const caps = inspectAvatar(riggedAvatar({ withMouth: false, withJaw: false }))
    expect(avatarDeficiencies(caps)).toContain('no-mouth')
  })
})
