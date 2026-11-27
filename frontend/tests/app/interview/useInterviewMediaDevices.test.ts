/**
 * @jest-environment jsdom
 */
import { act, renderHook, waitFor } from '@testing-library/react'
import { useInterviewMedia } from '@/app/interview/hooks/useInterviewMedia'
import { buildMediaConstraints, MEDIA_CONSTRAINTS } from '@/app/interview/lib/mediaStream'
import { applyAudioOutput, setAudioOutputId } from '@/lib/interview/audio-output'

type FakeTrack = { readyState: string; enabled: boolean; stop: jest.Mock }
const fakeStream = () => {
  const audio: FakeTrack = { readyState: 'live', enabled: true, stop: jest.fn() }
  const video: FakeTrack = { readyState: 'live', enabled: true, stop: jest.fn() }
  return {
    getTracks: () => [audio, video],
    getAudioTracks: () => [audio],
    getVideoTracks: () => [video],
  } as unknown as MediaStream
}

let getUserMedia: jest.Mock
beforeEach(() => {
  getUserMedia = jest.fn().mockImplementation(() => Promise.resolve(fakeStream()))
  Object.defineProperty(navigator, 'mediaDevices', { value: { getUserMedia }, configurable: true })
  jest.spyOn(window, 'requestAnimationFrame').mockImplementation((cb) => { cb(0); return 0 })
})
afterEach(() => jest.restoreAllMocks())

describe('buildMediaConstraints', () => {
  it('未選択なら既定の制約のまま、選んだ機器は exact で指定する', () => {
    expect(buildMediaConstraints('', '')).toEqual(MEDIA_CONSTRAINTS)
    const c = buildMediaConstraints('mic-b', 'cam-b')
    expect(c.audio).toMatchObject({ deviceId: { exact: 'mic-b' }, echoCancellation: true })
    expect(c.video).toEqual({ deviceId: { exact: 'cam-b' } })
  })
})

describe('useInterviewMedia 機器の選び直し', () => {
  it('選んだマイクで取り直し、オフにしていたマイクはオフのまま引き継ぐ', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'lobby' }))
    await waitFor(() => expect(result.current.lobbyStream).not.toBeNull())

    act(() => result.current.toggleMic())
    await act(() => result.current.selectInputDevice('mic', 'mic-b'))

    expect(getUserMedia).toHaveBeenLastCalledWith(buildMediaConstraints('mic-b', ''))
    expect(result.current.micId).toBe('mic-b')
    expect(result.current.lobbyStream?.getAudioTracks()[0].enabled).toBe(false)
    expect(result.current.lobbyStream?.getVideoTracks()[0].enabled).toBe(true)
  })

  it('選んだ機器で取れない（抜かれた）ときは、既定の機器で取り直して選択を外す', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'lobby' }))
    await waitFor(() => expect(result.current.lobbyStream).not.toBeNull())

    getUserMedia.mockImplementationOnce(() =>
      Promise.reject(Object.assign(new Error('gone'), { name: 'OverconstrainedError' })),
    )
    await act(() => result.current.selectInputDevice('camera', 'cam-gone'))

    expect(getUserMedia).toHaveBeenLastCalledWith(MEDIA_CONSTRAINTS)
    expect(result.current.cameraId).toBe('')
    expect(result.current.lobbyPermissionError).toBeNull()
  })
})

describe('applyAudioOutput', () => {
  // 対応ブラウザと同じく、setSinkId は prototype に置く
  class SinkCtx {
    sinkId = ''
    async setSinkId(id: string) { this.sinkId = id }
  }
  beforeEach(() => { (globalThis as { AudioContext?: unknown }).AudioContext = SinkCtx })
  afterEach(() => {
    delete (globalThis as { AudioContext?: unknown }).AudioContext
    setAudioOutputId('')
  })

  it('選んだスピーカーを当て、同じなら当て直さない', async () => {
    const ctx = new SinkCtx()
    const spy = jest.spyOn(ctx, 'setSinkId')
    setAudioOutputId('spk-b')
    await applyAudioOutput(ctx as unknown as AudioContext)
    await applyAudioOutput(ctx as unknown as AudioContext)

    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith('spk-b')
  })
})
