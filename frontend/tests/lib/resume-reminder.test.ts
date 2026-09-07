/**
 * @jest-environment jsdom
 */
import { resumeReminderMessage, fetchResumeStatus, type ResumeStatus } from '@/lib/resume-reminder'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ 'X-User-Token': 'user-jwt' }),
  },
}))

describe('resumeReminderMessage', () => {
  const cases: { name: string; status: ResumeStatus; expected: string | RegExp | null }[] = [
    {
      name: '未提出なら未作成メッセージを表示する',
      status: { has_document: false, latest_score: null, needs_attention: true },
      expected: '履歴書がまだ作成されていません',
    },
    {
      // レビューは自動生成されず学生が実行する操作なので、未実施は促す対象。
      name: 'アップ済みでレビュー未実施ならレビューを促す',
      status: { has_document: true, latest_score: null, needs_attention: true },
      expected: '履歴書のレビューをまだ受けていません',
    },
    {
      name: 'スコア59なら低スコアメッセージを表示する',
      status: { has_document: true, latest_score: 59, needs_attention: true },
      expected: /評価が低めです.*59/,
    },
    {
      name: 'スコア60（境界値）なら表示しない',
      status: { has_document: true, latest_score: 60, needs_attention: false },
      expected: null,
    },
    {
      name: 'スコア80なら表示しない',
      status: { has_document: true, latest_score: 80, needs_attention: false },
      expected: null,
    },
    {
      name: '閾値を上げた運用（例:75）でスコア70が要対応なら表示する',
      // 閾値は RESUME_COMPLETENESS_THRESHOLD で運用中に変更されうる。
      // フロントで閾値をミラーすると、この組み合わせを握り潰してしまう。
      status: { has_document: true, latest_score: 70, needs_attention: true },
      expected: /評価が低めです.*70/,
    },

  ]

  it.each(cases)('$name', ({ status, expected }) => {
    const message = resumeReminderMessage(status)
    if (expected === null) {
      expect(message).toBeNull()
    } else if (expected instanceof RegExp) {
      expect(message).toMatch(expected)
    } else {
      expect(message).toBe(expected)
    }
  })
})

describe('fetchResumeStatus', () => {
  it('ユーザー認証ヘッダーを付けて取得する', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ has_document: true, latest_score: 45, needs_attention: true }),
    })
    await expect(fetchResumeStatus()).resolves.toMatchObject({ latest_score: 45 })
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/resume/status',
      expect.objectContaining({
        cache: 'no-store',
        headers: { 'X-User-Token': 'user-jwt' },
      }),
    )
  })

  it('失敗時は例外を投げる（呼び出し側でバナーを出さない）', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false, json: async () => ({}) })
    await expect(fetchResumeStatus()).rejects.toThrow()
  })
})
