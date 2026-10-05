/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { StudentSearchContent } from '@/components/company-portal/StudentSearchContent'
import { companyStudentService } from '@/lib/company/students'

jest.mock('next/navigation', () => {
  const router = { replace: jest.fn(), push: jest.fn() }
  return { useRouter: () => router }
})

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ id: 1 }),
    ensureFreshToken: jest.fn().mockResolvedValue(undefined),
    getAuthHeaders: () => ({}),
  },
}))

jest.mock('@/lib/company/students', () => ({
  companyStudentService: {
    search: jest.fn().mockResolvedValue({ items: [], total: 0 }),
    listTagNames: jest.fn().mockResolvedValue({ items: [] }),
  },
}))

const search = companyStudentService.search as jest.Mock

describe('StudentSearchContent', () => {
  beforeEach(() => {
    search.mockClear()
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ items: [] }),
    }) as unknown as typeof fetch
  })

  it('学生を探す見出しを出す', async () => {
    render(<StudentSearchContent />)

    expect(await screen.findByRole('heading', { name: '学生を探す' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'ダッシュボードへ' })).not.toBeInTheDocument()
  })
})
