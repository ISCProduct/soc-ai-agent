/**
 * @jest-environment jsdom
 */
/**
 * 面接履歴の教員向け表示（#1814 #1815）。
 *
 * 以前は `user.role === 'teacher'` で判定していた。実運用では role が 'teacher' にならず
 * （職員は is_staff で表す）、教員モードも教員用詳細レポートも到達不能だった。
 *
 * 役割は2つに分かれている:
 * - 「教員モード」チップ: 保存済み利用者情報の is_staff / is_admin による表示ラベル。
 * - 教員用詳細レポートのパネル: 開示判定はサーバー側（実ロールで teacher_report_json
 *   自体を除去する。redactTeacherReport）。クライアントは返ってきたものを出すだけ。
 *   ここで保存済み利用者情報を見て再判定すると、昇格直後に localStorage が古いとき
 *   サーバーが返したレポートを隠してしまう。
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
const baseReport = {
  session_id: 1, summary_text: '', scores_json: '{}', evidence_json: '{}',
  created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
}
// 職員・管理者に対するサーバーの応答。
const detailWithTeacherReport: InterviewDetail = {
  session, utterances: [],
  report: { ...baseReport, teacher_report_json: JSON.stringify(teacherReport) },
}
// 学生に対するサーバーの応答（redactTeacherReport が空にしている）。
const detailWithoutTeacherReport: InterviewDetail = {
  session, utterances: [], report: { ...baseReport, teacher_report_json: '' },
}

const baseUser: User = { user_id: 1, email: 'a@example.com', name: 'A', is_guest: false }

describe('面接履歴の教員モード', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    mockList.mockResolvedValue({ sessions: [session], total: 1 })
    mockTrend.mockResolvedValue([])
    mockDetail.mockResolvedValue(detailWithTeacherReport)
  })

  // チップは保存済み利用者情報の is_staff / is_admin で出す。
  const chipCases: { name: string; user: User; wantChip: boolean }[] = [
    { name: '職員(is_staff)には出す', user: { ...baseUser, is_staff: true }, wantChip: true },
    { name: '管理者(is_admin)には出す', user: { ...baseUser, is_admin: true }, wantChip: true },
    { name: '学生には出さない', user: baseUser, wantChip: false },
    // role は旧実装の判定軸。これだけで出るとサーバーの開示条件とずれる。
    { name: "role='teacher' だけでは出さない", user: { ...baseUser, role: 'teacher' }, wantChip: false },
  ]

  for (const tt of chipCases) {
    it(`教員モードのチップ: ${tt.name}`, async () => {
      mockUser.mockReturnValue(tt.user)
      render(<PageContent />)
      await screen.findByText('面接セッション #1')
      if (tt.wantChip) {
        expect(screen.getByText('教員モード')).toBeInTheDocument()
      } else {
        expect(screen.queryByText('教員モード')).not.toBeInTheDocument()
      }
    })
  }

  it('サーバーが教員用レポートを返したら、保存済み利用者情報が古くても表示する', async () => {
    // 昇格直後で localStorage は学生のまま。サーバーは実ロールで職員と判定して返している。
    mockUser.mockReturnValue(baseUser)
    mockDetail.mockResolvedValue(detailWithTeacherReport)
    render(<PageContent />)
    const row = await screen.findByText('面接セッション #1')
    await act(async () => { fireEvent.click(row) })

    expect(screen.getByText('教員用詳細レポート')).toBeInTheDocument()
    expect(screen.getByText('面接官視点の総評')).toBeInTheDocument()
    expect(screen.getByText('・指導の着眼点')).toBeInTheDocument()
  })

  it('サーバーが教員用レポートを返さなければ、職員でもパネルを出さない', async () => {
    // 学生に対するサーバーの応答（teacher_report_json は除去済み）。
    mockUser.mockReturnValue({ ...baseUser, is_staff: true })
    mockDetail.mockResolvedValue(detailWithoutTeacherReport)
    render(<PageContent />)
    const row = await screen.findByText('面接セッション #1')
    await act(async () => { fireEvent.click(row) })

    expect(screen.queryByText('教員用詳細レポート')).not.toBeInTheDocument()
    expect(screen.queryByText('面接官視点の総評')).not.toBeInTheDocument()
  })
})
