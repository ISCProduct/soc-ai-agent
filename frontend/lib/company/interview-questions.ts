import { companyAuthService } from '@/lib/company/auth'

export interface CompanyInterviewQuestion {
  id: number
  company_id: number
  position: string
  category: string
  question_text: string
  priority: number
  is_required: boolean
}

export interface CompanyInterviewQuestionInput {
  position: string
  category: string
  question_text: string
  priority: number
  is_required: boolean
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  await companyAuthService.ensureFreshToken()
  const res = await fetch(`/api/company-portal${path}`, {
    ...init,
    headers: {
      ...companyAuthService.getAuthHeaders(),
      ...(init?.body ? { 'Content-Type': 'application/json' } : {}),
      ...init?.headers,
    },
  })
  if (!res.ok) {
    let message = '面接質問の処理に失敗しました'
    try {
      const body = (await res.json()) as { message?: string; error?: string }
      message = body.message ?? body.error ?? message
    } catch {
      // JSONでない場合は既定の文面
    }
    if (res.status === 403) {
      message = '面接質問の編集は企業管理者のみ行えます。'
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const companyInterviewQuestionService = {
  async list(): Promise<CompanyInterviewQuestion[]> {
    const response = await request<{ questions: CompanyInterviewQuestion[] }>('/interview-questions')
    return response.questions ?? []
  },

  async create(input: CompanyInterviewQuestionInput): Promise<CompanyInterviewQuestion> {
    return request<CompanyInterviewQuestion>('/interview-questions', {
      method: 'POST',
      body: JSON.stringify(input),
    })
  },

  async update(id: number, input: CompanyInterviewQuestionInput): Promise<CompanyInterviewQuestion> {
    return request<CompanyInterviewQuestion>(`/interview-questions/${id}`, {
      method: 'PATCH',
      body: JSON.stringify(input),
    })
  },

  async remove(id: number): Promise<void> {
    await request<void>(`/interview-questions/${id}`, { method: 'DELETE' })
  },
}
