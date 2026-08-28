const COMPANY_AUTH_USER_KEY = 'company_user'
const COMPANY_AUTH_TOKEN_KEY = 'company_token'

function getSessionStorage(): Storage | null {
  if (typeof window === 'undefined') return null
  return window.sessionStorage
}

export interface CompanyUser {
  company_user_id: number
  company_id: number
  email: string
  name: string
  role: 'owner' | 'member'
}

export interface CompanyAuthResponse extends CompanyUser {
  token: string
  refresh_token?: string
}

function persistCompanyAuth(data: CompanyAuthResponse) {
  const session = getSessionStorage()
  if (!session) return
  session.setItem(COMPANY_AUTH_USER_KEY, JSON.stringify({
    company_user_id: data.company_user_id,
    company_id: data.company_id,
    email: data.email,
    name: data.name,
    role: data.role,
  }))
  session.setItem(COMPANY_AUTH_TOKEN_KEY, data.token)
}

export const companyAuthService = {
  getStoredUser(): CompanyUser | null {
    const raw = getSessionStorage()?.getItem(COMPANY_AUTH_USER_KEY)
    if (!raw) return null
    try {
      return JSON.parse(raw) as CompanyUser
    } catch {
      return null
    }
  },

  getStoredToken(): string | null {
    return getSessionStorage()?.getItem(COMPANY_AUTH_TOKEN_KEY) ?? null
  },

  getAuthHeaders(): Record<string, string> {
    const user = this.getStoredUser()
    const token = this.getStoredToken()
    if (!user || !token) return {}
    return {
      'X-Company-User-ID': String(user.company_user_id),
      'X-Company-User-Token': token,
    }
  },

  logout() {
    getSessionStorage()?.removeItem(COMPANY_AUTH_USER_KEY)
    getSessionStorage()?.removeItem(COMPANY_AUTH_TOKEN_KEY)
  },

  async login(email: string, password: string): Promise<CompanyAuthResponse> {
    const res = await fetch('/api/company-auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      throw new Error('ログインに失敗しました')
    }
    const data = (await res.json()) as CompanyAuthResponse
    persistCompanyAuth(data)
    return data
  },

  async acceptInvite(token: string, password: string, name?: string): Promise<CompanyAuthResponse> {
    const res = await fetch('/api/company-auth/accept-invite', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token, password, name }),
    })
    if (!res.ok) {
      throw new Error('招待の受諾に失敗しました')
    }
    const data = (await res.json()) as CompanyAuthResponse
    persistCompanyAuth(data)
    return data
  },

  async fetchMe(): Promise<CompanyAuthResponse> {
    const res = await fetch('/api/company-auth/me', {
      headers: this.getAuthHeaders(),
    })
    if (!res.ok) {
      throw new Error('セッションの取得に失敗しました')
    }
    const data = (await res.json()) as CompanyAuthResponse
    persistCompanyAuth(data)
    return data
  },
}
