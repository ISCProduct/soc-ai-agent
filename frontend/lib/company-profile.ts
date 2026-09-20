import { companyAuthService } from '@/lib/company-auth'

// 企業ポータルの自社プロフィール編集と担当者管理（#1322）。
//
// company_id はサーバー側がJWTから解決する。URLにもボディにも企業は指定しない。

export interface CompanyProfile {
  id: number
  name: string
  description: string
  industry: string
  location: string
  website_url: string
  logo_url: string
  founded_year: number
  employee_count: number
  culture: string
  work_style: string
  welfare_details: string
  main_business: string
  // 以下は企業からは変更できない。状態を知るためだけに返る。
  data_status: string
  is_active: boolean
  corporate_number: string
}

// 未指定の項目は送らない（サーバー側で「変更しない」と解釈される）。
// 空文字を送ると「空にする」になる。両者は別の意味を持つ。
export type ProfileUpdate = Partial<
  Pick<
    CompanyProfile,
    | 'description'
    | 'industry'
    | 'location'
    | 'website_url'
    | 'logo_url'
    | 'founded_year'
    | 'employee_count'
    | 'culture'
    | 'work_style'
    | 'welfare_details'
    | 'main_business'
  >
>

export interface CompanyMember {
  id: number
  email: string
  name: string
  role: 'owner' | 'member'
  invite_pending: boolean
  disabled: boolean
  disabled_at?: string
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
    let message = 'リクエストに失敗しました'
    try {
      const body = (await res.json()) as { message?: string; error?: string }
      message = body.message ?? body.error ?? message
    } catch {
      // JSONでない場合は既定の文面
    }
    if (res.status === 403) {
      message = 'この操作は管理者のみ行えます。'
    }
    throw new Error(message)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const companyProfileService = {
  async get(): Promise<CompanyProfile> {
    return request<CompanyProfile>('/company')
  },

  async update(input: ProfileUpdate): Promise<CompanyProfile> {
    return request<CompanyProfile>('/company', { method: 'PATCH', body: JSON.stringify(input) })
  },

  async listMembers(): Promise<CompanyMember[]> {
    const res = await request<{ items: CompanyMember[] }>('/members')
    return res.items ?? []
  },

  async invite(email: string, name: string, role: 'owner' | 'member'): Promise<CompanyMember> {
    return request<CompanyMember>('/members', {
      method: 'POST',
      body: JSON.stringify({ email, name, role }),
    })
  },

  // 物理削除は提供されない。担当者が付けた自社タグが失われるため、
  // アクセス剥奪は無効化で行う。
  async setDisabled(userID: number, disabled: boolean): Promise<CompanyMember> {
    return request<CompanyMember>(`/members/${userID}`, {
      method: 'PATCH',
      body: JSON.stringify({ disabled }),
    })
  },
}
