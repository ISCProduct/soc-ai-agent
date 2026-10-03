/**
 * アバターモデルが「動ける」かを判定する。
 *
 * 面接官がうなずかない・口が動かない原因は、ほぼ必ずコードではなくモデル側にある。
 * 実際に本番へ入っていた male-avatar.glb / female-avatar.glb は Tripo（画像から3Dを
 * 生成するAIツール）製の静止メッシュで、次の状態だった。
 *
 *   skins: 0                                  骨格が無い
 *   animations: 0
 *   morph targets: 0                          表情ブレンドシェイプが無い
 *   attributes: POSITION, NORMAL, TEXCOORD_0  JOINTS_0 / WEIGHTS_0 が無い
 *
 * 首の関節も頂点ウェイトも無いので、どんなコードを書いてもうなずかない。
 * ThreeAvatar 側は口のモーフ検出も顎ボーンのフォールバックも実装済みだが、
 * モデルがどれも提供しないため全部空振りしていた。
 *
 * そこで「置いたモデルが要件を満たすか」を実行時とテストの両方で見る。
 * 静止メッシュを置いたら気付けるようにするのが目的で、描画は止めない
 * （面接を止めるより、動かないアバターでも面接を続けるほうがよい）。
 */
import type { Object3D, Mesh, Bone } from 'three'

/** アバターに期待する動作。できないものは理由つきで分かるようにする。 */
export interface AvatarCapabilities {
  /** 頭または首のボーン。これが無いとうなずけない。 */
  headBone: Bone | null
  /** まばたきに使えるモーフを持つメッシュ。 */
  blinkTargets: MorphRef[]
  /** 口の開閉に使えるモーフ。 */
  mouthTargets: MorphRef[]
  /** 顎ボーン。モーフが無いモデルでの口の代替。 */
  jawBone: Bone | null
  /** スキニングされているか（skins があるか）。 */
  skinned: boolean
  /**
   * VRM の表情API。VRM では表情が morphTargetDictionary の名前と1対1でなく、
   * expressionManager 経由で指定する（VRoid の出力は 'Fcl_ALL_Blink' のような
   * 内部名で、規格上の名前は 'blink' / 'aa'）。あるなら生のモーフより優先する。
   */
  vrmExpression: VrmExpressionSetter | null
}

/** VRM の expressionManager への最小のアクセス。three-vrm に依存させない。 */
export interface VrmExpressionSetter {
  setBlink(value: number): void
  setMouthOpen(value: number): void
  /** まばたき表情を持っているか。 */
  hasBlink: boolean
  /** 口の表情を持っているか。 */
  hasMouth: boolean
}

export interface MorphRef {
  mesh: Mesh
  index: number
  name: string
}

/** できないことの一覧。空なら要件を満たしている。 */
export type AvatarDeficiency =
  | 'no-skeleton'
  | 'no-head-bone'
  | 'no-blink'
  | 'no-mouth'

const HEAD_BONE_PATTERNS = [
  // VRM / glTF humanoid の標準名
  'head', 'neck',
  // Mixamo
  'mixamorighead', 'mixamorigneck',
  // 日本語リグ
  '頭', '首',
]

const JAW_BONE_PATTERNS = ['jaw', '顎', 'あご']

const BLINK_PATTERNS = [
  // VRM 1.0 / 0.x
  'blink', 'blink_l', 'blink_r', 'blinkleft', 'blinkright',
  // ARKit / Oculus visemes 併設モデル
  'eyeblinkleft', 'eyeblinkright', 'eyesclosed',
  // 日本語
  'まばたき', '目_閉じ',
]

const MOUTH_PATTERNS = [
  'mouth_open', 'mouthopen', 'jaw_open', 'jawopen',
  'aa', 'mouth_a', 'moutha', 'viseme_aa',
  'あ', '口_あ',
]

function norm(s: string): string {
  return s.toLowerCase().replace(/[\s_.\-:]/g, '')
}

function findBone(root: Object3D, patterns: string[]): Bone | null {
  // パターンの並び順が優先順位。'head' を 'neck' より先に書いているので、
  // 両方ある VRM では頭が選ばれる。走査順（シーングラフの並び）に
  // 依存させると、同じモデルでも出力側の都合で結果が変わる。
  const bones: Bone[] = []
  root.traverse((o) => {
    // three の Bone は isBone を持つ。instanceof は three の多重読み込みで壊れるので使わない。
    if ((o as Bone).isBone && o.name) bones.push(o as Bone)
  })
  for (const pattern of patterns.map(norm)) {
    // 同じパターンの中では完全一致を部分一致より優先する。
    // 'head' で 'HeadTop_End' を拾わないため。
    const exact = bones.find((b) => norm(b.name) === pattern)
    if (exact) return exact
  }
  for (const pattern of patterns.map(norm)) {
    const partial = bones.find((b) => norm(b.name).includes(pattern))
    if (partial) return partial
  }
  return null
}

function findMorphs(root: Object3D, patterns: string[]): MorphRef[] {
  const wanted = patterns.map(norm)
  const out: MorphRef[] = []
  root.traverse((o) => {
    const mesh = o as Mesh
    if (!mesh.isMesh || !mesh.morphTargetDictionary) return
    for (const [name, index] of Object.entries(mesh.morphTargetDictionary)) {
      const n = norm(name)
      if (wanted.includes(n) || wanted.some((w) => n.includes(w))) {
        out.push({ mesh, index, name })
      }
    }
  })
  return out
}

/**
 * 読み込んだモデルから、動かせる部位を洗い出す。
 *
 * vrm を渡した場合は VRM の API を優先する。VRM のボーン名は実装依存で
 * （VRoid は 'J_Bip_C_Head'）、表情も expressionManager 経由なので、
 * 名前のパターン照合より規格のAPIのほうが確実である。
 */
export function inspectAvatar(
  root: Object3D,
  vrm?: { headBone: Bone | null; expression: VrmExpressionSetter | null } | null,
): AvatarCapabilities {
  let skinned = false
  root.traverse((o) => {
    if ((o as { isSkinnedMesh?: boolean }).isSkinnedMesh) skinned = true
  })
  const expression = vrm?.expression ?? null
  return {
    // VRM の humanoid が頭を教えてくれるならそれを使う
    headBone: vrm?.headBone ?? findBone(root, HEAD_BONE_PATTERNS),
    jawBone: findBone(root, JAW_BONE_PATTERNS),
    // VRM の表情があるなら生のモーフは見ない（二重に動かすと破綻する）
    blinkTargets: expression?.hasBlink ? [] : findMorphs(root, BLINK_PATTERNS),
    mouthTargets: expression?.hasMouth ? [] : findMorphs(root, MOUTH_PATTERNS),
    skinned,
    vrmExpression: expression,
  }
}

/**
 * 足りないものを返す。空配列なら面接官として一通り動ける。
 *
 * 優先順は「うなずき > 口 > まばたき」。うなずきは相手が話を聞いていることを
 * 示す最小の動作で、無いと会話として成立しない。
 */
export function avatarDeficiencies(caps: AvatarCapabilities): AvatarDeficiency[] {
  const out: AvatarDeficiency[] = []
  if (!caps.skinned) out.push('no-skeleton')
  if (!caps.headBone) out.push('no-head-bone')
  if (caps.mouthTargets.length === 0 && !caps.jawBone && !caps.vrmExpression?.hasMouth) {
    out.push('no-mouth')
  }
  if (caps.blinkTargets.length === 0 && !caps.vrmExpression?.hasBlink) out.push('no-blink')
  return out
}

const DEFICIENCY_MESSAGE: Record<AvatarDeficiency, string> = {
  'no-skeleton': '骨格（skin）が無い。静止メッシュなので全身が動かせない',
  'no-head-bone': '頭・首のボーンが無いのでうなずけない',
  'no-mouth': '口のモーフも顎ボーンも無いので口が動かせない',
  'no-blink': 'まばたきのモーフが無い',
}

/** 開発時に原因を1行で分かるようにする。描画は止めない。 */
export function describeDeficiencies(d: AvatarDeficiency[]): string {
  if (d.length === 0) return ''
  return 'アバターモデルが要件を満たしていません: ' +
    d.map((k) => DEFICIENCY_MESSAGE[k]).join(' / ') +
    '。リグ済みのモデル（VRM 推奨）に差し替えてください。public/avatars/README.md 参照'
}
