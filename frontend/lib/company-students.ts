import { companyAuthService } from '@/lib/company-auth'

export interface StudentTag {
  id: number
  tag_name: string
}

export interface StudentListItem {
  user_id: number
  name: string
  school_name: string
  target_level: string
  avatar_url: string
  certifications_acquired: string
  certifications_in_progress: string
  desired_location: string
  desired_industry_id?: number
  desired_industry_name: string
  tags: StudentTag[]
}

export interface StudentSearchResult {
  items: StudentListItem[]
  total: number
}

export interface StudentFilters {
  industryId?: number
  location?: string
  skill?: string
  tag?: string
  limit?: number
  offset?: number
}

export interface StudentDetail {
  analysis: {
    user_id: number
    integrated_profile?: unknown
    chat_summary?: {
      strengths: string
      weaknesses: string
      recommended_working_style: string
    }
    interview_reports: {
      session_id: number
      summary_text: string
      strengths_json: string
      improvements_json: string
    }[]
  }
  tags: StudentTag[]
}

function toQuery(filters: StudentFilters): string {
  const params = new URLSearchParams()
  if (filters.industryId) params.set('industry_id', String(filters.industryId))
  if (filters.location) params.set('location', filters.location)
  if (filters.skill) params.set('skill', filters.skill)
  if (filters.tag) params.set('tag', filters.tag)
  if (filters.limit) params.set('limit', String(filters.limit))
  if (filters.offset) params.set('offset', String(filters.offset))
  const q = params.toString()
  return q ? `?${q}` : ''
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
    if (res.status === 503) {
      throw new Error('セマンティック検索を利用できません。時間をおいて再度お試しください。')
    }
    if (res.status === 404) {
      throw new Error('対象の学生が見つかりません（公開されていない可能性があります）。')
    }
    throw new Error('リクエストに失敗しました')
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const companyStudentService = {
  search(filters: StudentFilters): Promise<StudentSearchResult> {
    return request<StudentSearchResult>(`/students${toQuery(filters)}`)
  },

  /** 自然文クエリで意味的に近い学生を検索する。フィルタとはAND条件で併用される。 */
  semanticSearch(query: string, filters: StudentFilters): Promise<StudentSearchResult> {
    return request<StudentSearchResult>(`/students/semantic-search${toQuery(filters)}`, {
      method: 'POST',
      body: JSON.stringify({ query }),
    })
  },

  detail(userId: number): Promise<StudentDetail> {
    return request<StudentDetail>(`/students/${userId}`)
  },

  addTag(userId: number, tagName: string): Promise<{ tags: StudentTag[] }> {
    return request<{ tags: StudentTag[] }>(`/students/${userId}/tags`, {
      method: 'POST',
      body: JSON.stringify({ tag_name: tagName }),
    })
  },

  removeTag(userId: number, tagId: number): Promise<void> {
    return request<void>(`/students/${userId}/tags/${tagId}`, { method: 'DELETE' })
  },

  /** 自社で使用中のタグ名一覧（入力候補用） */
  listTagNames(): Promise<{ items: string[] }> {
    return request<{ items: string[] }>('/tags')
  },
}
