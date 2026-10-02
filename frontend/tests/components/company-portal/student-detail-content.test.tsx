/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { StudentDetailContent } from '@/components/company-portal/StudentDetailContent'
import { companyStudentService } from '@/lib/company/students'
import { companyScoutService } from '@/lib/company/scouts'
import { companyProfileService } from '@/lib/company/profile'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: jest.fn(), push: jest.fn() }),
}))

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ id: 1, name: '担当者' }),
    ensureFreshToken: jest.fn().mockResolvedValue(undefined),
    getAuthHeaders: () => ({}),
  },
}))

jest.mock('@/lib/company/students', () => ({
  companyStudentService: {
    detail: jest.fn(),
    addTag: jest.fn(),
    removeTag: jest.fn(),
  },
}))

jest.mock('@/lib/company/scouts', () => {
  const actual = jest.requireActual<typeof import('@/lib/company/scouts')>('@/lib/company/scouts')
  return {
    ...actual,
    companyScoutService: {
      listTemplates: jest.fn().mockResolvedValue({
        items: [
          {
            id: 1,
            title: '案内',
            body: '{{学生名}}さん、{{企業名}}です',
            created_at: '',
            updated_at: '',
          },
        ],
      }),
      cooldown: jest.fn().mockResolvedValue({ user_id: 5, remaining_ms: 0, blocked: false }),
      send: jest.fn(),
    },
  }
})

jest.mock('@/lib/company/profile', () => ({
  companyProfileService: {
    get: jest.fn().mockResolvedValue({ name: 'デモ株式会社' }),
  },
}))

const detail = companyStudentService.detail as jest.Mock
const getProfile = companyProfileService.get as jest.Mock

function visibleStudent(name: string) {
  detail.mockResolvedValue({
    analysis: {
      user_id: 5,
      name,
      interview_reports: [],
    },
    tags: [],
  })
}

describe('StudentDetailContent', () => {
  beforeEach(() => {
    detail.mockReset()
  })

  it('公開済み学生の氏名を見出しとスカウト文面に使う', async () => {
    visibleStudent('山田太郎')
    render(<StudentDetailContent userId={5} />)

    expect(await screen.findByRole('heading', { name: '山田太郎' })).toBeInTheDocument()
    expect(screen.queryByText(/学生 #/)).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'スカウトする' }))

    expect(await screen.findByText('山田太郎さん向けに、定型文を差し込んで送ります。')).toBeInTheDocument()
    await waitFor(() => {
      expect(screen.getByText('山田太郎さん、デモ株式会社です')).toBeInTheDocument()
    })
    expect(companyScoutService.listTemplates).toHaveBeenCalled()
  })

  it('氏名が空のときはIDの仮名にしない', async () => {
    visibleStudent('  ')
    render(<StudentDetailContent userId={5} />)

    expect(await screen.findByRole('heading', { name: '氏名未設定' })).toBeInTheDocument()
    expect(screen.queryByText(/学生 #5/)).not.toBeInTheDocument()
  })

  it('企業名の取得に失敗したらスカウトを送れない', async () => {
    getProfile.mockRejectedValueOnce(new Error('取得に失敗しました'))
    visibleStudent('山田太郎')
    render(<StudentDetailContent userId={5} />)

    fireEvent.click(await screen.findByRole('button', { name: 'スカウトする' }))

    expect(await screen.findByText('取得に失敗しました')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'スカウトを送る' })).toBeDisabled()
    expect(screen.queryByText(/貴社/)).not.toBeInTheDocument()
    expect(companyScoutService.send).not.toHaveBeenCalled()
  })
})
