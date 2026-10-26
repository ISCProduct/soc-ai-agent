/**
 * Live2Dモデルが「うちの音声パイプラインで動かせるか」を判定する。
 *
 * 面接官が喋らない・うなずかない原因は、ほぼ必ずコードではなくモデル側にある。
 * 3D版(ThreeAvatar)を捨てた経緯がそのままで、あのときの male-avatar.glb は
 * 骨もモーフも持たない静止メッシュだったため、どんなコードを書いても動かなかった。
 *
 * Live2Dでも同じことが起きる。公式サンプルモデルを調べたら、どちらも実在した。
 *
 *   Mao : 132パラメータ持つが ParamMouthOpenY が無く、口は あいうえお の母音のみ。
 *         model3.json も LipSync -> ["ParamA"] と申告している。
 *         うちが持っているのは音声振幅(aiLevel)だけで母音の判別はしていないので、
 *         「あ」の開閉で近似するしかない。
 *   Mark: ParamMouthOpenY を持っているのに LipSync -> [] （空）。
 *         SDK標準のリップシンクは LipSync グループを回すだけなので、
 *         パラメータがあっても口は動かない。自分で名指しして動かす必要がある。
 *
 * つまり「パラメータがあるか」だけ見ても足りず、モデル自身の申告(model3.jsonの
 * Groups)を一次情報として読む必要がある。描画は止めない（面接を止めるより、
 * 動かないアバターでも面接を続けるほうがよい）。
 */

/** 標準パラメータID。Cubismの命名規約。 */
export const PARAM = {
  mouthOpen: 'ParamMouthOpenY',
  eyeLOpen: 'ParamEyeLOpen',
  eyeROpen: 'ParamEyeROpen',
  angleX: 'ParamAngleX',
  angleY: 'ParamAngleY',
  breath: 'ParamBreath',
} as const

/** 母音方式のモデルの口パラメータ。開き具合に最も近いのは「あ」。 */
export const VOWEL_PARAMS = ['ParamA', 'ParamI', 'ParamU', 'ParamE', 'ParamO'] as const

/**
 * 口の駆動方式。
 * - `amplitude`   : LipSync に ParamMouthOpenY が申告されている（本命）
 * - `vowel`       : LipSync が母音パラメータ。振幅を「あ」に流して近似する
 * - `undeclared`  : LipSync が空だが ParamMouthOpenY は存在する。自分で名指しすれば動く
 * - `none`        : 申告も使えるパラメータも無い。リップシンク不可
 */
export type MouthScheme = 'amplitude' | 'vowel' | 'undeclared' | 'none'

/** モデルから読み取る情報。`.model3.json` と `.cdi3.json`（または実行時のモデル）由来。 */
export interface Live2DModelInfo {
  /** 公開されている全パラメータID。 */
  parameterIds: readonly string[]
  /** `model3.json` の Groups[Name="LipSync"].Ids。モデル自身の申告。 */
  lipSyncIds?: readonly string[]
  /** 同 Groups[Name="EyeBlink"].Ids。 */
  eyeBlinkIds?: readonly string[]
}

export interface Live2DCapabilities {
  mouth: MouthScheme
  /** 実際に振幅を流すパラメータ。空ならリップシンクしない。 */
  mouthParamIds: string[]
  /** まばたきに使うパラメータ。空ならまばたきしない。 */
  blinkParamIds: string[]
  /** うなずきに使える首の角度パラメータがあるか。 */
  canNod: boolean
  /** 満たせなかった要件。空なら問題なし。 */
  deficiencies: Live2DDeficiency[]
}

export type Live2DDeficiency =
  | 'no-mouth-param'
  | 'vowel-mouth-only'
  | 'lipsync-group-empty'
  | 'no-head-angle'
  | 'no-eye-params'

/** 欠けているものの説明。check:live2d と開発時の警告に出す。 */
export const DEFICIENCY_REASON: Record<Live2DDeficiency, string> = {
  'no-mouth-param':
    `LipSync グループの申告が無く、${PARAM.mouthOpen} も母音パラメータも持っていません。` +
    'リップシンクは成立しません。面接官には使えません。',
  'vowel-mouth-only':
    `${PARAM.mouthOpen} が無く、口は母音パラメータだけです。音声振幅を「あ」に流して` +
    '近似します。母音の作り分けはしていないので、口の形は「あ」のまま開閉します。',
  'lipsync-group-empty':
    `${PARAM.mouthOpen} は存在しますが model3.json の LipSync グループが空です。` +
    'SDK標準のリップシンクは LipSync を回すだけなので、このままでは口が動きません。' +
    'パラメータを名指しで駆動するか、Cubism Editor で LipSync グループに追加してください。',
  'no-head-angle':
    `${PARAM.angleY} / ${PARAM.angleX} が無く、相槌のうなずきができません。`,
  'no-eye-params':
    `${PARAM.eyeLOpen} / ${PARAM.eyeROpen} が揃っておらず、まばたきができません。`,
}

function pickMouth(
  info: Live2DModelInfo,
  ids: Set<string>,
): { scheme: MouthScheme; paramIds: string[]; deficiency: Live2DDeficiency | null } {
  // モデル自身の申告を一次情報にする。存在するパラメータだけに絞る
  // （申告されているのに実体が無いモデルもあり得るため）。
  const declared = (info.lipSyncIds ?? []).filter((id) => ids.has(id))

  if (declared.includes(PARAM.mouthOpen)) {
    return { scheme: 'amplitude', paramIds: [PARAM.mouthOpen], deficiency: null }
  }
  if (declared.length > 0) {
    // 母音方式。開き具合に最も近い「あ」だけを動かす。
    const a = declared.find((id) => id === VOWEL_PARAMS[0]) ?? declared[0]
    return { scheme: 'vowel', paramIds: [a], deficiency: 'vowel-mouth-only' }
  }
  // 申告が無い・空。パラメータの実体があれば自分で名指しして動かせる。
  if (ids.has(PARAM.mouthOpen)) {
    return { scheme: 'undeclared', paramIds: [PARAM.mouthOpen], deficiency: 'lipsync-group-empty' }
  }
  const vowel = VOWEL_PARAMS.find((id) => ids.has(id))
  if (vowel) {
    return { scheme: 'vowel', paramIds: [vowel], deficiency: 'vowel-mouth-only' }
  }
  return { scheme: 'none', paramIds: [], deficiency: 'no-mouth-param' }
}

/** モデルの情報から、できること・できないことを出す。 */
export function inspectLive2DModel(info: Live2DModelInfo): Live2DCapabilities {
  const ids = new Set(info.parameterIds)
  const deficiencies: Live2DDeficiency[] = []

  const mouth = pickMouth(info, ids)
  if (mouth.deficiency) deficiencies.push(mouth.deficiency)

  // うなずきは上下(AngleY)が主役。片方しか無いモデルでも傾きで代用できる。
  const canNod = ids.has(PARAM.angleY) || ids.has(PARAM.angleX)
  if (!canNod) deficiencies.push('no-head-angle')

  // 申告があればそれを使う。無ければ標準名で拾う。
  const declaredBlink = (info.eyeBlinkIds ?? []).filter((id) => ids.has(id))
  const fallbackBlink = [PARAM.eyeLOpen, PARAM.eyeROpen].filter((id) => ids.has(id))
  const blink = declaredBlink.length > 0 ? declaredBlink : fallbackBlink
  // 片目だけ閉じると不気味になるので、両目そろって初めて使う。
  const blinkParamIds = blink.length >= 2 ? blink : []
  if (blinkParamIds.length === 0) deficiencies.push('no-eye-params')

  return {
    mouth: mouth.scheme,
    mouthParamIds: mouth.paramIds,
    blinkParamIds,
    canNod,
    deficiencies,
  }
}

/** 面接官として最低限成立するか。口が動かないモデルは採用しない。 */
export function isUsableAsInterviewer(caps: Live2DCapabilities): boolean {
  return caps.mouthParamIds.length > 0 && caps.canNod
}

/**
 * 音声振幅を口の開き(0-1)にする。Avatar2D と同じ味付けにそろえてある。
 * 喋っている間は完全に閉じない（0.15が下限）。無音で口が閉じきると、
 * 発話中なのに静止画に見えるため。
 */
export function mouthOpenValue(level: number, speaking: boolean): number {
  if (!speaking) return 0
  if (!Number.isFinite(level)) return 0.15
  return Math.max(0.15, Math.min(1, level))
}

/**
 * うなずき1回ぶんの首の角度を返す。0→1 の進行度に対し、下げて戻す山を1つ作る。
 * Cubism の Angle 系は -30..30 度。
 */
export function nodAngleY(progress: number): number {
  if (!Number.isFinite(progress) || progress <= 0 || progress >= 1) return 0
  return -Math.sin(progress * Math.PI) * 12
}
