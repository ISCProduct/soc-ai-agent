import { confidenceBand, needsReview, parseFlags, type DiagnosisQualityReport } from '@/lib/admin/diagnosis-quality'

const report = (over: Partial<DiagnosisQualityReport>): DiagnosisQualityReport => ({
  ID: 1,
  UserID: 7,
  SessionID: 's',
  Confidence: 90,
  FlagsJSON: '[]',
  Summary: '',
  UpdatedAt: '',
  ...over,
})

describe('diagnosis quality', () => {
  it('信頼度が60未満、またはフラグがあるものを確認対象にする', () => {
    expect(needsReview(report({ Confidence: 59 }))).toBe(true)
    expect(needsReview(report({ Confidence: 60 }))).toBe(false)
    expect(needsReview(report({ Confidence: 90, FlagsJSON: '["回答が短い"]' }))).toBe(true)
  })

  it('壊れたフラグJSONは空として扱う', () => {
    expect(parseFlags('not-json')).toEqual([])
    expect(parseFlags('{"a":1}')).toEqual([])
  })

  it('信頼度は低い・ふつう・高いの文言にする', () => {
    expect(confidenceBand(59)).toBe('低い')
    expect(confidenceBand(60)).toBe('ふつう')
    expect(confidenceBand(80)).toBe('高い')
  })
})
