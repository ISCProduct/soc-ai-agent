/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import CompanyPortalScoutsPage from '@/app/company-portal/scouts/page'
import { companyScoutService } from '@/lib/company/scouts'

jest.mock('next/navigation', () => {
  const router = { replace: jest.fn(), push: jest.fn() }
  return { useRouter: () => router }
})

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ id: 1 }),
  },
}))

jest.mock('@/lib/company/scouts', () => ({
  SCOUT_STATUS_LABEL: { sent: '未読', viewed: '既読', accepted: '承諾', declined: '辞退' },
  companyScoutService: {
    listScouts: jest.fn(),
  },
}))

const listScouts = companyScoutService.listScouts as jest.Mock

describe('企業のスカウト送信履歴', () => {
  beforeEach(() => {
    listScouts.mockReset()
    listScouts.mockImplementation(async (params?: { offset?: number }) => ({
      items: [
        {
          id: (params?.offset ?? 0) + 1,
          user_id: 9,
          student_name: params?.offset ? '佐藤' : '山田',
          message: '本文',
          status: 'sent',
          created_at: '2026-10-02T00:00:00Z',
        },
      ],
      total: 31,
    }))
  })

  it('31件目以降は offset を付けて次ページを読む', async () => {
    render(<CompanyPortalScoutsPage />)

    expect(await screen.findByText('山田さん')).toBeInTheDocument()
    expect(listScouts).toHaveBeenCalledWith({ limit: 30, offset: 0 })

    fireEvent.click(screen.getByRole('button', { name: '次へ' }))

    expect(await screen.findByText('佐藤さん')).toBeInTheDocument()
    expect(listScouts).toHaveBeenLastCalledWith({ limit: 30, offset: 30 })
  })
})
