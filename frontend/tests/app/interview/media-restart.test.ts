/**
 * @jest-environment jsdom
 */
import { renderHook, act, waitFor } from '@testing-library/react'
import { useInterviewMedia } from '@/app/interview/hooks/useInterviewMedia'

type FakeTrack = MediaStreamTrack & { readyState: 'live' | 'ended'; stop: jest.Mock }

let created: Array<{ id: number; tracks: FakeTrack[] }> = []
let nextId = 0

function makeStream() {
  const id = nextId++
  const mk = (kind: string): FakeTrack => {
    const t = {
      kind,
      readyState: 'live' as const,
      enabled: true,
      stop: jest.fn(function (this: FakeTrack) {
        this.readyState = 'ended'
      }),
    }
    return t as unknown as FakeTrack
  }
  const tracks = [mk('audio'), mk('video')]
  created.push({ id, tracks })
  return {
    id,
    getTracks: () => tracks,
    getAudioTracks: () => tracks.filter((t) => t.kind === 'audio'),
    getVideoTracks: () => tracks.filter((t) => t.kind === 'video'),
  } as unknown as MediaStream
}

/** 掴みっぱなし = 停止されていないのに参照が失われたストリーム。 */
function liveStreamCount() {
  return created.filter((s) => s.tracks.some((t) => t.readyState === 'live')).length
}

beforeEach(() => {
  created = []
  nextId = 0
  Object.defineProperty(navigator, 'mediaDevices', {
    configurable: true,
    value: {
      getUserMedia: jest.fn(async () => {
        // 実機と同じく即座には返らない
        await new Promise((r) => setTimeout(r, 5))
        return makeStream()
      }),
    },
  })
})

describe('面接を繰り返してもデバイスを掴みっぱなしにしない', () => {
  // 報告された症状の核心。
  // handleStart は setStatus('connecting') したうえで ensureStream() を呼ぶため、
  // ロビープレビューの effect と ensureStream がほぼ同時に getUserMedia する。
  // まとめないと片方のストリームが stop() されずに残り、デバイスを掴み続ける。
  it('同時に取得しても getUserMedia は1回だけで、ストリームは1本', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    await act(async () => {
      await Promise.all([result.current.ensureStream(), result.current.ensureStream()])
    })

    await waitFor(() => {
      expect(created.length).toBe(1)
    })
    expect(liveStreamCount()).toBe(1)
  })

  it('同時取得の全員が同じストリームを受け取る', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    let a: MediaStream | null = null
    let b: MediaStream | null = null
    await act(async () => {
      ;[a, b] = await Promise.all([result.current.ensureStream(), result.current.ensureStream()])
    })
    expect(a).toBe(b)
  })

  // 面接 → 終了 → もう一度面接、を繰り返す。
  it('3回繰り返しても掴んだままのストリームは高々1本', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    for (let i = 0; i < 3; i++) {
      await act(async () => {
        await result.current.ensureStream()
      })
      act(() => {
        result.current.stopStream()
      })
      // 停止直後は掴んでいるストリームが無いこと
      expect(liveStreamCount()).toBe(0)
    }
  })

  it('停止後はもう一度取得できる', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    await act(async () => {
      await result.current.ensureStream()
    })
    act(() => {
      result.current.stopStream()
    })

    let again: MediaStream | null = null
    await act(async () => {
      again = await result.current.ensureStream()
    })
    expect(again).not.toBeNull()
    expect(liveStreamCount()).toBe(1)
  })

  // 片側のトラックだけ死んだストリームを再利用しない。
  // 以前は「全トラックが ended」でないと取り直さなかった。
  it('音声トラックだけ死んでいたら取り直す', async () => {
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    let first: MediaStream | null = null
    await act(async () => {
      first = await result.current.ensureStream()
    })
    // 音声だけ停止した状態を作る
    act(() => {
      first!.getAudioTracks().forEach((t) => t.stop())
    })

    let second: MediaStream | null = null
    await act(async () => {
      second = await result.current.ensureStream()
    })
    expect(second).not.toBe(first)
    expect(created.length).toBe(2)
  })
})

describe('デバイス使用中を権限拒否として扱わない', () => {
  it('ensureStream は使用中を NotReadableError として投げる', async () => {
    ;(navigator.mediaDevices.getUserMedia as jest.Mock).mockRejectedValue(
      new DOMException('Could not start video source', 'NotReadableError'),
    )
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    await expect(result.current.ensureStream()).rejects.toThrow('NotReadableError')
  })

  it('ensureStream は本物の権限拒否を NotAllowedError として投げる', async () => {
    ;(navigator.mediaDevices.getUserMedia as jest.Mock).mockRejectedValue(
      new DOMException('denied', 'NotAllowedError'),
    )
    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'connecting' }))

    await expect(result.current.ensureStream()).rejects.toThrow('NotAllowedError')
  })
})
