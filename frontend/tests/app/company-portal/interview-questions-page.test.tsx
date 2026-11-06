/**
 * @jest-environment jsdom
 */
import { act, fireEvent, render, screen } from '@testing-library/react'
import CompanyPortalInterviewQuestionsPage from '@/app/company-portal/interview-questions/page'
import { companyInterviewQuestionService } from '@/lib/company/interview-questions'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ replace: jest.fn(), push: jest.fn() }),
}))

jest.mock('@/lib/company/auth', () => ({
  companyAuthService: {
    getStoredUser: () => ({ company_user_id: 1 }),
    fetchMe: jest.fn().mockResolvedValue({ role: 'owner' }),
    logout: jest.fn(),
  },
}))

jest.mock('@/lib/company/interview-questions', () => ({
  companyInterviewQuestionService: {
    list: jest.fn(),
    create: jest.fn(),
    update: jest.fn(),
    remove: jest.fn(),
  },
}))

const list = companyInterviewQuestionService.list as jest.Mock
const create = companyInterviewQuestionService.create as jest.Mock
const update = companyInterviewQuestionService.update as jest.Mock
const remove = companyInterviewQuestionService.remove as jest.Mock

describe('企業ポータルの面接質問管理', () => {
  beforeEach(() => {
    list.mockResolvedValue([])
    create.mockResolvedValue(undefined)
    update.mockResolvedValue(undefined)
    remove.mockResolvedValue(undefined)
  })

  it('ownerは面接質問を追加でき、空状態から一覧に反映される', async () => {
    const savedQuestion = {
      id: 8,
      company_id: 2,
      position: '',
      category: '志望動機',
      question_text: '当社を志望した理由を教えてください。',
      priority: 1,
      is_required: true,
    }

    render(<CompanyPortalInterviewQuestionsPage />)
    expect(await screen.findByText('面接質問がまだ登録されていません')).toBeInTheDocument()
    fireEvent.click(screen.getAllByRole('button', { name: '面接質問を追加' })[0])
    fireEvent.change(screen.getByRole('textbox', { name: /質問カテゴリ/ }), { target: { value: '志望動機' } })
    fireEvent.change(screen.getByRole('textbox', { name: /質問文/ }), {
      target: { value: '当社を志望した理由を教えてください。' },
    })

    list.mockResolvedValue([savedQuestion])
    await act(async () => {
      fireEvent.click(screen.getByRole('button', { name: '保存する' }))
    })

    expect(create).toHaveBeenCalledWith(expect.objectContaining({
      category: '志望動機',
      question_text: '当社を志望した理由を教えてください。',
    }))
    expect(await screen.findByText('当社を志望した理由を教えてください。')).toBeInTheDocument()
  })

  it('memberは質問を閲覧できるが変更操作は表示されない', async () => {
    const { companyAuthService } = jest.requireMock('@/lib/company/auth') as {
      companyAuthService: { fetchMe: jest.Mock }
    }
    companyAuthService.fetchMe.mockResolvedValueOnce({ role: 'member' })
    list.mockResolvedValueOnce([{
      id: 8,
      company_id: 2,
      position: '',
      category: '志望動機',
      question_text: '当社を志望した理由を教えてください。',
      priority: 1,
      is_required: true,
    }])

    render(<CompanyPortalInterviewQuestionsPage />)

    expect(await screen.findByText('当社を志望した理由を教えてください。')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '面接質問を追加' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '編集' })).not.toBeInTheDocument()
  })
})
