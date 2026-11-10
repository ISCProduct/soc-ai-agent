/**
 * @jest-environment jsdom
 */
/**
 * Live2D の非同期な失敗で、アバターが黙って止まらないこと。
 *
 * `onUnavailable` を呼べない失敗が2つあった。
 * 1. Core 読み込み後の初期化（Framework の動的import、`ensureCubismFramework`）の失敗。
 *    `setup()` の中の try/catch を通らず、未処理のPromise拒否になるだけで Avatar2D に
 *    切り替わらない。import の失敗も同じ経路なので、ここでは初期化の例外で代表させる。
 * 2. 描画ループ（requestAnimationFrame のコールバック）の例外。次フレームを要求できず
 *    canvas が止まるが、rAF の例外は React のエラー境界にも届かない。
 * どちらも親（InterviewerAvatar）が Avatar2D へ切り替えられるよう `onUnavailable` に落とす。
 */
import { act, render } from '@testing-library/react'

import Live2DAvatar from '@/app/interview/components/Live2DAvatar'
import { FEMALE_OVERLAY_MESH_IDS, OVERLAY_MESH_IDS } from '@/lib/interview/live2d-overlay'

jest.mock('@/lib/interview/live2d-loader', () => ({
  // Core は読めた（＝既存の try/catch は通らない）状況を作る。
  ensureCubismCore: jest.fn().mockResolvedValue(undefined),
  modelUrlForGender: (gender: 'male' | 'female') =>
    `/live2d/interviewers/${gender}/${gender}-interviewer.model3.json`,
  DEFAULT_SHADER_PATH: '/live2d/shaders/',
}))

/** 面接官として使える最小のモデル（口=振幅、うなずき=AngleY、まばたき=両目）。 */
const USABLE_PARAM_IDS = ['ParamMouthOpenY', 'ParamAngleY', 'ParamEyeLOpen', 'ParamEyeROpen']

/** テストごとに「初期化で投げる」「描画で投げる」を切り替える。 */
let failInit = false
let throwOnFrame = false
/** Live2DFrameworkModel.load に渡された引数。性別ごとの読み込み先の確認に使う。 */
let loadArgs: unknown[] = []

jest.mock('@/lib/interview/live2d-framework-model', () => ({
  __esModule: true,
  ensureCubismFramework: () => {
    if (failInit) throw new Error('Live2DCubismCore is not defined')
  },
  Live2DFrameworkModel: {
    load: (...args: unknown[]) => ((loadArgs = args), {
      getCapabilitiesInfo: () => ({ parameterIds: USABLE_PARAM_IDS }),
      resize: jest.fn(),
      release: jest.fn(),
      setParameterValue: jest.fn(),
      setOverlayState: jest.fn(),
      draw: jest.fn(),
      commitFrame: () => {
        if (throwOnFrame) throw new Error('Core の drawable が解放済み')
      },
    }),
  },
}))

/** jsdom には WebGL も ResizeObserver も無いので、描画ループが回るぶんだけ用意する。 */
const fakeGl = {
  clearColor: jest.fn(),
  viewport: jest.fn(),
  clear: jest.fn(),
  COLOR_BUFFER_BIT: 0x4000,
}

describe('Live2DAvatar の非同期な失敗', () => {
  let rafCallbacks: FrameRequestCallback[]

  beforeEach(() => {
    failInit = false
    throwOnFrame = false
    loadArgs = []
    jest.spyOn(console, 'warn').mockImplementation(() => {})
    rafCallbacks = []
    global.ResizeObserver = class {
      observe() {}
      unobserve() {}
      disconnect() {}
    } as unknown as typeof ResizeObserver
    jest.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => {
      rafCallbacks.push(cb)
      return rafCallbacks.length
    })
    jest.spyOn(window, 'cancelAnimationFrame').mockImplementation(() => {})
    jest
      .spyOn(HTMLCanvasElement.prototype, 'getContext')
      .mockImplementation(() => fakeGl as unknown as RenderingContext)
  })
  afterEach(() => {
    jest.restoreAllMocks()
  })

  async function renderAvatar(gender: 'male' | 'female' = 'male') {
    const onUnavailable = jest.fn()
    await act(async () => {
      render(
        <Live2DAvatar gender={gender} level={0} speaking={false} active onUnavailable={onUnavailable} />,
      )
    })
    return onUnavailable
  }

  it('Core 読み込み後の初期化で投げたら onUnavailable を呼ぶ', async () => {
    failInit = true
    const onUnavailable = await renderAvatar()
    expect(onUnavailable).toHaveBeenCalled()
  })

  it('描画ループで例外が出たら onUnavailable を呼び、次フレームを要求しない', async () => {
    const onUnavailable = await renderAvatar()
    expect(onUnavailable).not.toHaveBeenCalled()
    expect(rafCallbacks).toHaveLength(1)

    throwOnFrame = true
    await act(async () => {
      rafCallbacks[0](16)
    })
    expect(onUnavailable).toHaveBeenCalled()
    // 例外のあとも回し続けると、同じ例外を毎フレーム出し続ける
    expect(rafCallbacks).toHaveLength(1)
  })

  // 書き出し済みモデルは男女で別ファイルかつ差分ArtMeshのIDも逆。取り違えると
  // 画面には「相手の性別の面接官」や「何も描画されないcanvas」が出る。
  it.each([
    ['male', '/live2d/interviewers/male/', 'male-interviewer.model3.json', OVERLAY_MESH_IDS],
    ['female', '/live2d/interviewers/female/', 'female-interviewer.model3.json', FEMALE_OVERLAY_MESH_IDS],
  ] as const)('%s は自分のモデルと差分メッシュIDで読み込む', async (gender, dir, fileName, meshIds) => {
    await renderAvatar(gender)
    expect(loadArgs[1]).toBe(dir)
    expect(loadArgs[2]).toBe(fileName)
    expect(loadArgs[6]).toEqual(meshIds)
  })
})
