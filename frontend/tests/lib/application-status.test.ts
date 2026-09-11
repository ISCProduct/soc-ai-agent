import {
  adminNextStatuses,
  canUserTransition,
  isTerminalStatus,
  nextActionLabel,
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

describe('adminNextStatuses', () => {
  it('applied から書類選考と辞退へ進める', () => {
    expect(adminNextStatuses('applied')).toEqual(['document_screening', 'withdrawn'])
  })

  it('レガシー interview は面接中として次ステータスを返す', () => {
    expect(adminNextStatuses('interview')).toEqual(['offered', 'rejected', 'withdrawn'])
  })
})

describe('nextActionLabel', () => {
  it('選べる操作が1つなら「◯◯する」の具体的なラベルにする', () => {
    expect(nextActionLabel('applied')).toBe('辞退する')
    expect(nextActionLabel('interview_scheduled')).toBe('辞退する')
  })

  it('選べる操作が複数なら選択肢を列挙する', () => {
    expect(nextActionLabel('offered')).toBe('内定承諾・辞退を選ぶ')
  })

  it('終了状態など次の操作がない場合は空文字を返す', () => {
    expect(nextActionLabel('accepted')).toBe('')
    expect(nextActionLabel('rejected')).toBe('')
  })
})
