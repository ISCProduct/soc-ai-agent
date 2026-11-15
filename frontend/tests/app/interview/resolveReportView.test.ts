import {
  buildMetaLine,
  firstSentence,
  formatClock,
  formatDurationJa,
  resolveReportView,
} from '@/app/interview/components/report/resolveReportView'

const REPORT = { session_id: 1, summary_text: '', scores_json: '{}', evidence_json: '{}', created_at: '', updated_at: '' }
const view = (stalled: boolean) => ({ stage: 'generating' as const, percent: 42, stalled })

describe('resolveReportView', () => {
  it.each([
    ['idle', null, null, 'idle'],
    ['pending', null, null, 'fallback'],
    ['pending', view(false), null, 'progress'],
    ['pending', view(true), null, 'stalled'],
    ['error', null, null, 'failed'],
    ['timeout', view(false), null, 'failed'],
    ['ready', null, REPORT, 'ready'],
    ['ready', null, null, 'fallback'],
  ] as const)('%s / progress=%j / report=%j -> %s', (reportStatus, reportProgress, report, expected) => {
    expect(resolveReportView({ reportStatus, reportProgress, report })).toBe(expected)
  })
})

describe('表示用の整形', () => {
  it('秒を日本語・時計表記にする', () => {
    expect(formatDurationJa(872)).toBe('14分32秒')
    expect(formatDurationJa(32)).toBe('32秒')
    expect(formatClock(75)).toBe('1:15')
  })

  it('概要行は取れた項目だけで組み立てる', () => {
    expect(buildMetaLine({ questionCount: 8, durationSeconds: 872, companyName: '港南システムズ', positionTitle: 'SE' })).toBe('質問8問・14分32秒　港南システムズ ／ SE')
    expect(buildMetaLine({ companyName: '港南システムズ' })).toBe('港南システムズ')
    expect(buildMetaLine({ questionCount: 0, durationSeconds: 0 })).toBeNull()
  })

  it('要約の一文目だけ取り出す', () => {
    expect(firstSentence('A。B。')).toBe('A。')
    expect(firstSentence('  ')).toBeNull()
    expect(firstSentence(undefined)).toBeNull()
  })
})
