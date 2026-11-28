/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import AxisComparison from '@/components/interview/AxisComparison'
import { interviewApi, type InterviewTrendPoint } from '@/lib/interview'

jest.mock('@/lib/interview', () => ({
  interviewApi: { compareAxes: jest.fn() },
}))

const compareAxes = interviewApi.compareAxes as jest.MockedFunction<typeof interviewApi.compareAxes>

const points: InterviewTrendPoint[] = [
  {
    session_id: 10,
    created_at: '2026-11-01T01:00:00Z',
    logic: 1,
    specificity: 2,
    ownership: 3,
    communication: 4,
    enthusiasm: 5,
  },
  {
    session_id: 11,
    created_at: '2026-11-10T01:00:00Z',
    logic: 3,
    specificity: 2,
    ownership: 1,
    communication: 4,
    enthusiasm: 5,
  },
]

describe('回ごとの比較', () => {
  beforeEach(() => {
    compareAxes.mockReset()
  })

  it('完了が2回未満のときは比較を始めない', () => {
    render(<AxisComparison points={points.slice(0, 1)} />)
    expect(screen.getByText('比較するには、完了した面接が2回以上必要です。')).toBeInTheDocument()
    expect(compareAxes).not.toHaveBeenCalled()
  })

  it('直近2回の伸びと落ちを言葉で出す', async () => {
    compareAxes.mockResolvedValue({
      base_session_id: 10,
      target_session_id: 11,
      axes: [
        { axis: 'logic', label: '論理性', base: 1, target: 3, delta: 2, direction: 'improved', change_label: '伸びた' },
        { axis: 'specificity', label: '具体性', base: 2, target: 2, delta: 0, direction: 'unchanged', change_label: '変化なし' },
        { axis: 'ownership', label: '主体性', base: 3, target: 1, delta: -2, direction: 'declined', change_label: '落ちた' },
        { axis: 'communication', label: 'コミュニケーション', base: 4, target: 4, delta: 0, direction: 'unchanged', change_label: '変化なし' },
        { axis: 'enthusiasm', label: '熱意', base: 5, target: 5, delta: 0, direction: 'unchanged', change_label: '変化なし' },
      ],
    })

    render(<AxisComparison points={points} />)

    expect(await screen.findByText('伸びた（+2）')).toBeInTheDocument()
    expect(screen.getByText('落ちた（-2）')).toBeInTheDocument()
    await waitFor(() => {
      expect(compareAxes).toHaveBeenCalledWith(10, 11)
    })
  })
})
