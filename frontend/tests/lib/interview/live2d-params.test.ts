import {
  inspectLive2DModel,
  isUsableAsInterviewer,
  mouthOpenValue,
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
      expect(caps.deficiencies).toEqual([])
      expect(isUsableAsInterviewer(caps)).toBe(true)
    },
  )

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
