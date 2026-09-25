/**
 * ハンズフリー録音の発話判定。
 *
 * 以前は固定の絶対しきい値 (RMS 0.015) ひとつで開始と停止を決めていた。
 * これには実環境で2つの問題があった。
 *
 * 1. 頭切れ
 *    「しきい値を超えてから録音を開始する」ため、開始を判断できた時点で
 *    最初の音は既に過ぎている。MediaRecorder の起動にも時間がかかる。
 *    日本語は先頭の1モーラが落ちると意味が変わる（「御社は」→「社は」）。
 *    認識モデルは冒頭を手がかりに文脈を決めるため、影響は先頭語だけに留まらない。
 *
 * 2. 固定しきい値が静かな話者と騒がしい部屋の両方に効かない
 *    小声だと 0.015 を超えず録音が始まらない。逆に暗騒音が 0.015 を超える
 *    部屋では、黙っていても発話とみなされて止まらない。
 *
 * 対策として、暗騒音を追従で推定し、2段階のしきい値を使う。
 *
 * - 開始は低いしきい値で早めに切る（先頭を取りこぼさない）
 * - 発話とみなすのは高いしきい値を超えたときだけ
 * - 高いほうを一定時間超えなければ、その録音は破棄する
 *
 * 先頭に無音が少し入るのは認識上ほぼ無害だが、頭が切れるのは有害。
 * 「早めに録り始めて、違ったら捨てる」ほうが安全側に倒れる。
 */

/** 暗騒音の推定が過剰に下がらないための下限。完全な無音環境での除算を防ぐ。 */
export const NOISE_FLOOR_MIN = 0.002
/** 暗騒音がこれを超えたら、それ以上は追従しない（騒音で判定が壊れるのを防ぐ）。 */
export const NOISE_FLOOR_MAX = 0.05
/** 録音を開始する倍率（暗騒音比）。低めにして頭切れを防ぐ。 */
export const START_RATIO = 2.0
/** 発話と確定する倍率（暗騒音比）。 */
export const SPEECH_RATIO = 3.5
/** 静かな環境でも、これ未満は発話とみなさない。 */
export const SPEECH_ABSOLUTE_MIN = 0.008
/** 開始後この時間内に発話が確定しなければ録音を破棄する。 */
export const CONFIRM_MS = 700
/** この無音が続いたら送信する。 */
export const SILENCE_MS = 2500
/** 録音開始からこの時間は無音でも止めない（息継ぎ対策）。 */
export const MIN_RECORDING_MS = 1000
/**
 * 暗騒音を較正するまで録音を開始しないフレーム数。
 *
 * 推定は下限 (NOISE_FLOOR_MIN) から始まるため、較正前はしきい値が低すぎる。
 * これが無いと、騒がしい部屋でハンズフリーを入れた瞬間に暗騒音そのもので
 * 録音が始まる。60fps で約0.5秒。
 */
export const WARMUP_FRAMES = 30

/** 暗騒音の追従係数。上がるときは速く、下がるときはゆっくり。 */
const RISE_ALPHA = 0.02
const FALL_ALPHA = 0.3

/**
 * 暗騒音の推定を更新する。
 *
 * 発話中は更新しない。発話音量を暗騒音として取り込むと、
 * しきい値が上がり続けて自分の声で止まらなくなる。
 */
export function updateNoiseFloor(current: number, rms: number): number {
  // 現在値より小さい観測は速く取り込む（静かになったら素早く追従）。
  // 大きい観測はゆっくり取り込む（一時的な物音でしきい値を上げない）。
  const alpha = rms < current ? FALL_ALPHA : RISE_ALPHA
  const next = current + (rms - current) * alpha
  return Math.min(NOISE_FLOOR_MAX, Math.max(NOISE_FLOOR_MIN, next))
}

export function startThreshold(noiseFloor: number): number {
  return noiseFloor * START_RATIO
}

export function speechThreshold(noiseFloor: number): number {
  return Math.max(noiseFloor * SPEECH_RATIO, SPEECH_ABSOLUTE_MIN)
}

export type VadState = {
  /** 録音中か。 */
  recording: boolean
  /** 現在の録音で発話が確定したか。 */
  speechConfirmed: boolean
  /** 録音を開始した時刻 (ms)。未録音なら null。 */
  startedAt: number | null
  /** 無音が始まった時刻 (ms)。無音でなければ null。 */
  silenceSince: number | null
  /** 暗騒音の推定値。 */
  noiseFloor: number
  /** 較正のために観測した非録音フレーム数。 */
  observed: number
}

export function initialVadState(): VadState {
  return {
    recording: false,
    speechConfirmed: false,
    startedAt: null,
    silenceSince: null,
    noiseFloor: NOISE_FLOOR_MIN,
    observed: 0,
  }
}

/** 呼び出し側が実行すべき動作。 */
export type VadAction = 'none' | 'start' | 'stop' | 'discard'

export type VadInput = {
  rms: number
  now: number
  /** 録音を開始してよいか（AI発話中・送信中は false）。 */
  canStart: boolean
}

/**
 * 1フレーム分の判定を行い、次の状態と動作を返す。
 *
 * 純粋関数にしてあるのは、実音声なしで境界を検証できるようにするため。
 */
export function stepVad(state: VadState, input: VadInput): { state: VadState; action: VadAction } {
  const { rms, now, canStart } = input

  if (!state.recording) {
    // 録音していない間だけ暗騒音を更新する。
    const noiseFloor = updateNoiseFloor(state.noiseFloor, rms)
    const observed = state.observed + 1
    // 較正が終わるまでは開始しない。しきい値が下限由来のままだと
    // 暗騒音そのもので録音が始まってしまう。
    const calibrated = observed >= WARMUP_FRAMES
    if (calibrated && canStart && rms > startThreshold(noiseFloor)) {
      return {
        state: {
          recording: true,
          // 開始と同時に発話が確定していることもある（いきなり大声）
          speechConfirmed: rms > speechThreshold(noiseFloor),
          startedAt: now,
          silenceSince: null,
          noiseFloor,
          observed,
        },
        action: 'start',
      }
    }
    return { state: { ...state, noiseFloor, observed }, action: 'none' }
  }

  // 録音中。暗騒音は更新しない（自分の声を取り込まないため）。
  const speaking = rms > speechThreshold(state.noiseFloor)
  if (speaking) {
    return {
      state: { ...state, speechConfirmed: true, silenceSince: null },
      action: 'none',
    }
  }

  // 発話が確定しないまま確認時間を過ぎたら破棄する。
  // 物音やノイズで始まった録音を送らないための出口。
  if (!state.speechConfirmed && state.startedAt !== null && now - state.startedAt >= CONFIRM_MS) {
    return { state: initialVadStateKeepingNoise(state.noiseFloor), action: 'discard' }
  }

  // 発話確定後の無音。最短録音時間を過ぎ、無音が続いたら送信する。
  if (state.speechConfirmed) {
    const elapsed = state.startedAt === null ? Infinity : now - state.startedAt
    if (elapsed >= MIN_RECORDING_MS) {
      const silenceSince = state.silenceSince ?? now
      if (now - silenceSince >= SILENCE_MS) {
        return { state: initialVadStateKeepingNoise(state.noiseFloor), action: 'stop' }
      }
      return { state: { ...state, silenceSince }, action: 'none' }
    }
  }

  return { state, action: 'none' }
}

// 録音が終わったあとも較正結果は保持する。毎ターン較正し直すと、
// 2ターン目以降の先頭が取りこぼされる。
function initialVadStateKeepingNoise(noiseFloor: number): VadState {
  return { ...initialVadState(), noiseFloor, observed: WARMUP_FRAMES }
}
