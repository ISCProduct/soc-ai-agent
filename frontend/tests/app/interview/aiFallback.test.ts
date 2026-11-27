import {
  isEmptyAudio,
  parseAIFallback,
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

  it('空の音声は no_audio、縮退ありは fallback', () => {
    expect(voiceDegradeReason(new Blob([]), [])).toBe('no_audio')
    expect(voiceDegradeReason(new Blob(['x']), ['tts'])).toBe('fallback')
    expect(voiceDegradeReason(new Blob(['x']), [])).toBeNull()
    expect(isEmptyAudio(new Blob([]))).toBe(true)
  })

  it('バナー文面は理由ごとに違う', () => {
    expect(voiceDegradeBannerText('no_audio')).toMatch(/文字で回答/)
    expect(voiceDegradeBannerText('fallback')).toMatch(/代替経路/)
    expect(voiceDegradeBannerText(null)).toBeNull()
  })
})
