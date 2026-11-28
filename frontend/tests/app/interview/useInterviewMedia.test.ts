/**
 * @jest-environment jsdom
 */
import { renderHook, waitFor } from '@testing-library/react'
import { useInterviewMedia } from '@/app/interview/hooks/useInterviewMedia'

const denied = () => Object.assign(new Error('Permission denied'), { name: 'NotAllowedError' })
const fakeStream = () => {
  const track = { readyState: 'live', stop: jest.fn(), enabled: true }
  return { getTracks: () => [track], getAudioTracks: () => [track], getVideoTracks: () => [track] } as unknown as MediaStream
}

let getUserMedia: jest.Mock
beforeEach(() => {
  getUserMedia = jest.fn()
  Object.defineProperty(navigator, 'mediaDevices', { value: { getUserMedia }, configurable: true })
})

describe('useInterviewMedia 準備画面の取得', () => {
  it('初回が拒否扱いでも、個別に確かめて拒否されていなければ取り直してエラーを出さない (#2633)', async () => {
    getUserMedia
      .mockRejectedValueOnce(denied()) // 初回: 許可ダイアログ直後に拒否扱いで失敗
      .mockResolvedValue(fakeStream()) // 個別確認（カメラ・マイク）と取り直しは成功

    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'lobby' }))

    await waitFor(() => expect(getUserMedia).toHaveBeenCalledTimes(4))
    expect(result.current.lobbyPermissionError).toBeNull()
  })

  it('本当に拒否されたものがあれば、取り直さずにその名前でエラーを出す', async () => {
    getUserMedia.mockImplementation((c: MediaStreamConstraints) =>
      c.video === true && !c.audio ? Promise.reject(denied()) : c.audio && !c.video ? Promise.resolve(fakeStream()) : Promise.reject(denied()),
    )

    const { result } = renderHook(() => useInterviewMedia({ loading: false, status: 'lobby' }))

    await waitFor(() => expect(result.current.lobbyPermissionError).toContain('カメラへのアクセスが拒否されました'))
    expect(result.current.lobbyPermissionError).not.toContain('マイク')
    // 一括取得1回 + 個別確認2回 + エラー文言用の個別確認2回。取り直しはしない
    expect(getUserMedia).toHaveBeenCalledTimes(5)
  })
})
