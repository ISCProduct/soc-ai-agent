import { deriveTranscriptView, progressDots, sessionWarningText, urgencyOf } from '@/app/interview/components/session/sessionDerive'
import { PHASE_VIEW, talkDisabled, turnPhaseOf } from '@/app/interview/components/session/turnPhase'

describe('deriveTranscriptView', () => {
  it('発話が無ければ空', () => {
    expect(deriveTranscriptView([], '', '')).toEqual({ currentQuestionText: '', liveAnswerText: '' })
  })
  it('読み上げ中(partialAi)を確定済みの質問より優先する', () => {
    const v = deriveTranscriptView([{ role: 'ai', text: '前' }], '途中', '')
    expect(v.currentQuestionText).toBe('途中')
  })
  it('認識中(partialUser)を優先する', () => {
    expect(deriveTranscriptView([{ role: 'ai', text: 'Q' }], '', '途中回答').liveAnswerText).toBe('途中回答')
  })
  it('次の質問が確定したら前の質問への回答を拾わない', () => {
    const v = deriveTranscriptView(
      [{ role: 'ai', text: 'Q1' }, { role: 'user', text: 'A1' }, { role: 'ai', text: 'Q2' }], '', '')
    expect(v).toEqual({ currentQuestionText: 'Q2', liveAnswerText: '' })
  })
  it('現在の質問の後のuser発話は回答として拾う', () => {
    const v = deriveTranscriptView(
      [{ role: 'ai', text: 'Q1' }, { role: 'user', text: 'A1' }, { role: 'user', text: 'A1続き' }], '', '')
    expect(v.liveAnswerText).toBe('A1続き')
  })
  it('AI発話が一度も無い間はuser発話を拾わない', () => {
    expect(deriveTranscriptView([{ role: 'user', text: 'A' }], '', '').liveAnswerText).toBe('')
  })
})

describe('urgencyOf', () => {
  it.each([[180, 'safe'], [61, 'safe'], [60, 'warning'], [31, 'warning'], [30, 'critical'], [0, 'critical']] as const)(
    '%i秒 → %s', (sec, expected) => expect(urgencyOf(sec)).toBe(expected))
})

describe('progressDots', () => {
  it('済・現在・未を並べる', () => {
    expect(progressDots(2, 4)).toEqual(['done', 'current', 'todo', 'todo'])
  })
  it('総数が0以下なら空', () => {
    expect(progressDots(1, 0)).toEqual([])
  })
})

describe('sessionWarningText', () => {
  it('非表示なら null', () => expect(sessionWarningText(false, 100)).toBeNull())
  it('残り分を切り上げる', () => expect(sessionWarningText(true, 61)).toBe('残り約2分です。'))
  it('0秒以下は終了', () => expect(sessionWarningText(true, 0)).toBe('セッションが終了しました。'))
})

describe('turnPhaseOf / talkDisabled', () => {
  const base = { isRecording: false, turnPending: false, aiSpeaking: false }
  it.each([
    [{}, 'yourTurn', '話す', false],
    [{ aiSpeaking: true }, 'listening', '質問を聞いています', true],
    [{ turnPending: true }, 'thinking', '考えています', true],
    [{ isRecording: true }, 'recording', '話し終わった', false],
    // 録音中は面接官発話中でも録音中表示を優先する（止める手段を隠さない）。押せるかは従来どおり
    [{ isRecording: true, aiSpeaking: true }, 'recording', '話し終わった', true],
  ] as const)('%j', (flags, phase, label, disabled) => {
    const f = { ...base, ...flags }
    expect(turnPhaseOf(f)).toBe(phase)
    expect(PHASE_VIEW[turnPhaseOf(f)].talkLabel).toBe(label)
    expect(talkDisabled({ ...f, isConnected: true })).toBe(disabled)
  })
  it('未接続なら押せない', () => {
    expect(talkDisabled({ ...base, isConnected: false })).toBe(true)
  })
})
