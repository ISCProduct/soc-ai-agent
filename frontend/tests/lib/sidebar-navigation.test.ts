import {
  getTodayActionLabel,
  shouldShowResultsCta,
  getApplicationsNavTarget,
} from '@/lib/sidebar-navigation'

describe('sidebar-navigation', () => {
  describe('getTodayActionLabel', () => {
    it('進捗0はチャット開始を促す', () => {
      expect(getTodayActionLabel(0)).toBe('① 自己分析チャットを始めましょう')
    })

    it('進捗80未満はチャット継続を促す', () => {
      expect(getTodayActionLabel(40)).toBe('① チャットを続けて分析を完成させましょう')
      expect(getTodayActionLabel(79)).toBe('① チャットを続けて分析を完成させましょう')
    })

    it('進捗80以上はマッチング結果確認を促す', () => {
      expect(getTodayActionLabel(80)).toBe('② マッチング結果を確認しましょう')
      expect(getTodayActionLabel(99)).toBe('② マッチング結果を確認しましょう')
    })
  })

  describe('shouldShowResultsCta', () => {
    it('80未満では CTA を出さない', () => {
      expect(shouldShowResultsCta(0)).toBe(false)
      expect(shouldShowResultsCta(79)).toBe(false)
    })

    it('80以上100未満で CTA を出す', () => {
      expect(shouldShowResultsCta(80)).toBe(true)
      expect(shouldShowResultsCta(99)).toBe(true)
    })

    it('100以上（完了）では CTA を出さない', () => {
      expect(shouldShowResultsCta(100)).toBe(false)
    })
  })

  describe('getApplicationsNavTarget', () => {
    it('通常ユーザーは選考管理へ', () => {
      expect(getApplicationsNavTarget(false)).toBe('/applications')
    })

    it('ゲストはログインへ', () => {
      expect(getApplicationsNavTarget(true)).toBe('/login')
    })
  })
})
