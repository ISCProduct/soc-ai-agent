import {
  SCOUT_COOLDOWN_MS,
  blockCompany,
  cooldownRemainingMs,
  createTemplate,
  emptyScoutState,
  interpolateScoutBody,
  lastScoutToStudent,
  sendScout,
} from '@/lib/scout/store'

describe('scout store', () => {
  it('学生名と企業名を差し込む', () => {
    const text = interpolateScoutBody('{{学生名}} さん / {{企業名}}', {
      studentName: '山田太郎',
      companyName: 'デモ株式会社',
    })
    expect(text).toBe('山田太郎 さん / デモ株式会社')
  })

  it('同一学生への再送は24時間クールダウンする', () => {
    const template = emptyScoutState().templates[0]
    const first = sendScout(emptyScoutState(), {
      userId: 101,
      studentName: '山田太郎',
      template,
      nowMs: 1_000_000,
    })
    if (!first.ok) throw new Error('first send should succeed')

    const second = sendScout(first.state, {
      userId: 101,
      studentName: '山田太郎',
      template,
      nowMs: 1_000_000 + SCOUT_COOLDOWN_MS - 1,
    })
    expect(second.ok).toBe(false)
    if (second.ok) return
    expect(second.reason).toBe('cooldown')

    const later = sendScout(first.state, {
      userId: 101,
      studentName: '山田太郎',
      template,
      nowMs: 1_000_000 + SCOUT_COOLDOWN_MS + 1,
    })
    expect(later.ok).toBe(true)
  })

  it('ブロック後は送信できない', () => {
    const blocked = blockCompany(emptyScoutState(), emptyScoutState().companyName)
    const result = sendScout(blocked, {
      userId: 101,
      studentName: '山田太郎',
      template: blocked.templates[0],
      nowMs: Date.now(),
    })
    expect(result.ok).toBe(false)
    if (result.ok) return
    expect(result.reason).toBe('blocked')
  })

  it('テンプレートを追加できる', () => {
    const next = createTemplate(emptyScoutState(), '追加', '本文 {{学生名}}')
    expect(next.templates.some((t) => t.title === '追加')).toBe(true)
  })

  it('直近の送信を学生ごとに取る', () => {
    const template = emptyScoutState().templates[0]
    const a = sendScout(emptyScoutState(), {
      userId: 101,
      studentName: '山田',
      template,
      nowMs: 10,
    })
    if (!a.ok) throw new Error('send')
    const last = lastScoutToStudent(a.state.scouts, 101)
    expect(last?.userId).toBe(101)
    expect(cooldownRemainingMs(last, 10)).toBe(SCOUT_COOLDOWN_MS)
  })
})
