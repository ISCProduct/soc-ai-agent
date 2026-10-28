import {
  inspectLive2DModel,
  isUsableAsInterviewer,
  mouthOpenValue,
  smoothMouth,
  valueAt,
  normalizeLevel,
  nodAngleY,
  PARAM,
  type Live2DModelInfo,
} from '@/lib/interview/live2d-params'

/**
 * Live2D 公式サンプルモデルの実データ。
 * parameterIds は `<model>.cdi3.json` の Parameters[].Id（口・目・首に関係するものだけ）、
 * lipSyncIds / eyeBlinkIds は `<model>.model3.json` の Groups からそのまま取った。
 */
const FIXTURES: Record<string, Live2DModelInfo> = {
  // 70パラメータ。申告も実体も揃っている理想形。
  Hiyori: {
    parameterIds: [
      'ParamAngleX', 'ParamAngleY', 'ParamAngleZ',
      'ParamEyeLOpen', 'ParamEyeROpen', 'ParamEyeBallX', 'ParamEyeBallY',
      'ParamMouthForm', 'ParamMouthOpenY', 'ParamBodyAngleX', 'ParamBreath',
    ],
    lipSyncIds: ['ParamMouthOpenY'],
    eyeBlinkIds: ['ParamEyeLOpen', 'ParamEyeROpen'],
  },
  // 96パラメータ。Hiyori と同じく理想形。
  Natori: {
    parameterIds: [
      'ParamAngleX', 'ParamAngleY', 'ParamAngleZ',
      'ParamEyeLOpen', 'ParamEyeROpen',
      'ParamMouthForm', 'ParamMouthOpenY', 'ParamBodyAngleX', 'ParamBreath',
    ],
    lipSyncIds: ['ParamMouthOpenY'],
    eyeBlinkIds: ['ParamEyeLOpen', 'ParamEyeROpen'],
  },
  // 132パラメータあっても ParamMouthOpenY が無い。モデル自身も ParamA を申告。
  Mao: {
    parameterIds: [
      'ParamAngleX', 'ParamAngleY', 'ParamAngleZ',
      'ParamEyeLOpen', 'ParamEyeROpen',
      'ParamA', 'ParamI', 'ParamU', 'ParamE', 'ParamO',
      'ParamMouthUp', 'ParamMouthDown', 'ParamBodyAngleX', 'ParamBreath',
    ],
    lipSyncIds: ['ParamA'],
    eyeBlinkIds: ['ParamEyeLOpen', 'ParamEyeROpen'],
  },
  // ParamMouthOpenY はあるのに LipSync が空。パラメータ有無だけ見ると見逃す。
  Mark: {
    parameterIds: [
      'ParamAngleX', 'ParamAngleY', 'ParamAngleZ',
      'ParamEyeLOpen', 'ParamEyeROpen', 'ParamEyeBallX', 'ParamEyeBallY',
      'ParamMouthOpenY', 'ParamBodyAngleX', 'ParamBreath',
    ],
    lipSyncIds: [],
    eyeBlinkIds: ['ParamEyeLOpen', 'ParamEyeROpen'],
  },
}

describe('inspectLive2DModel', () => {
  it.each([['Hiyori'], ['Natori']])(
    '%s は LipSync に ParamMouthOpenY を申告しており、振幅をそのまま流せる',
    (name) => {
      const caps = inspectLive2DModel(FIXTURES[name])
      expect(caps.mouth).toBe('amplitude')
      expect(caps.mouthParamIds).toEqual([PARAM.mouthOpen])
      expect(caps.blinkParamIds).toEqual([PARAM.eyeLOpen, PARAM.eyeROpen])
      expect(caps.canNod).toBe(true)
      expect(caps.nodParamId).toBe(PARAM.angleY)
      expect(caps.deficiencies).toEqual([])
      expect(isUsableAsInterviewer(caps)).toBe(true)
    },
  )

  it('ParamAngleXしか無いモデルは、そのAngleXで代用する（AngleY決め打ちでは動かないバグの再発防止）', () => {
    const caps = inspectLive2DModel({
      parameterIds: [PARAM.mouthOpen, PARAM.angleX, PARAM.eyeLOpen, PARAM.eyeROpen],
      lipSyncIds: [PARAM.mouthOpen],
    })
    expect(caps.canNod).toBe(true)
    expect(caps.nodParamId).toBe(PARAM.angleX)
  })

  // パラメータ数の多さは「動く」の保証にならない。
  it('Mao は132パラメータあっても母音方式。「あ」だけを動かす', () => {
    const caps = inspectLive2DModel(FIXTURES.Mao)
    expect(caps.mouth).toBe('vowel')
    expect(caps.mouthParamIds).toEqual(['ParamA'])
    expect(caps.deficiencies).toEqual(['vowel-mouth-only'])
    expect(isUsableAsInterviewer(caps)).toBe(true)
  })

  // これが一番見つけにくい。パラメータはあるのに標準経路では動かない。
  it('Mark は ParamMouthOpenY があるが LipSync が空なので、名指し駆動が必要と分かる', () => {
    const caps = inspectLive2DModel(FIXTURES.Mark)
    expect(caps.mouth).toBe('undeclared')
    expect(caps.mouthParamIds).toEqual([PARAM.mouthOpen])
    expect(caps.deficiencies).toEqual(['lipsync-group-empty'])
    // 自分で動かせるので使えないとは判定しない。
    expect(isUsableAsInterviewer(caps)).toBe(true)
  })

  it('何も無いモデル（3D版のTripo製glbと同じ状態）は使えないと判定する', () => {
    const caps = inspectLive2DModel({ parameterIds: [] })
    expect(caps.mouth).toBe('none')
    expect(caps.mouthParamIds).toEqual([])
    expect(caps.canNod).toBe(false)
    expect(caps.nodParamId).toBeNull()
    expect(caps.deficiencies).toEqual(['no-mouth-param', 'no-head-angle', 'no-eye-params'])
    expect(isUsableAsInterviewer(caps)).toBe(false)
  })

  it('LipSync に申告されていても実体が無いパラメータは使わない', () => {
    const caps = inspectLive2DModel({
      parameterIds: ['ParamAngleY', 'ParamEyeLOpen', 'ParamEyeROpen'],
      lipSyncIds: [PARAM.mouthOpen],
    })
    expect(caps.mouth).toBe('none')
    expect(caps.deficiencies).toContain('no-mouth-param')
  })

  it('片目だけのモデルはまばたきさせない', () => {
    const caps = inspectLive2DModel({
      parameterIds: [PARAM.mouthOpen, PARAM.angleY, PARAM.eyeLOpen],
      lipSyncIds: [PARAM.mouthOpen],
    })
    expect(caps.blinkParamIds).toEqual([])
    expect(caps.deficiencies).toContain('no-eye-params')
  })
})

describe('mouthOpenValue', () => {
  it('喋っていないときは閉じる', () => {
    expect(mouthOpenValue(0.9, false)).toBe(0)
  })

  it('喋っている間は完全に閉じない（静止画に見えるため）', () => {
    expect(mouthOpenValue(0, true)).toBe(0.15)
  })

  it('振幅をそのまま使い、1で止める', () => {
    expect(mouthOpenValue(0.5, true)).toBe(0.5)
    expect(mouthOpenValue(3, true)).toBe(1)
  })

  it('負値やNaNでも範囲から出ない', () => {
    expect(mouthOpenValue(-1, true)).toBe(0.15)
    expect(mouthOpenValue(Number.NaN, true)).toBe(0.15)
  })
})

describe('nodAngleY', () => {
  it('始点と終点では傾いていない', () => {
    expect(nodAngleY(0)).toBe(0)
    expect(nodAngleY(1)).toBe(0)
  })

  it('途中で下を向き、Cubismの範囲(-30..30)を超えない', () => {
    const mid = nodAngleY(0.5)
    expect(mid).toBeLessThan(0)
    expect(mid).toBeGreaterThan(-30)
  })

  it('範囲外・NaNは0に倒す', () => {
    expect(nodAngleY(1.5)).toBe(0)
    expect(nodAngleY(Number.NaN)).toBe(0)
  })
})

describe('smoothMouth', () => {
  it('目標へ向かうが、1フレームでは届かない', () => {
    const v = smoothMouth(0, 1, 1 / 60)
    expect(v).toBeGreaterThan(0)
    expect(v).toBeLessThan(0.5)
  })

  it('十分な時間がたてば目標に収束する', () => {
    expect(smoothMouth(0, 1, 2)).toBeCloseTo(1, 3)
    expect(smoothMouth(1, 0, 2)).toBeCloseTo(0, 3)
  })

  it('開くほうが閉じるより速い', () => {
    const opened = smoothMouth(0, 1, 1 / 60)
    const closed = 1 - smoothMouth(1, 0, 1 / 60)
    expect(opened).toBeGreaterThan(closed)
  })

  it('dt が 0 や負なら動かず、NaN の前値は目標で置き換える', () => {
    expect(smoothMouth(0.3, 1, 0)).toBe(0.3)
    expect(smoothMouth(0.3, 1, -1)).toBe(0.3)
    expect(smoothMouth(NaN, 0.5, 0.016)).toBe(0.5)
  })
})

describe('valueAt', () => {
  const h = [{ t: 1, v: 0.1 }, { t: 2, v: 0.5 }, { t: 3, v: 0.9 }]

  it('指定時刻以前で最も新しい値を返す', () => {
    expect(valueAt(h, 2.5)).toBe(0.5)
    expect(valueAt(h, 3)).toBe(0.9)
    expect(valueAt(h, 99)).toBe(0.9)
  })

  it('履歴より前の時刻なら最も古い値、空なら 0', () => {
    expect(valueAt(h, 0)).toBe(0.1)
    expect(valueAt([], 5)).toBe(0)
  })
})

describe('normalizeLevel', () => {
  it('小さな声でも、直近の最大に対する割合で口が全開になる', () => {
    // 声が小さく raw の最大が 0.3 でも、そのピークでは level=1 になる
    const a = normalizeLevel(0.3, 0, 1 / 60)
    expect(a.level).toBe(1)
    const b = normalizeLevel(0.15, a.peak, 1 / 60)
    expect(b.level).toBeCloseTo(0.5, 1)
  })

  it('無音やごく小さい音は増幅しない（floor が下限）', () => {
    expect(normalizeLevel(0.02, 0, 1 / 60).level).toBeCloseTo(0.02 / 0.15, 5)
    expect(normalizeLevel(0, 0, 1 / 60).level).toBe(0)
  })

  it('peak は時間とともに下がる', () => {
    const loud = normalizeLevel(1, 0, 1 / 60)
    const later = normalizeLevel(0, loud.peak, 5)
    expect(later.peak).toBeLessThan(0.3)
  })

  it('NaN や範囲外は 0-1 に収める', () => {
    expect(normalizeLevel(NaN, NaN, NaN).level).toBe(0)
    expect(normalizeLevel(5, 0, 0.016).level).toBe(1)
  })
})
