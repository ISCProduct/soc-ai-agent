import {
  hasVoiceFallbackStages,
  isEmptyAudio,
  metaFlagTrue,
  parseAIFallback,
  shouldEnterTextContinue,
  voiceDegradeBannerText,
  voiceDegradeReason,
} from '@/app/interview/lib/aiFallback'

describe('aiFallback (#1831 テキスト継続)', () => {
  it('ai_fallback 配列を工程名として読む', () => {
    expect(parseAIFallback({ ai_fallback: ['stt', 'tts'] })).toEqual(['stt', 'tts'])
    expect(parseAIFallback({ ai_fallback: [] })).toEqual([])
    expect(parseAIFallback({})).toEqual([])
    expect(parseAIFallback({ ai_fallback: 'stt' })).toEqual([])
  })

  it('空の音声は no_audio、音声系縮退は fallback、STT 失敗は stt_failed', () => {
    expect(voiceDegradeReason(new Blob([]), [])).toBe('no_audio')
    expect(voiceDegradeReason(new Blob(['x']), ['tts'])).toBe('fallback')
    expect(voiceDegradeReason(new Blob(['x']), ['stt'])).toBe('fallback')
    expect(voiceDegradeReason(new Blob(['x']), [], { sttFailed: true })).toBe('stt_failed')
    expect(voiceDegradeReason(new Blob(['x']), [])).toBeNull()
    expect(isEmptyAudio(new Blob([]))).toBe(true)
  })

  it('llm だけの縮退では音声バナーを出さない', () => {
    expect(hasVoiceFallbackStages(['llm'])).toBe(false)
    expect(voiceDegradeReason(new Blob(['x']), ['llm'])).toBeNull()
    expect(hasVoiceFallbackStages(['stt', 'llm'])).toBe(true)
  })

  it('バナー文面は理由ごとに違う', () => {
    expect(voiceDegradeBannerText('no_audio')).toMatch(/文字で回答/)
    expect(voiceDegradeBannerText('stt_failed')).toMatch(/聞き取れません/)
    expect(voiceDegradeBannerText('fallback')).toMatch(/代替経路/)
    expect(voiceDegradeBannerText(null)).toBeNull()
  })

  it('文字回答欄は理由があるときだけ入る', () => {
    expect(shouldEnterTextContinue('no_audio')).toBe(true)
    expect(shouldEnterTextContinue('stt_failed')).toBe(true)
    expect(shouldEnterTextContinue(null)).toBe(false)
  })

  it('メタの真偽フラグを読む', () => {
    expect(metaFlagTrue({ stt_failed: true }, 'stt_failed')).toBe(true)
    expect(metaFlagTrue({ stt_failed: 'true' }, 'stt_failed')).toBe(true)
    expect(metaFlagTrue({ stt_failed: false }, 'stt_failed')).toBe(false)
    expect(metaFlagTrue({}, 'stt_failed')).toBe(false)
  })
})
