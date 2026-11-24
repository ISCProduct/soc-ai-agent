/**
 * @jest-environment jsdom
 *
 * 面接開始前の Live2D 資産の先読み。表示のときに1つずつ待つと面接官が出るまで数秒かかる。
 */
describe('preloadLive2D', () => {
  const prevFlag = process.env.NEXT_PUBLIC_LIVE2D_ENABLED
  let fetchMock: jest.Mock

  beforeEach(() => {
    jest.resetModules()
    document.head.innerHTML = ''
    fetchMock = jest.fn((url: string) =>
      Promise.resolve(
        url.endsWith('.model3.json')
          ? {
              ok: true,
              json: () =>
                Promise.resolve({
                  FileReferences: { Moc: 'm.moc3', Physics: 'm.physics3.json', Textures: ['m.2048/texture_00.png'] },
                }),
            }
          : { ok: true },
      ),
    )
    global.fetch = fetchMock as unknown as typeof fetch
  })

  afterEach(() => {
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = prevFlag
  })

  const flush = () => new Promise((resolve) => setTimeout(resolve, 0))

  it('Core を読み始め、モデル一式（moc3・physics・テクスチャ）を取りに行く', async () => {
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = 'true'
    const { preloadLive2D } = await import('@/lib/interview/live2d-loader')
    preloadLive2D('male')
    await flush()

    const dir = '/live2d/interviewers/male/'
    expect(fetchMock.mock.calls.map((c) => c[0])).toEqual([
      `${dir}male-interviewer.model3.json`,
      `${dir}m.moc3`,
      `${dir}m.physics3.json`,
      `${dir}m.2048/texture_00.png`,
    ])
    expect(document.head.querySelector('script[src="/live2d/core/live2dcubismcore.min.js"]')).not.toBeNull()
  })

  it('2回目は何もしない（画面の再描画のたびに取り直さない）', async () => {
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = 'true'
    const { preloadLive2D } = await import('@/lib/interview/live2d-loader')
    preloadLive2D('male')
    await flush()
    const calls = fetchMock.mock.calls.length
    preloadLive2D('male')
    await flush()
    expect(fetchMock.mock.calls.length).toBe(calls)
  })

  it('Live2D が無効な環境では何も取りに行かない（未配置の資産へ毎回 404 を出さない）', async () => {
    process.env.NEXT_PUBLIC_LIVE2D_ENABLED = 'false'
    const { preloadLive2D } = await import('@/lib/interview/live2d-loader')
    preloadLive2D('male')
    await flush()
    expect(fetchMock).not.toHaveBeenCalled()
    expect(document.head.querySelector('script')).toBeNull()
  })
})
