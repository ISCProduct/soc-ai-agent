/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import CompanyPortalJobsPage from '@/app/company-portal/jobs/page'
import { companyJobService } from '@/lib/company/jobs'

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

jest.mock('@/lib/company/jobs', () => ({
  isPublished: (job: { data_status: string; is_active: boolean }) =>
    job.data_status === 'published' && job.is_active,
  companyJobService: {
    list: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
    setPublished: jest.fn(),
    remove: jest.fn(),
  },
}))

const list = companyJobService.list as jest.Mock
const remove = companyJobService.remove as jest.Mock

describe('求人管理', () => {
  beforeEach(() => {
    list.mockResolvedValue({
      jobs: [
        {
          id: 7,
          title: 'バックエンド',
          description: '',
          job_url: '',
          job_category_id: 0,
          min_salary: 0,
          max_salary: 0,
          employment_type: '',
          work_location: '東京',
          remote_option: false,
          required_skills: '',
          preferred_skills: '',
          data_status: 'draft',
          is_active: true,
          created_at: '',
          updated_at: '',
        },
      ],
      companyPublished: true,
    })
    remove.mockResolvedValue(undefined)
  })

  it('編集から確認したあと求人を削除する', async () => {
    render(<CompanyPortalJobsPage />)

    expect(await screen.findByText('バックエンド')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '編集' }))
    fireEvent.click(screen.getByRole('button', { name: 'この求人を削除' }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '削除する' }))
    })

    expect(remove).toHaveBeenCalledWith(7)
  })

  it('削除後の一覧再取得に失敗してもエラーを残し、消した求人は一覧から外す', async () => {
    list.mockResolvedValueOnce({
      jobs: [
        {
          id: 7,
          title: 'バックエンド',
          description: '',
          job_url: '',
          job_category_id: 0,
          min_salary: 0,
          max_salary: 0,
          employment_type: '',
          work_location: '東京',
          remote_option: false,
          required_skills: '',
          preferred_skills: '',
          data_status: 'draft',
          is_active: true,
          created_at: '',
          updated_at: '',
        },
      ],
      companyPublished: true,
    })
    list.mockRejectedValueOnce(new Error('通信エラー'))

    render(<CompanyPortalJobsPage />)

    expect(await screen.findByText('バックエンド')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '編集' }))
    fireEvent.click(screen.getByRole('button', { name: 'この求人を削除' }))
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '削除する' }))
    })

    expect(await screen.findByText(/削除は完了しています/)).toBeInTheDocument()
    expect(screen.queryByText('バックエンド')).not.toBeInTheDocument()
  })
})
