import { toLive2DModelInfo, type Live2DModelLike } from '@/lib/interview/live2d-loader'

function fakeModel(opts: {
  parameterIds: string[]
  groups?: Array<{ Name: string; Ids: string[] }>
  useFileReferences?: boolean
}): Live2DModelLike {
  const json = opts.useFileReferences
    ? { FileReferences: { Groups: opts.groups ?? [] } }
    : { Groups: opts.groups ?? [] }
  return {
    internalModel: {
      coreModel: {
        getModel: () => ({ parameters: { ids: opts.parameterIds } }),
      },
      settings: { json },
    },
  }
}

describe('toLive2DModelInfo', () => {
  it('reads parameterIds from coreModel.getModel().parameters.ids', () => {
    const model = fakeModel({ parameterIds: ['ParamMouthOpenY', 'ParamAngleY'] })
    expect(toLive2DModelInfo(model).parameterIds).toEqual(['ParamMouthOpenY', 'ParamAngleY'])
  })

  it('reads LipSync/EyeBlink groups from the top-level Groups field (model3.json spec)', () => {
    const model = fakeModel({
      parameterIds: ['ParamMouthOpenY', 'ParamEyeLOpen', 'ParamEyeROpen'],
      groups: [
        { Name: 'LipSync', Ids: ['ParamMouthOpenY'] },
        { Name: 'EyeBlink', Ids: ['ParamEyeLOpen', 'ParamEyeROpen'] },
      ],
    })
    const info = toLive2DModelInfo(model)
    expect(info.lipSyncIds).toEqual(['ParamMouthOpenY'])
    expect(info.eyeBlinkIds).toEqual(['ParamEyeLOpen', 'ParamEyeROpen'])
  })

  it('falls back to FileReferences.Groups when Groups is nested there instead', () => {
    const model = fakeModel({
      parameterIds: ['ParamMouthOpenY'],
      groups: [{ Name: 'LipSync', Ids: ['ParamMouthOpenY'] }],
      useFileReferences: true,
    })
    expect(toLive2DModelInfo(model).lipSyncIds).toEqual(['ParamMouthOpenY'])
  })

  it('leaves lipSyncIds/eyeBlinkIds undefined when no Groups are declared (Mao-style model)', () => {
    const model = fakeModel({ parameterIds: ['ParamA'] })
    const info = toLive2DModelInfo(model)
    expect(info.lipSyncIds).toBeUndefined()
    expect(info.eyeBlinkIds).toBeUndefined()
  })
})
