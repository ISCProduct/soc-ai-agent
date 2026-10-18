/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import CompanyPortalApplicationsPage from '@/app/company-portal/applications/page'
import { companyApplicationService } from '@/lib/company/applications'

jest.mock('next/navigation', () => {
  const router = { replace: jest.fn(), push: jest.fn() }
  return { useRouter: () => router }
})

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ id: 1 }),
    fetchMe: jest.fn().mockResolvedValue({ role: 'owner', name: '担当' }),
    logout: jest.fn(),
  },
}))

jest.mock('@/lib/company/applications', () => ({
  ALLOWED_TRANSITIONS: {},
  statusLabel: (status: string) => status,
  companyApplicationService: {
    fetchDashboard: jest.fn().mockResolvedValue({ status_counts: {} }),
    list: jest.fn(),
    updateStatus: jest.fn(),
  },
}))

const list = companyApplicationService.list as jest.Mock

describe('応募管理', () => {
  it('応募が0件（絞り込みなし）のとき理由と次の操作を表示する（SOCAIAGENT-401）', async () => {
    list.mockResolvedValueOnce({ applications: [], total: 0 })

    render(<CompanyPortalApplicationsPage />)

    expect(await screen.findByText('応募はまだありません')).toBeInTheDocument()
    expect(
      screen.getByText('公開中の求人がないと応募は発生しません。求人を用意するか、学生を探してみてください。'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '求人を作る' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '学生を探す' })).toBeInTheDocument()
  })

  it('絞り込み中に0件のときは、絞り込み解除を促す文言を表示する（SOCAIAGENT-401）', async () => {
    const initialUrl = window.location.href
    window.history.pushState({}, '', '/company-portal/applications?status=選考中')
    list.mockResolvedValueOnce({ applications: [], total: 0 })

    render(<CompanyPortalApplicationsPage />)

    expect(await screen.findByText('この段階の応募はありません')).toBeInTheDocument()
    expect(screen.getByText('別の段階を選ぶと一覧が切り替わります。')).toBeInTheDocument()

    window.history.pushState({}, '', initialUrl)
  })
})
