/**
 * @jest-environment jsdom
 */

/**
 * 診断チャットのサイドバーの進捗表示（#1586）。
 *
 * ここで押さえるのは、既存の sidebar-navigation / sidebar-nav テストでは
 * 検知できない2点。あちらはナビゲーション関数しか通らない。
 *
 * 1. 待機中(0%)と完了(100%)の項目から進捗表示が消えないこと
 *    進捗を undefined にして表示条件に使うと、両端で丸ごと消える
 * 2. ラベルに内部フェーズ名を出さないこと
 *    「職種分析進行中」は学生に処理の名前を読ませることになる
 *    (.claude/skills/school-career-ui-design/SKILL.md 4.3)
 */
import { render, screen, act } from '@testing-library/react'
import { AnalysisSidebar } from '@/components/AnalysisSidebar'
import type { User } from '@/lib/types'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn(), refresh: jest.fn() }),
  usePathname: () => '/',
}))

const user = { user_id: 1, name: '山田太郎', email: 'student@example.com' } as unknown as User

/** phases はチャット画面から CustomEvent で届く。 */
function sendProgress(phases: unknown[]) {
  act(() => {
    window.dispatchEvent(
      new CustomEvent('chatProgress', {
        detail: { messageCount: 1, questionCount: 4, totalQuestions: 15, phases },
      }),
    )
  })
}

function phase(name: string, asked: number, valid: number, completed: boolean) {
  return {
    phase_name: name,
    display_name: name,
    questions_asked: asked,
    valid_answers: valid,
    min_questions: 4,
    max_questions: 4,
    is_completed: completed,
  }
}

describe('AnalysisSidebar の進捗表示', () => {
  beforeEach(() => {
    global.fetch = jest.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
  })

  it('待機中と完了の項目でも進捗の文言を出す', () => {
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)

    sendProgress([
      phase('job_analysis', 4, 4, true), // 100%
      phase('interest_analysis', 2, 1, false), // 25%
      phase('aptitude_analysis', 0, 0, false), // 0%
      phase('future_analysis', 0, 0, false), // 0%
    ])

    // サイドバーはモバイル用とデスクトップ用の2つが描画されるので件数は数えない。
    // 100%：以前は progress が undefined になり、何も出なかった
    expect(screen.getAllByText('完了').length).toBeGreaterThan(0)
    // 0%：同上。2項目あるので最低2件
    expect(screen.getAllByText('これから').length).toBeGreaterThanOrEqual(2)
    // 進行中は割合を出す
    expect(screen.getAllByText('25% 完了').length).toBeGreaterThan(0)
  })

  it('ラベルは話題名で、内部フェーズ名や状態語を出さない', () => {
    render(<AnalysisSidebar user={user} onLogout={jest.fn()} />)

    sendProgress([
      phase('job_analysis', 2, 1, false),
      phase('interest_analysis', 0, 0, false),
      phase('aptitude_analysis', 0, 0, false),
      phase('future_analysis', 0, 0, false),
    ])

    for (const label of ['希望の職種', '興味のあること', '得意なこと', '働き方の希望']) {
      expect(screen.getAllByText(label).length).toBeGreaterThan(0)
    }
    // 「職種分析進行中」「興味分析待機中」のような内部名＋状態は出さない
    expect(screen.queryAllByText(/分析(進行中|待機中|完了)/)).toHaveLength(0)
  })
})
