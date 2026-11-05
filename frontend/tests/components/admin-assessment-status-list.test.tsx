/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { StatusList, statusDisplay } from '@/app/admin/assessments/status-list'
import type { StudentAssessmentStatus, StudentStatusList } from '@/lib/admin/assessment-templates'

const row = (over: Partial<StudentAssessmentStatus>): StudentAssessmentStatus => ({
  student_id: 1,
  name: '学生A',
  status: 'not_started',
  overdue: false,
  answered_count: 0,
  question_count: 10,
  top_categories: [],
  ...over,
})

const data = (students: StudentAssessmentStatus[]): StudentStatusList => ({ template_id: 1, title: 'A校', students })

describe('StatusList', () => {
  it('Backend が返した順（期限超過 → 未受検 → 途中 → 完了）をそのまま並べる', () => {
    render(
      <StatusList
        loading={false}
        data={data([
          row({ student_id: 1, name: '超過', overdue: true }),
          row({ student_id: 2, name: '未受検' }),
          row({ student_id: 3, name: '途中', status: 'in_progress', answered_count: 4 }),
          row({ student_id: 4, name: '完了', status: 'submitted', top_categories: ['技術志向', '創造性志向'] }),
        ])}
      />,
    )
    const names = screen.getAllByRole('row').slice(1).map((r) => r.firstChild?.textContent)
    expect(names).toEqual(['超過', '未受検', '途中', '完了'])
  })

  it('状態は記号と文言の両方で示す。期限超過は受検状況より優先する', () => {
    expect(statusDisplay(row({ overdue: true }))).toEqual({ mark: '⚠', label: '期限超過（未受検）' })
    expect(statusDisplay(row({ status: 'submitted' }))).toEqual({ mark: '●', label: '提出済み' })
  })

  it('点数は出さず、提出済みだけ傾向の強い項目名を出す', () => {
    render(
      <StatusList
        loading={false}
        data={data([
          row({ student_id: 4, name: '完了', status: 'submitted', top_categories: ['技術志向', '創造性志向'] }),
          row({ student_id: 5, name: '途中', status: 'in_progress', answered_count: 3, top_categories: ['安定志向'] }),
        ])}
      />,
    )
    expect(screen.getByText('技術志向、創造性志向')).toBeInTheDocument()
    expect(screen.queryByText(/安定志向/)).not.toBeInTheDocument() // 未提出では出さない
    expect(screen.getByText('3 / 10問')).toBeInTheDocument()
    expect(screen.queryByText(/点/)).not.toBeInTheDocument()
  })

  it('学生がいなければ理由つきの Empty、読み込み中は Skeleton', () => {
    const { rerender } = render(<StatusList loading={false} data={data([])} />)
    expect(screen.getByText('対象の学生がいません')).toBeInTheDocument()
    rerender(<StatusList loading data={null} />)
    expect(screen.getByText('受検状況を読み込み中')).toBeInTheDocument()
  })
})
