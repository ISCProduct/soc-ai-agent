/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import PageContent from '@/app/scout/page-content'
import { studentScoutService } from '@/lib/scout/api'

jest.mock('next/navigation', () => {
  const router = { replace: jest.fn(), push: jest.fn(), back: jest.fn() }
  return { useRouter: () => router }
})

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ id: 1, is_guest: false }),
  },
}))

jest.mock('@/lib/scout/api', () => ({
  SCOUT_STATUS_LABEL: { sent: '未読', viewed: '既読', accepted: '承諾', declined: '辞退' },
  studentScoutService: {
    list: jest.fn(),
    view: jest.fn(),
    decline: jest.fn(),
    blockCompany: jest.fn(),
  },
}))

const list = studentScoutService.list as jest.Mock
const view = studentScoutService.view as jest.Mock
const decline = studentScoutService.decline as jest.Mock
const blockCompany = studentScoutService.blockCompany as jest.Mock

function scout(id: number, companyName: string) {
  return {
    id,
    company_id: 8,
    company_name: companyName,
    message: `${companyName}からの本文`,
    status: 'sent' as const,
    created_at: '2026-10-02T00:00:00Z',
  }
}

describe('学生のスカウト受信箱', () => {
  beforeEach(() => {
    list.mockReset()
    view.mockReset()
    decline.mockReset()
    blockCompany.mockReset()
    list.mockImplementation(async (params?: { offset?: number }) => ({
      items: [scout((params?.offset ?? 0) + 1, params?.offset ? '次の企業' : 'デモ株式会社')],
      total: 31,
      blocked_company_ids: [],
    }))
    view.mockResolvedValue({ id: 1, company_id: 8, status: 'viewed', message: '本文' })
    decline.mockResolvedValue({ id: 1, status: 'declined' })
    blockCompany.mockResolvedValue(undefined)
  })

  it('31件目以降は offset を付けて次ページを読む', async () => {
    render(<PageContent />)

    expect(await screen.findByText('デモ株式会社')).toBeInTheDocument()
    expect(list).toHaveBeenCalledWith({ limit: 30, offset: 0 })

    fireEvent.click(screen.getByRole('button', { name: '次へ' }))

    expect(await screen.findByText('次の企業')).toBeInTheDocument()
    expect(list).toHaveBeenLastCalledWith({ limit: 30, offset: 30 })
  })

  it('内容確認・辞退・ブロックは studentScoutService を通す', async () => {
    render(<PageContent />)
    expect(await screen.findByText('デモ株式会社')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '内容を確認' }))
    expect(view).toHaveBeenCalledWith(1)

    fireEvent.click(screen.getByRole('button', { name: 'このスカウトを辞退' }))
    expect(decline).toHaveBeenCalledWith(1)

    fireEvent.click(screen.getByRole('button', { name: 'この企業をブロック' }))
    fireEvent.click(await screen.findByRole('button', { name: 'ブロックする' }))
    expect(blockCompany).toHaveBeenCalledWith(8)
  })
})
