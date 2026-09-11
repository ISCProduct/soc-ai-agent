/**
 * @jest-environment jsdom
 */
import {
  VAD_THRESHOLD,
  SILENCE_MS,
  MIN_RECORDING_MS,
  useHandsFreeVad,
} from '@/app/interview/hooks/useHandsFreeVad'
import {
  MEDIA_CONSTRAINTS,
  useInterviewMedia,
} from '@/app/interview/hooks/useInterviewMedia'
import { useInterviewSession } from '@/app/interview/hooks/useInterviewSession'

describe('useHandsFreeVad exports', () => {
  it('VAD 定数が既存実装と同じ値である', () => {
    expect(VAD_THRESHOLD).toBe(0.015)
    expect(SILENCE_MS).toBe(2500)
    expect(MIN_RECORDING_MS).toBe(1000)
  })

  it('useHandsFreeVad が関数として export されている', () => {
    expect(typeof useHandsFreeVad).toBe('function')
  })
})

describe('useInterviewMedia exports', () => {
  it('MEDIA_CONSTRAINTS がロビープレビューと同じ制約である', () => {
    expect(MEDIA_CONSTRAINTS).toEqual({
      audio: { sampleRate: 48000, channelCount: 1, echoCancellation: true, noiseSuppression: true },
      video: true,
    })
  })

  it('useInterviewMedia が関数として export されている', () => {
    expect(typeof useInterviewMedia).toBe('function')
  })
})

describe('useInterviewSession exports', () => {
  it('useInterviewSession が関数として export されている', () => {
    expect(typeof useInterviewSession).toBe('function')
  })
})
