/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import ReviewPlayer from '@/components/interview/ReviewPlayer'
import { interviewApi, type InterviewPlayback } from '@/lib/interview'

jest.mock('@/lib/interview', () => ({
  interviewApi: { videoPlaybackURL: jest.fn() },
}))

const videoPlaybackURL = interviewApi.videoPlaybackURL as jest.MockedFunction<typeof interviewApi.videoPlaybackURL>

const playback: InterviewPlayback = {
  videos: [{ id: 4, status: 'done', mime_type: 'video/webm' }],
  cues: [{ utterance_id: 9, role: 'user', offset_seconds: 42 }],
  anchor: 'session_started_at',
}

describe('録画で振り返る', () => {
  beforeEach(() => {
    videoPlaybackURL.mockReset()
    videoPlaybackURL.mockResolvedValue({ url: 'https://example.test/video', expires_at: '2026-11-20T00:15:00Z' })
    Object.defineProperty(window.HTMLMediaElement.prototype, 'play', {
      configurable: true,
      value: jest.fn().mockResolvedValue(undefined),
    })
  })

  it('録画が無いことを伝える', () => {
    render(
      <ReviewPlayer
        sessionId={1}
        playback={{ videos: [], cues: [], anchor: 'session_started_at' }}
        loading={false}
        error={null}
        onRetry={() => undefined}
        seekUtteranceId={null}
        seekNonce={0}
      />,
    )
    expect(screen.getByText('この面接の録画はありません。カメラをオンにして練習すると、ここに残ります。')).toBeInTheDocument()
    expect(videoPlaybackURL).not.toHaveBeenCalled()
  })

  it('根拠の発話が指定されたら、その秒数の再生URLを取る', async () => {
    const { rerender } = render(
      <ReviewPlayer
        sessionId={8}
        playback={playback}
        loading={false}
        error={null}
        onRetry={() => undefined}
        seekUtteranceId={null}
        seekNonce={0}
      />,
    )
    rerender(
      <ReviewPlayer
        sessionId={8}
        playback={playback}
        loading={false}
        error={null}
        onRetry={() => undefined}
        seekUtteranceId={9}
        seekNonce={1}
      />,
    )
    expect(await screen.findByText('レポートの「録画のこのあたりを再生」を押すと、その発話のあたりから再生します。')).toBeInTheDocument()
    expect(videoPlaybackURL).toHaveBeenCalledWith(8, 4)
  })
})
