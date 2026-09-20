/**
 * @jest-environment jsdom
 */
import {
  SILENCE_MS,
  MIN_RECORDING_MS,
  CONFIRM_MS,
  WARMUP_FRAMES,
  useHandsFreeVad,
} from '@/app/interview/hooks/useHandsFreeVad'
import {
  startThreshold,
  speechThreshold,
  NOISE_FLOOR_MIN,
} from '@/app/interview/lib/vadDecision'
import {
  MEDIA_CONSTRAINTS,
  useInterviewMedia,
} from '@/app/interview/hooks/useInterviewMedia'
import { useInterviewSession } from '@/app/interview/hooks/useInterviewSession'

describe('useHandsFreeVad exports', () => {
  it('無音・最短録音の時間は据え置き', () => {
    expect(SILENCE_MS).toBe(2500)
    expect(MIN_RECORDING_MS).toBe(1000)
  })

  // 固定しきい値 VAD_THRESHOLD (0.015) は廃止した。
  // 小声だと超えず録音が始まらず、暗騒音が高い部屋では逆に止まらなかった。
  // 現在は暗騒音に対する相対値で、開始と発話確定を別のしきい値にしている。
  it('開始しきい値は発話確定しきい値より低い（頭切れ防止）', () => {
    expect(startThreshold(NOISE_FLOOR_MIN)).toBeLessThan(speechThreshold(NOISE_FLOOR_MIN))
  })

  it('較正と確認の時間が設定されている', () => {
    expect(WARMUP_FRAMES).toBeGreaterThan(0)
    expect(CONFIRM_MS).toBeGreaterThan(0)
    expect(CONFIRM_MS).toBeLessThan(MIN_RECORDING_MS)
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
