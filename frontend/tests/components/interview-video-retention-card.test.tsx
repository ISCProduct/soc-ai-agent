/**
 * @jest-environment jsdom
 */
import { render, screen, waitFor } from '@testing-library/react'
import { InterviewVideoRetentionCard } from '@/components/admin/InterviewVideoRetentionCard'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ is_admin: false, is_staff: true }),
    getAdminFetchHeaders: () => ({ 'X-Admin-Token': 't' }),
  },
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({
    restricted: true,
    schools: [{ id: 5, organization_id: 1, name: '情報科学', status: 'active' }],
  }),
}))

beforeEach(() => {
  global.fetch = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ interview_video_retention_days: 90 }),
  }) as unknown as typeof fetch
})

it('職員には今の期間だけ見せて、保存させない', async () => {
  render(<InterviewVideoRetentionCard />)
  expect(await screen.findByText('変更できるのは学校管理者です。')).toBeInTheDocument()
  expect(screen.queryByRole('button', { name: 'この期間で保存する' })).not.toBeInTheDocument()
  await waitFor(() => {
    expect(screen.getByLabelText('残す期間')).toHaveTextContent('90日')
  })
})
