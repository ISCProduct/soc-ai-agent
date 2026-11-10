import { nextQuestionIndex } from '@/app/interview/questionEvents'

describe('nextQuestionIndex', () => {
  it('advances for a question event, including follow-ups', () => {
    expect(nextQuestionIndex(1, 'topic')).toBe(2)
    expect(nextQuestionIndex(2, 'follow_up')).toBe(3)
  })

  it('does not advance when the response contains no question event', () => {
    expect(nextQuestionIndex(3, '')).toBe(3)
    expect(nextQuestionIndex(3, undefined)).toBe(3)
  })
})
