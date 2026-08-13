import {
  canUserTransition,
  isTerminalStatus,
  normalizeApplicationStatus,
  userNextStatuses,
} from '@/lib/application-status'

describe('application-status', () => {
  it('applied から accepted への直接遷移は拒否する', () => {
    expect(canUserTransition('applied', 'accepted')).toBe(false)
    expect(userNextStatuses('applied')).toEqual(['withdrawn'])
  })

  it('offered からは承諾と辞退だけ選べる', () => {
    expect(userNextStatuses('offered')).toEqual(['accepted', 'withdrawn'])
    expect(canUserTransition('offered', 'accepted')).toBe(true)
  })

  it('終了状態からの再遷移を制限する', () => {
    expect(isTerminalStatus('accepted')).toBe(true)
    expect(isTerminalStatus('declined')).toBe(true)
    expect(userNextStatuses('accepted')).toEqual([])
    expect(canUserTransition('rejected', 'applied')).toBe(false)
  })

  it('レガシー interview / declined を現行コードへ正規化する', () => {
    expect(normalizeApplicationStatus('interview')).toBe('interview_in_progress')
    expect(normalizeApplicationStatus('declined')).toBe('withdrawn')
    expect(userNextStatuses('interview')).toEqual(['withdrawn'])
  })
})
