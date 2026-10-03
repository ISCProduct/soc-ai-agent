/**
 * three-vrm と avatar-capabilities の間の変換。
 *
 * VRM を直接 capabilities に持ち込まないのは、three-vrm への依存を
 * アバターの判定ロジックから切り離してテストしやすくするため。
 */
import type { Bone, Object3D } from 'three'
import type { VrmExpressionSetter } from './avatar-capabilities'

/** three-vrm の VRM のうち、ここで使う部分だけ。 */
interface VrmLike {
  scene: Object3D
  humanoid?: {
    getNormalizedBoneNode(name: string): Object3D | null
  } | null
  expressionManager?: {
    getExpression(name: string): unknown
    setValue(name: string, weight: number): void
  } | null
  update?(delta: number): void
}

/** VRM 1.0 / 0.x の表情名。0.x は大文字（Blink / A）で来ることがある。 */
const BLINK_NAMES = ['blink', 'Blink']
const MOUTH_NAMES = ['aa', 'A']

function firstAvailable(
  mgr: NonNullable<VrmLike['expressionManager']>,
  names: string[],
): string | null {
  for (const n of names) {
    if (mgr.getExpression(n)) return n
  }
  return null
}

/** VRM から表情の操作口を作る。使える表情が1つも無ければ null。 */
export function vrmExpressionSetter(vrm: VrmLike): VrmExpressionSetter | null {
  const mgr = vrm.expressionManager
  if (!mgr) return null
  const blink = firstAvailable(mgr, BLINK_NAMES)
  const mouth = firstAvailable(mgr, MOUTH_NAMES)
  if (!blink && !mouth) return null
  return {
    hasBlink: blink !== null,
    hasMouth: mouth !== null,
    setBlink(value) { if (blink) mgr.setValue(blink, value) },
    setMouthOpen(value) { if (mouth) mgr.setValue(mouth, value) },
  }
}

/**
 * VRM から頭のボーンを取る。
 *
 * VRM は humanoid が規格名からボーンを引けるので、名前のパターン照合より確実。
 * VRoid の出力は 'J_Bip_C_Head' のような内部名で、照合に頼ると実装依存になる。
 * 頭が無いモデルは首で代替する。
 */
export function vrmHeadBone(vrm: VrmLike): Bone | null {
  const h = vrm.humanoid
  if (!h) return null
  const node = h.getNormalizedBoneNode('head') ?? h.getNormalizedBoneNode('neck')
  return (node as Bone | null) ?? null
}

/** 読み込んだ GLTF から VRM を取り出す。VRM でなければ null。 */
export function extractVrm(gltf: { userData?: Record<string, unknown> }): VrmLike | null {
  const vrm = gltf.userData?.vrm
  if (!vrm || typeof vrm !== 'object') return null
  return vrm as VrmLike
}
