/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import ListContent from '@/app/assessments/page-content'
import ResultContent from '@/app/assessments/[sessionId]/result/page-content'
import { assessmentApi, type AssessmentResult, type AssessmentSummary } from '@/lib/assessments'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn() }),
  useParams: () => ({ sessionId: 's1' }),
}))
jest.mock('@/lib/auth', () => ({ authService: { getUserFetchHeaders: () => ({}) } }))
jest.mock('@/lib/results-navigation', () => ({ getResultsPathOrChat: () => '/results' }))
jest.mock('@/lib/assessments', () => ({
  ...jest.requireActual('@/lib/assessments'),
  assessmentApi: jest.fn(),
}))
jest.mock('@/components/common/BottomNavSpacer', () => ({ BottomNavSpacer: () => null }))

const api = assessmentApi as jest.Mock

const item = (over: Partial<AssessmentSummary>): AssessmentSummary => ({
  template_id: 1,
  title: '性格検査（標準）',
  description: '',
  question_count: 20,
  status: 'not_started',
  answered_count: 0,
  scope: 'standard',
  overdue: false,
  ...over,
})

describe('学生の検査一覧（学校独自）', () => {
  it('学校独自は「おすすめ企業の計算には使われない」と示し、標準には出さない', async () => {
    api.mockResolvedValue({
      assessments: [item({}), item({ template_id: 2, title: 'A校の検査', scope: 'school' })],
    })
    render(<ListContent />)
    await screen.findByText('A校の検査')
    const notes = screen.getAllByTestId('assessment-school-note')
    expect(notes).toHaveLength(1)
    expect(notes[0]).toHaveTextContent('おすすめ企業の計算には使われません')
  })

  it('期限と、超過していること（記号つき）を示す。超過でも受検は始められる', async () => {
    api.mockResolvedValue({
      assessments: [item({ template_id: 2, title: 'A校の検査', scope: 'school', due_at: '2026-10-01T00:00:00Z', overdue: true })],
    })
    render(<ListContent />)
    const due = await screen.findByTestId('assessment-due')
    expect(due).toHaveTextContent('受検期限')
    expect(due).toHaveTextContent('期限を過ぎています')
    expect(screen.getByRole('button', { name: '受検を始める' })).toBeEnabled()
  })

  it('期限がなければ期限の行は出さない', async () => {
    api.mockResolvedValue({ assessments: [item({})] })
    render(<ListContent />)
    await screen.findByText('性格検査（標準）')
    expect(screen.queryByTestId('assessment-due')).not.toBeInTheDocument()
  })
})

describe('結果画面の「この結果の使われ方」', () => {
  const result = (scope: AssessmentResult['scope']): AssessmentResult => ({
    session_id: 's1',
    title: '検査',
    scores: [{ category: '技術志向', score: 80 }],
    scope,
  })

  it('標準はマッチングに使うと説明し、マッチング結果への導線を出す', async () => {
    api.mockResolvedValue(result('standard'))
    render(<ResultContent />)
    await waitFor(() => expect(screen.getByText(/マッチングに使います/)).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'マッチング結果を見る' })).toBeInTheDocument()
  })

  it('学校独自はマッチングに使わないと説明し、マッチング結果への導線を出さない', async () => {
    api.mockResolvedValue(result('school'))
    render(<ResultContent />)
    await waitFor(() => expect(screen.getByText(/おすすめ企業の計算には使われません/)).toBeInTheDocument())
    expect(screen.queryByText(/マッチングに使います/)).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'マッチング結果を見る' })).not.toBeInTheDocument()
    expect(screen.getByText(/点数は見えません/)).toBeInTheDocument()
  })
})
