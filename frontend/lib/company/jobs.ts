import { companyAuthService } from '@/lib/company/auth'

// 企業ポータルの求人管理（#1321）。
//
// company_id はサーバー側がJWTから解決する。ボディに入れても無視される。
// 削除は提供されない（応募が紐づくため非公開化で対応）。

export interface CompanyJob {
  id: number
  title: string
  description: string
  job_url: string
  job_category_id: number
  min_salary: number
  max_salary: number
  employment_type: string
  work_location: string
  remote_option: boolean
  required_skills: string
  preferred_skills: string
  // data_status は 'draft' | 'published'。published でも企業本体が
  // 未公開なら学生には見えない。
  data_status: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface JobInput {
  title: string
  description?: string
  job_url?: string
  job_category_id?: number
  min_salary?: number
  max_salary?: number
  employment_type?: string
  work_location?: string
  remote_option?: boolean
  required_skills?: string
  preferred_skills?: string
}

export function isPublished(job: CompanyJob): boolean {
  return job.data_status === 'published' && job.is_active
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
    // サーバーのメッセージをそのまま出す。企業未公開の案内など、
    // 利用者が次に何をすべきか分かる文面が返る。
    let message = 'リクエストに失敗しました'
    try {
      const body = (await res.json()) as { message?: string; error?: string }
      message = body.message ?? body.error ?? message
    } catch {
      // JSONでない場合は既定の文面
    }
    if (res.status === 403) {
      message = '求人の管理は管理者のみ行えます。'
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export interface JobListResult {
  jobs: CompanyJob[]
  // company_published が false のとき、求人を公開しても学生に表示されない
  // 可能性がある。UI で黙らせず警告を出す（#1321）。
  companyPublished: boolean
}

export const companyJobService = {
  async list(): Promise<JobListResult> {
    const res = await request<{ jobs: CompanyJob[]; company_published: boolean }>('/jobs')
    return { jobs: res.jobs ?? [], companyPublished: res.company_published }
  },

  async create(input: JobInput): Promise<CompanyJob> {
    return request<CompanyJob>('/jobs', { method: 'POST', body: JSON.stringify(input) })
  },

  async update(id: number, input: JobInput): Promise<CompanyJob> {
    return request<CompanyJob>(`/jobs/${id}`, { method: 'PATCH', body: JSON.stringify(input) })
  },

  async setPublished(id: number, published: boolean): Promise<{ job: CompanyJob; companyPublished: boolean }> {
    const res = await request<{ job: CompanyJob; company_published: boolean }>(`/jobs/${id}/publish`, {
      method: 'POST',
      body: JSON.stringify({ published }),
    })
    return { job: res.job, companyPublished: res.company_published }
  },
}
