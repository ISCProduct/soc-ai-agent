/**
 * @jest-environment jsdom
 */
/**
 * 面接履歴の「教員モード」表示が、サーバー側の開示条件と同じ主体で出ること（#1814 #1815）。
 *
 * 以前は `user.role === 'teacher'` で判定していた。実運用では role が 'teacher' にならず
 * （職員は is_staff で表す）、教員モードも教員用詳細レポートのパネルも到達不能だった。
 * 判定を is_staff / is_admin に寄せたので、ここが壊れたら落ちるテストを置く。
 * サーバーは別途、実ロールで teacher_report_json 自体を除去する（redactTeacherReport）。
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import PageContent from '@/app/interview/history/page-content'
import { authService, type User } from '@/lib/auth'
import { interviewApi, type InterviewDetail } from '@/lib/interview'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
}))
jest.mock('@/lib/auth', () => ({
  ...jest.requireActual('@/lib/auth'),
  authService: { getStoredUser: jest.fn() },
}))
jest.mock('@/lib/interview', () => ({
  ...jest.requireActual('@/lib/interview'),
  interviewApi: { listSessions: jest.fn(), getTrend: jest.fn(), getDetail: jest.fn() },
}))
// 推移グラフと総評パネルはこのテストの対象外。描画コストと外部依存を避ける。
jest.mock('@/components/InterviewTrendChart', () => () => <div />)
jest.mock('@/app/interview/components/InterviewSummary', () => () => <div>総評</div>)

const mockUser = authService.getStoredUser as jest.MockedFunction<typeof authService.getStoredUser>
const mockList = interviewApi.listSessions as jest.MockedFunction<typeof interviewApi.listSessions>
const mockTrend = interviewApi.getTrend as jest.MockedFunction<typeof interviewApi.getTrend>
const mockDetail = interviewApi.getDetail as jest.MockedFunction<typeof interviewApi.getDetail>

const session = {
  id: 1, user_id: 1, status: 'completed', language: 'ja', estimated_cost_usd: 0,
  template_version: 'v1', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
}
const teacherReport = {
  overall_comment: '面接官視点の総評', detailed_evidence: {}, coaching_points: ['指導の着眼点'],
  strengths_for_teacher: [], next_steps: [],
}
const detail: InterviewDetail = {
  session, utterances: [],
  report: {
    session_id: 1, summary_text: '', scores_json: '{}', evidence_json: '{}',
    teacher_report_json: JSON.stringify(teacherReport),
    created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
  },
}

const baseUser: User = { user_id: 1, email: 'a@example.com', name: 'A', is_guest: false }

describe('面接履歴の教員モード', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockList.mockResolvedValue({ sessions: [session], total: 1 })
    mockTrend.mockResolvedValue([])
    mockDetail.mockResolvedValue(detail)
  })

  const tests: { name: string; user: User; wantTeacher: boolean }[] = [
    { name: '職員(is_staff)には出す', user: { ...baseUser, is_staff: true }, wantTeacher: true },
    { name: '管理者(is_admin)には出す', user: { ...baseUser, is_admin: true }, wantTeacher: true },
    { name: '学生には出さない', user: baseUser, wantTeacher: false },
    // role は旧実装の判定軸。これだけで出てしまうと、サーバーの開示条件とずれる。
    { name: "role='teacher' だけでは出さない", user: { ...baseUser, role: 'teacher' }, wantTeacher: false },
  ]

  for (const tt of tests) {
    it(tt.name, async () => {
      mockUser.mockReturnValue(tt.user)
      render(<PageContent />)

      const row = await screen.findByText('面接セッション #1')
      await act(async () => { fireEvent.click(row) })

      const chip = screen.queryByText('教員モード')
      const panel = screen.queryByText('教員用詳細レポート')
      if (tt.wantTeacher) {
        expect(chip).toBeInTheDocument()
        expect(panel).toBeInTheDocument()
        expect(screen.getByText('面接官視点の総評')).toBeInTheDocument()
        expect(screen.getByText('・指導の着眼点')).toBeInTheDocument()
      } else {
        expect(chip).not.toBeInTheDocument()
        expect(panel).not.toBeInTheDocument()
        expect(screen.queryByText('面接官視点の総評')).not.toBeInTheDocument()
      }
    })
  }
})
