/**
 * @jest-environment jsdom
 */
import { render, screen, within } from '@testing-library/react'
import InterviewSummary from '@/app/interview/components/InterviewSummary'
import { parseQuestionCategoryBreakdown } from '@/lib/interview/utils'
import type { InterviewReport } from '@/lib/interview'

// SOCAIAGENT-610: 質問の種類ごとの評価。

const base: InterviewReport = {
  session_id: 1,
  summary_text: '落ち着いて回答できていました。',
  scores_json: '',
  evidence_json: '',
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
}

const BREAKDOWN = [
  { category: '自己紹介・志望動機', score: 4, weakness: '', practice: '企業の事業と結びつける' },
  { category: '強み・弱み', score: 2, weakness: '弱みの改善策が抽象的', practice: '改善のために続けている行動を1つ話す' },
  { category: '逆質問', score: 3, weakness: '質問が1つだけ', practice: '3つ用意する' },
]

test('壊れた行や形の違う行は捨て、点数は 0〜5 に収める', () => {
  const got = parseQuestionCategoryBreakdown(JSON.stringify([
    { category: '逆質問', score: 7, weakness: 'x', practice: 'y' },
    { category: '', score: 3 },
    { category: '強み・弱み', score: 'a' },
    null,
    { category: '自己紹介・志望動機', score: 2.6 },
  ]))
  expect(got).toEqual([
    { category: '逆質問', score: 5, weakness: 'x', practice: 'y' },
    { category: '自己紹介・志望動機', score: 3, weakness: '', practice: '' },
  ])
  expect(parseQuestionCategoryBreakdown('壊れた JSON')).toEqual([])
  expect(parseQuestionCategoryBreakdown(null)).toEqual([])
})

test('質問の種類ごとの点数・足りなかった点・次の練習を出し、いちばん低い種類に印を付ける', () => {
  render(<InterviewSummary report={{ ...base, category_breakdown_json: JSON.stringify(BREAKDOWN) }} theme="light" />)
  const section = screen.getByRole('region', { name: '質問の種類ごとの評価' })
  const items = within(section).getAllByRole('listitem')
  expect(items).toHaveLength(3)
  expect(within(items[1]).getByText('強み・弱み')).toBeInTheDocument()
  expect(within(items[1]).getByText('2 / 5')).toBeInTheDocument()
  expect(within(items[1]).getByText('弱みの改善策が抽象的')).toBeInTheDocument()
  expect(within(items[1]).getByText('改善のために続けている行動を1つ話す')).toBeInTheDocument()
  expect(within(items[1]).getByText('次に練習したい')).toBeInTheDocument()
  expect(within(items[0]).queryByText('次に練習したい')).not.toBeInTheDocument()
  expect(within(section).getByText(/AI が回答を 0〜5 で評価した参考値/)).toBeInTheDocument()
})

test('全部同じ点なら、弱点の印は付けない', () => {
  const same = BREAKDOWN.map((b) => ({ ...b, score: 3 }))
  render(<InterviewSummary report={{ ...base, category_breakdown_json: JSON.stringify(same) }} theme="light" />)
  expect(screen.queryByText('次に練習したい')).not.toBeInTheDocument()
})

test('この機能より前のレポート（内訳なし）では、欄ごと出さない', () => {
  render(<InterviewSummary report={{ ...base, category_breakdown_json: null }} theme="light" />)
  expect(screen.queryByText('質問の種類ごとの評価')).not.toBeInTheDocument()
  expect(screen.getByText('落ち着いて回答できていました。')).toBeInTheDocument()
})
