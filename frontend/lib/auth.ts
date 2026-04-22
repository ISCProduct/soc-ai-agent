import { BACKEND_URL } from './backend-url'
const AUTH_USER_KEY = 'user'
const AUTH_TOKEN_KEY = 'token'
const AUTH_USER_TOKEN_KEY = 'user_token'

function fixMojibake(s: string): string {
  // Detect common mojibake patterns (Ã, å followed by other Latin-1 chars)
  return /[Ãå][^\s]/.test(s) ? (() => {
    try {
      // Interpret the current UTF-16 code units as Latin-1 bytes and decode as UTF-8
      const bytes = new Uint8Array([...s].map(c => c.charCodeAt(0)))
      return new TextDecoder('utf-8').decode(bytes)
    } catch {
      try {
        // Fallback legacy method
        // @ts-ignore
        return decodeURIComponent(escape(s))
      } catch {
        return s
      }
    }
  })() : s
}

function getSessionStorage(): Storage | null {
  if (typeof window === 'undefined') return null
  return window.sessionStorage
}

function getLocalStorage(): Storage | null {
  if (typeof window === 'undefined') return null
  return window.localStorage
}

function migrateAuthValue(key: string): string | null {
  const session = getSessionStorage()
  if (!session) return null

  const current = session.getItem(key)
  if (current) return current

  const legacy = getLocalStorage()?.getItem(key)
  if (!legacy) return null

  session.setItem(key, legacy)
  getLocalStorage()?.removeItem(key)
  return legacy
}

export interface User {
  user_id: number
  email: string
  name: string
  is_guest: boolean
  role?: 'student' | 'teacher'
  target_level?: string
  school_name?: string
  is_admin?: boolean
  certifications_acquired?: string
  certifications_in_progress?: string
  oauth_provider?: string
  avatar_url?: string
}

export interface AuthResponse {
  user_id: number | string
  email: string
  name: string
  is_guest: boolean
  target_level?: string
  school_name?: string
  is_admin?: boolean
  certifications_acquired?: string
  certifications_in_progress?: string
  oauth_provider?: string
  avatar_url?: string
  token?: string
  user_token?: string
}

export const authService = {
  async login(email: string, password: string): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      if (res.status === 403) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'email_not_verified')
      }
      const error = await res.text()
      throw new Error(error || 'Login failed')
    }
    return res.json()
  },

  async verifyRegistration(token: string): Promise<{ email: string; token: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/verify-registration?token=${encodeURIComponent(token)}`)
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || '無効または期限切れのトークンです')
    }
    return res.json()
  },

  async requestRegistration(email: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/auth/request-registration`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to send registration email')
    }
  },

  async register(
    email: string,
    password: string,
    name: string,
    targetLevel: string,
    certificationsAcquired: string,
    certificationsInProgress: string,
    registrationToken?: string,
  ): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/register`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        email,
        password,
        name,
        target_level: targetLevel,
        certifications_acquired: certificationsAcquired,
        certifications_in_progress: certificationsInProgress,
        registration_token: registrationToken,
      }),
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Registration failed')
    }
    return res.json()
  },

  async createGuest(): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/guest`, {
      method: 'POST',
    })
    if (!res.ok) throw new Error('Failed to create guest user')
    return res.json()
  },

  async getUser(userId: number): Promise<User> {
    const res = await fetch(`${BACKEND_URL}/api/auth/user?user_id=${userId}`, {
      headers: this.getUserFetchHeaders(),
    })
    if (!res.ok) throw new Error('Failed to get user')
    return res.json()
  },

  async updateProfile(
    userId: number,
    name: string,
    targetLevel: string,
    schoolName: string,
    certificationsAcquired: string,
    certificationsInProgress: string,
  ): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...this.getUserFetchHeaders() },
      body: JSON.stringify({
        user_id: userId,
        name,
        target_level: targetLevel,
        school_name: schoolName,
        certifications_acquired: certificationsAcquired,
        certifications_in_progress: certificationsInProgress,
      }),
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to update profile')
    }
    return res.json()
  },

  async requestPasswordReset(email: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/auth/forgot-password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to send password reset email')
    }
  },

  async resetPassword(token: string, password: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/auth/reset-password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token, password }),
    })
    if (!res.ok) {
      const error = await res.text()
      throw new Error(error || 'Failed to reset password')
    }
  },

  async getGoogleAuthUrl(): Promise<{ auth_url: string; state: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/google`)
    if (!res.ok) throw new Error('Failed to get Google auth URL')
    return res.json()
  },

  async getGithubAuthUrl(): Promise<{ auth_url: string; state: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/github`)
    if (!res.ok) throw new Error('Failed to get GitHub auth URL')
    return res.json()
  },

  saveAuth(authResponse: AuthResponse) {
    const user: User = {
      user_id: typeof authResponse.user_id === 'string' ? Number(authResponse.user_id) : authResponse.user_id,
      email: authResponse.email,
      name: fixMojibake(authResponse.name),
      is_guest: authResponse.is_guest,
      target_level: authResponse.target_level,
      school_name: authResponse.school_name,
      is_admin: authResponse.is_admin,
      certifications_acquired: authResponse.certifications_acquired,
      certifications_in_progress: authResponse.certifications_in_progress,
      oauth_provider: authResponse.oauth_provider,
      avatar_url: authResponse.avatar_url,
    }
    const session = getSessionStorage()
    session?.setItem(AUTH_USER_KEY, JSON.stringify(user))
    if (authResponse.token) {
      session?.setItem(AUTH_TOKEN_KEY, authResponse.token)
    }
    if (authResponse.user_token) {
      session?.setItem(AUTH_USER_TOKEN_KEY, authResponse.user_token)
    }
    // 既存localStorageからの段階移行: 保存後は残存値を削除
    getLocalStorage()?.removeItem(AUTH_USER_KEY)
    getLocalStorage()?.removeItem(AUTH_TOKEN_KEY)
    getLocalStorage()?.removeItem(AUTH_USER_TOKEN_KEY)
  },

  getStoredUser(): User | null {
    const user = migrateAuthValue(AUTH_USER_KEY)
    if (!user) return null
    try {
      const parsed: User = JSON.parse(user)
      // Repair potential mojibake stored earlier
      return { ...parsed, name: fixMojibake(parsed.name) }
    } catch {
      return null
    }
  },

  getStoredToken(): string | null {
    return migrateAuthValue(AUTH_TOKEN_KEY)
  },

  getStoredUserToken(): string | null {
    return migrateAuthValue(AUTH_USER_TOKEN_KEY)
  },

  // ユーザーAPIリクエスト用のヘッダーを返す（X-User-ID + X-User-Token）
  getUserFetchHeaders(): Record<string, string> {
    const user = this.getStoredUser()
    const token = this.getStoredUserToken()
    return {
      'X-User-ID': String(user?.user_id ?? ''),
      'X-User-Token': token || '',
    }
  },

  // 管理者APIリクエスト用のヘッダーを返す（X-Admin-Email + X-Admin-Token）
  getAdminFetchHeaders(): Record<string, string> {
    const user = this.getStoredUser()
    const token = this.getStoredToken()
    return {
      'X-Admin-Email': user?.email || '',
      'X-Admin-Token': token || '',
    }
  },

  logout() {
    // ユーザー情報とトークンを削除（sessionStorage運用 + legacy localStorage清掃）
    getSessionStorage()?.removeItem(AUTH_USER_KEY)
    getSessionStorage()?.removeItem(AUTH_TOKEN_KEY)
    getSessionStorage()?.removeItem(AUTH_USER_TOKEN_KEY)
    getLocalStorage()?.removeItem(AUTH_USER_KEY)
    getLocalStorage()?.removeItem(AUTH_TOKEN_KEY)
    getLocalStorage()?.removeItem(AUTH_USER_TOKEN_KEY)
    
    // チャットキャッシュを削除
    const sessionId = localStorage.getItem('chat_session_id')
    if (sessionId) {
      localStorage.removeItem(`chat_cache_${sessionId}`)
    }
    localStorage.removeItem('chat_session_id')
  },
}
