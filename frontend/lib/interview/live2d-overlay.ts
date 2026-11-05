/**
 * 差分画像(目閉じ・口開き)を重ねて表情を切り替えるリグ用の、不透明度の計算。
 *
 * 現在の書き出し済みモデルは、目閉じ・口開きの差分が独立した ArtMesh として
 * 入っているが、`ParamMouthOpenY` / `ParamEyeLOpen` などにキーフォームが
 * 結び付いていない（パラメータを振っても不透明度が変わらず、口開き差分が常時
 * 見えてしまう）。Cubism Editor で正しくキーを打ち直すまでの暫定対応として、
 * Coreが計算したdrawableの不透明度を、描画の直前にコード側で上書きする。
 *
 * 差分画像は全身1枚のフルサイズなので、重なる順は「口開き」が「目閉じ」より前。
 * まばたき中に口開きが残ると目閉じが隠れるため、まばたきを優先して口を消す。
 *
 * `base`（常時表示される通常絵のArtMesh）は、男性モデルではCore側の既定値が
 * 不透明度1になっているが、女性モデル（2026-10-29書き出し分）はこれが0で
 * 書き出されており、何も描画されない原因になっていた。毎フレーム1に上書きすることで、
 * Cubism側の既定値に関わらず常に見える状態にする（男性では既に1なので無害な上書き）。
 */

/** 書き出し済みモデルの差分ArtMesh。モデルを書き出し直したらIDを確認すること。 */
export const OVERLAY_MESH_IDS = {
  base: 'ArtMesh',
  eyesClosed: 'ArtMesh0',
  mouthOpen: 'ArtMesh1',
} as const

/** 女性モデルは男性モデルと目閉じ・口開きのIDが逆。 */
export const FEMALE_OVERLAY_MESH_IDS = {
  base: 'ArtMesh1',
  eyesClosed: 'ArtMesh',
  mouthOpen: 'ArtMesh0',
} as const

export type OverlayMeshIds = { base: string; eyesClosed: string; mouthOpen: string }

export interface OverlayOpacities {
  eyesClosed: number
  mouthOpen: number
}

const clamp01 = (v: number): number => (Number.isFinite(v) ? Math.min(1, Math.max(0, v)) : 0)

/**
 * 口の開き(0-1)を、口開き差分の不透明度へ変換する。
 * 差分は「閉じ」と「開き」の2枚しか無く、中間の値だと2枚が半透明に重なって
 * 口がぼやける。閉じ/開きをはっきり切り替え、重なるのはしきい値の間だけにする。
 */
const MOUTH_CLOSE_BELOW = 0.25
const MOUTH_OPEN_ABOVE = 0.5
function mouthCrossfade(mouthOpen: number): number {
  const t = clamp01((clamp01(mouthOpen) - MOUTH_CLOSE_BELOW) / (MOUTH_OPEN_ABOVE - MOUTH_CLOSE_BELOW))
  return t * t * (3 - 2 * t)
}

/**
 * @param eyeOpen 目の開き具合 0(閉じ)〜1(開き)
 * @param mouthOpen 口の開き具合 0(閉じ)〜1(開き)
 */
export function overlayOpacities(eyeOpen: number, mouthOpen: number): OverlayOpacities {
  const eyesClosed = 1 - clamp01(eyeOpen)
  return {
    eyesClosed,
    mouthOpen: mouthCrossfade(mouthOpen) * (1 - eyesClosed),
  }
}

/** 書き込める数値配列（Core の `drawables.opacities` / `dynamicFlags` は TypedArray）。 */
type MutableNumbers = { readonly length: number; [index: number]: number }

/** 差分ArtMeshの位置。見つからなかったものは null。 */
export interface OverlayTargets {
  base: number | null
  eyesClosed: number | null
  mouthOpen: number | null
}

/** Core が計算する「IsVisible」動的フラグのビット（Core の `hasIsVisibleBit` と同じ bit 0）。 */
const IS_VISIBLE_BIT = 1

/**
 * Core の drawable 配列を書き換えて、差分ArtMesh方式のモデルの目・口を動かす。
 *
 * `opacities` / `dynamicFlags` は描画側(renderer)が読むのと同じ配列。`update()` のたびに
 * Core が計算し直すので、毎フレームその直後に呼ぶ。
 *
 * 不透明度だけでは足りない。Core が計算する IsVisible 動的フラグが false だと、
 * レンダラーは不透明度を見ずにその drawable の描画自体をスキップする
 * （`cubismrenderer_webgl.ts` が `getDrawableDynamicFlagIsVisible` で弾き、判定は
 * Core の `hasIsVisibleBit`）。そのため不透明度とフラグを必ず揃える。
 *
 * `base`（常時表示の通常絵）は、書き出しによって Core 側の既定値が不透明度0・不可視に
 * なっている（女性モデルで確認。これが原因で画面に何も出なかった）。差分の有無とは
 * 無関係に常に見える状態へ固定する（男性モデルでは既にそうなので上書きしても無害）。
 */
export function applyOverlayToDrawables(
  opacities: MutableNumbers,
  dynamicFlags: MutableNumbers,
  targets: OverlayTargets,
  eyeOpen: number,
  mouthOpen: number,
): void {
  const opacity = overlayOpacities(eyeOpen, mouthOpen)
  for (const [key, value] of [
    ['eyesClosed', opacity.eyesClosed],
    ['mouthOpen', opacity.mouthOpen],
  ] as const) {
    const index = targets[key]
    if (index === null) continue
    opacities[index] = value
    dynamicFlags[index] =
      value > 0 ? dynamicFlags[index] | IS_VISIBLE_BIT : dynamicFlags[index] & ~IS_VISIBLE_BIT
  }
  if (targets.base !== null) {
    opacities[targets.base] = 1
    dynamicFlags[targets.base] |= IS_VISIBLE_BIT
  }
}
