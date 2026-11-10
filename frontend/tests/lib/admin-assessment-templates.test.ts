import { dueAtToISO, validateDraft, type TemplateDraft } from '@/lib/admin/assessment-templates'

const base = (): TemplateDraft => ({
  title: 'A校の検査',
  description: '',
  due_at: '',
  questions: [{ body: '好きだ', weight_category: '技術志向', reverse_scored: false }],
})

describe('validateDraft', () => {
  it.each([
    ['正常', (d: TemplateDraft) => d, null],
    ['タイトル空白', (d: TemplateDraft) => ({ ...d, title: '  ' }), 'タイトル'],
    ['問題0件', (d: TemplateDraft) => ({ ...d, questions: [] }), '1問以上'],
    ['問題文が空', (d: TemplateDraft) => ({ ...d, questions: [{ ...d.questions[0], body: ' ' }] }), '1問目の問題文'],
    ['カテゴリ未選択', (d: TemplateDraft) => ({ ...d, questions: [{ ...d.questions[0], weight_category: '' }] }), '1問目のカテゴリ'],
  ])('%s', (_name, mutate, want) => {
    const got = validateDraft(mutate(base()))
    if (want === null) expect(got).toBeNull()
    else expect(got).toContain(want)
  })
})

describe('dueAtToISO', () => {
  it('空は期限なし', () => expect(dueAtToISO('')).toBeUndefined())
  it('不正な値は期限なし扱い（壊れた値を送らない）', () => expect(dueAtToISO('not-a-date')).toBeUndefined())
  it('ローカル時刻をISOへ直す', () => {
    const iso = dueAtToISO('2026-11-30T23:59')
    expect(iso).toBe(new Date('2026-11-30T23:59').toISOString())
  })
})
