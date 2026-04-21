/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import InterviewSummary from '@/app/interview/components/InterviewSummary'
import type { InterviewReport } from '@/lib/interview'

describe('InterviewSummary', () => {
  it('面接サマリーとスコア情報を表示する', () => {
    const report: InterviewReport = {
      session_id: 1,
      summary_text: '総合的に落ち着いて回答できています。',
      scores_json: JSON.stringify({
        logic: 4.5,
        communication: 4.0,
      }),
      evidence_json: JSON.stringify({
        logic: '結論から先に説明できていました。',
      }),
      strengths_json: JSON.stringify(['結論ファーストで回答できる']),
      improvements_json: JSON.stringify(['具体例を増やす']),
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }

    render(<InterviewSummary report={report} />)

    expect(screen.getByText('総合評価')).toBeInTheDocument()
    expect(screen.getByText('総合的に落ち着いて回答できています。')).toBeInTheDocument()
    expect(screen.getByText('カテゴリ別スコア')).toBeInTheDocument()
    expect(screen.getByText('論理性')).toBeInTheDocument()
    expect(screen.getByText('4.5 / 5')).toBeInTheDocument()
    expect(screen.getByText('「結論から先に説明できていました。」')).toBeInTheDocument()
  })
})
