import { buildCapabilitiesInfo } from '@/lib/interview/live2d-capabilities-info'

describe('buildCapabilitiesInfo', () => {
  it('reads parameterIds as-is', () => {
    const info = buildCapabilitiesInfo(['ParamMouthOpenY', 'ParamAngleY'], null, null)
    expect(info.parameterIds).toEqual(['ParamMouthOpenY', 'ParamAngleY'])
  })

  it('reads LipSync/EyeBlink ids from the model3.json accessors', () => {
    const lipSync = { count: 1, idAt: () => 'ParamMouthOpenY' }
    const eyeBlink = { count: 2, idAt: (i: number) => ['ParamEyeLOpen', 'ParamEyeROpen'][i] }
    const info = buildCapabilitiesInfo(['ParamMouthOpenY', 'ParamEyeLOpen', 'ParamEyeROpen'], lipSync, eyeBlink)
    expect(info.lipSyncIds).toEqual(['ParamMouthOpenY'])
    expect(info.eyeBlinkIds).toEqual(['ParamEyeLOpen', 'ParamEyeROpen'])
  })

  it('leaves lipSyncIds/eyeBlinkIds undefined when no groups are declared (Mao-style model)', () => {
    const info = buildCapabilitiesInfo(['ParamA'], null, null)
    expect(info.lipSyncIds).toBeUndefined()
    expect(info.eyeBlinkIds).toBeUndefined()
  })
})
