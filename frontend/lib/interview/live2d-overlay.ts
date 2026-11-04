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
 */

/** 書き出し済み男性モデルの差分ArtMesh。モデルを書き出し直したらIDを確認すること。 */
export const OVERLAY_MESH_IDS = {
  eyesClosed: 'ArtMesh0',
  mouthOpen: 'ArtMesh1',
} as const

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
