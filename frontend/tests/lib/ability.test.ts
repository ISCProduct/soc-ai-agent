/** @jest-environment node */

/**
 * 能力検査のクライアント側ロジック（SOCAIAGENT-433）。
 *
 * 残り時間の表示とページ送りは、時間切れの見え方に直結する。
 * 負の残り時間を出すと「まだ出せる」と誤読される。
 */
import {
  QUESTIONS_PER_PAGE,
  SECTION_LABEL,
  STATUS_DISPLAY,
  firstUnansweredPage,
  formatRemaining,
  formatTimeLimit,
  type AbilitySessionView,
} from '@/lib/ability'

const session = (answers: Record<string, number>): AbilitySessionView => ({
  session_id: 's',
  title: '能力検査',
  status: 'in_progress',
  questions: [
    { id: 1, position: 1, section: 'verbal', body: 'q1', choices: ['a', 'b'] },
    { id: 2, position: 2, section: 'verbal', body: 'q2', choices: ['a', 'b'] },
    { id: 3, position: 3, section: 'numerical', body: 'q3', choices: ['a', 'b'] },
  ],
  answers,
})

describe('formatRemaining', () => {
  it('分:秒で出す', () => {
    expect(formatRemaining(900)).toBe('15:00')
    expect(formatRemaining(61)).toBe('1:01')
    expect(formatRemaining(9)).toBe('0:09')
  })

  it('負の値は 0:00 にする（まだ出せると誤読されない）', () => {
    expect(formatRemaining(-1)).toBe('0:00')
    expect(formatRemaining(-120)).toBe('0:00')
  })

  it('端数は切り捨てる', () => {
    expect(formatRemaining(59.9)).toBe('0:59')
  })
})

describe('formatTimeLimit', () => {
  it('分で出す', () => {
    expect(formatTimeLimit(900)).toBe('15分')
    expect(formatTimeLimit(1800)).toBe('30分')
  })

  it('1分未満は秒で出す', () => {
    expect(formatTimeLimit(30)).toBe('30秒')
  })

  it('端数を丸めない（実際より長く・短く見せない）', () => {
    expect(formatTimeLimit(90)).toBe('1分30秒')
    expect(formatTimeLimit(930)).toBe('15分30秒')
  })
})

describe('firstUnansweredPage', () => {
  it('最初の未回答から始める', () => {
    expect(firstUnansweredPage(session({}))).toBe(0)
    expect(firstUnansweredPage(session({ '1': 0 }))).toBe(1)
    expect(firstUnansweredPage(session({ '1': 0, '2': 1 }))).toBe(2)
  })

  it('全部答えていれば最後の問題', () => {
    expect(firstUnansweredPage(session({ '1': 0, '2': 1, '3': 0 }))).toBe(2)
  })

  it('choice=0 を未回答として扱わない（0 は正当な選択肢）', () => {
    // '1' は 0 を選んでいる。これを未回答と誤判定すると、答えたのに先頭へ戻される。
    expect(firstUnansweredPage(session({ '1': 0 }))).toBe(1)
  })

  it('能力検査は1問ずつ出す', () => {
    expect(QUESTIONS_PER_PAGE).toBe(1)
  })
})

describe('表示用の定数', () => {
  it('状態は記号と文言の両方を持つ（色に頼らない）', () => {
    for (const key of ['not_started', 'in_progress', 'submitted', 'expired'] as const) {
      expect(STATUS_DISPLAY[key].mark).toBeTruthy()
      expect(STATUS_DISPLAY[key].label).toBeTruthy()
    }
    expect(STATUS_DISPLAY.expired.label).toBe('時間切れ')
  })

  it('section には日本語のラベルがある', () => {
    expect(SECTION_LABEL.verbal).toBe('言語')
    expect(SECTION_LABEL.numerical).toBe('非言語')
  })
})
