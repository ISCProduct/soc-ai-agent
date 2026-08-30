const COMPANY_AUTH_USER_KEY = 'company_user'
const COMPANY_AUTH_TOKEN_KEY = 'company_token'

// middleware と同じマージン（期限2分前に同期・リフレッシュする）(#1091, #616踏襲)
const COMPANY_TOKEN_REFRESH_MARGIN_SECONDS = 120

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

function persistCompanyToken(token: string) {
  getSessionStorage()?.setItem(COMPANY_AUTH_TOKEN_KEY, token)
}

/** JWT の exp を検証なしで読み、期限切れ（または間もなく期限切れ）か判定する */
function companyTokenNeedsRefresh(token: string): boolean {
  try {
    const payload = token.split('.')[1]
    if (!payload) return true
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
    const decoded: unknown = JSON.parse(atob(padded))
    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return true
    return exp - Date.now() / 1000 < COMPANY_TOKEN_REFRESH_MARGIN_SECONDS
  } catch {
    return true
  }
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

  // httpOnly CookieにトークンをセットしてDevTools経由の露出を防ぐ。
  // refresh_token はストレージには保存せず httpOnly Cookie のみで保持する (#1091, #616踏襲)。
  fetch('/api/company-auth/session', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      companyUserId: data.company_user_id,
      companyUserToken: data.token,
      companyRefreshToken: data.refresh_token,
    }),
  }).catch(() => {})
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

  // 企業APIリクエスト用のヘッダーを返す（X-Company-User-Token JWTのみ。
  // X-Company-User-IDはBackendのEchoCompanyAuthでは参照されずJWTのみで解決されるため送らない）
  getAuthHeaders(): Record<string, string> {
    const token = this.getStoredToken()
    if (!token) return {}
    return {
      'X-Company-User-Token': token,
    }
  },

  /**
   * httpOnly Cookie（+ middleware 自動リフレッシュ）から有効な company_user_token を取得し、
   * sessionStorage へ同期する。Backend 直叩きの前に呼ぶ (#1091, #616踏襲)。
   */
  async ensureFreshToken(): Promise<void> {
    const current = this.getStoredToken()
    if (current && !companyTokenNeedsRefresh(current)) {
      return
    }

    const res = await fetch('/api/company-auth/session', {
      method: 'GET',
      cache: 'no-store',
      credentials: 'same-origin',
    })
    if (!res.ok) {
      throw new Error('Unauthorized: セッションの有効期限が切れました。再ログインしてください。')
    }
    const data: { company_user_token?: string } = await res.json()
    if (!data.company_user_token) {
      throw new Error('Unauthorized: セッションの有効期限が切れました。再ログインしてください。')
    }
    persistCompanyToken(data.company_user_token)
  },

  logout() {
    getSessionStorage()?.removeItem(COMPANY_AUTH_USER_KEY)
    getSessionStorage()?.removeItem(COMPANY_AUTH_TOKEN_KEY)
    // httpOnly Cookieを削除（Backend側でリフレッシュトークンも失効させる）
    fetch('/api/company-auth/session', { method: 'DELETE' }).catch(() => {})
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
