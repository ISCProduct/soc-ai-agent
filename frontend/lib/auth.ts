import { BACKEND_URL } from './backend-url'
import { extractTenantSlug } from './tenant'
import { clearChatSessionOnEnd } from '@/components/mui-chat/utils'

// middleware.tsを経由しない直接Backend fetch用に、現在のHostから学園slugヘッダーを組み立てる
function getTenantHeaders(): Record<string, string> {
  if (typeof window === 'undefined') return {}
  const slug = extractTenantSlug(window.location.hostname)
  return slug ? { 'X-Tenant-Slug': slug } : {}
}
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

// バックエンドが返す既知の英語エラーメッセージ(error_response.go)を日本語に変換する。
// 未知のメッセージはそのまま使わず、呼び出し側が渡すフォールバック文言を使う。
const ERROR_MESSAGE_JA: Record<string, string> = {
  'invalid email or password': 'メールアドレスまたはパスワードが正しくありません。',
  'guest users cannot login': 'ゲストアカウントではログインできません。',
  'tenant mismatch': 'このメールアドレスは別の組織に登録されています。',
  'email already exists': 'このメールアドレスは既に登録されています。',
  'Registration failed': '登録に失敗しました。時間をおいて再度お試しください。',
  'Invalid request body': '入力内容を確認してください。',
}

// レスポンスの JSON エラーボディ({error, code, detail})からユーザー向けメッセージを取り出す。
// JSON でない/未知のメッセージの場合はフォールバック文言(日本語)を返す。
export async function extractErrorMessage(res: Response, fallback: string): Promise<string> {
  try {
    const data: unknown = await res.json()
    const raw = (data as { error?: unknown })?.error
    if (typeof raw === 'string' && raw) {
      return ERROR_MESSAGE_JA[raw] || fallback
    }
  } catch {
    // JSON でないレスポンス
  }
  return fallback
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
  if (session) {
    const current = session.getItem(key)
    if (current) return current
  }

  // sessionStorage になければ localStorage を確認（タブ間・セッション間の継続）
  const persisted = getLocalStorage()?.getItem(key)
  if (persisted) {
    // sessionStorage にも同期しておく
    session?.setItem(key, persisted)
    return persisted
  }

  return null
}

// middleware と同じマージン（期限2分前に同期・リフレッシュする）
const USER_TOKEN_REFRESH_MARGIN_SECONDS = 120

function persistUserToken(token: string) {
  getSessionStorage()?.setItem(AUTH_USER_TOKEN_KEY, token)
  getLocalStorage()?.setItem(AUTH_USER_TOKEN_KEY, token)
}

/** JWT の exp を検証なしで読み、期限切れ（または間もなく期限切れ）か判定する */
function userTokenNeedsRefresh(token: string): boolean {
  try {
    const payload = token.split('.')[1]
    if (!payload) return true
    const normalized = payload.replace(/-/g, '+').replace(/_/g, '/')
    const padded = normalized + '='.repeat((4 - (normalized.length % 4)) % 4)
    const decoded: unknown = JSON.parse(atob(padded))
    const exp = (decoded as { exp?: unknown }).exp
    if (typeof exp !== 'number') return true
    return exp - Date.now() / 1000 < USER_TOKEN_REFRESH_MARGIN_SECONDS
  } catch {
    return true
  }
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
  refresh_token?: string
}

export const authService = {
  async login(email: string, password: string): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getTenantHeaders() },
      body: JSON.stringify({ email, password }),
    })
    if (!res.ok) {
      if (res.status === 403) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || 'email_not_verified')
      }
      throw new Error(await extractErrorMessage(res, 'ログインに失敗しました。時間をおいて再度お試しください。'))
    }
    return res.json()
  },

  async verifyRegistration(token: string): Promise<{ email: string; token: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/verify-registration?token=${encodeURIComponent(token)}`)
    if (!res.ok) {
      throw new Error(await extractErrorMessage(res, '無効または期限切れのトークンです。'))
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
      throw new Error(await extractErrorMessage(res, '確認メールの送信に失敗しました。時間をおいて再度お試しください。'))
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
      headers: { 'Content-Type': 'application/json', ...getTenantHeaders() },
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
      throw new Error(await extractErrorMessage(res, '登録に失敗しました。時間をおいて再度お試しください。'))
    }
    return res.json()
  },

  async createGuest(): Promise<AuthResponse> {
    const res = await fetch(`${BACKEND_URL}/api/auth/guest`, {
      method: 'POST',
      headers: getTenantHeaders(),
    })
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'ゲストログインに失敗しました。時間をおいて再度お試しください。'))
    return res.json()
  },

  async getUser(): Promise<User> {
    const res = await fetch(`${BACKEND_URL}/api/auth/user`, {
      headers: this.getUserFetchHeaders(),
    })
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'ユーザー情報の取得に失敗しました。'))
    return res.json()
  },

  async updateProfile(
    _userId: number,
    name: string,
    targetLevel: string,
    schoolName: string,
    certificationsAcquired: string,
    certificationsInProgress: string,
  ): Promise<AuthResponse> {
    const res = await fetch(`/api/auth/profile`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...this.getUserFetchHeaders() },
      body: JSON.stringify({
        name,
        target_level: targetLevel,
        school_name: schoolName,
        certifications_acquired: certificationsAcquired,
        certifications_in_progress: certificationsInProgress,
      }),
    })
    if (!res.ok) {
      throw new Error(await extractErrorMessage(res, 'プロフィールの更新に失敗しました。'))
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
      throw new Error(await extractErrorMessage(res, 'パスワード再設定メールの送信に失敗しました。'))
    }
  },

  async resetPassword(token: string, password: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/auth/reset-password`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token, password }),
    })
    if (!res.ok) {
      throw new Error(await extractErrorMessage(res, 'パスワードの再設定に失敗しました。'))
    }
  },

  async getGoogleAuthUrl(): Promise<{ auth_url: string; state: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/google`)
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'Google認証の開始に失敗しました。'))
    return res.json()
  },

  async getGithubAuthUrl(): Promise<{ auth_url: string; state: string }> {
    const res = await fetch(`${BACKEND_URL}/api/auth/github`)
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'GitHub認証の開始に失敗しました。'))
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
    const local = getLocalStorage()
    const userJson = JSON.stringify(user)
    // sessionStorage（タブ内）と localStorage（タブ間・再起動後）の両方に保存する
    session?.setItem(AUTH_USER_KEY, userJson)
    local?.setItem(AUTH_USER_KEY, userJson)
    if (authResponse.token) {
      session?.setItem(AUTH_TOKEN_KEY, authResponse.token)
      local?.setItem(AUTH_TOKEN_KEY, authResponse.token)
    }
    if (authResponse.user_token) {
      session?.setItem(AUTH_USER_TOKEN_KEY, authResponse.user_token)
      local?.setItem(AUTH_USER_TOKEN_KEY, authResponse.user_token)
    }

    // httpOnly CookieにトークンをセットしてDevTools経由の露出を防ぐ
    // refresh_token はストレージには保存せず httpOnly Cookie のみで保持する (#616)
    if (authResponse.user_token) {
      const userId = typeof authResponse.user_id === 'string' ? Number(authResponse.user_id) : authResponse.user_id
      fetch('/api/auth/session', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          userId,
          userToken: authResponse.user_token,
          refreshToken: authResponse.refresh_token,
        }),
      }).catch(() => {})
    }
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

  // ユーザーAPIリクエスト用のヘッダーを返す（X-User-Token JWT）
  getUserFetchHeaders(): Record<string, string> {
    const token = this.getStoredUserToken()
    return {
      'X-User-Token': token || '',
      ...getTenantHeaders(),
    }
  },

  /**
   * httpOnly Cookie（+ middleware 自動リフレッシュ）から有効な user_token を取得し、
   * sessionStorage / localStorage へ同期する。
   * Backend 直叩き（面接など）はストレージの JWT を使うため、参加前に呼ぶ。
   */
  async ensureFreshUserToken(): Promise<void> {
    const current = this.getStoredUserToken()
    if (current && !userTokenNeedsRefresh(current)) {
      return
    }

    const res = await fetch('/api/auth/session', {
      method: 'GET',
      cache: 'no-store',
      credentials: 'same-origin',
    })
    if (!res.ok) {
      throw new Error('Unauthorized: ログインの有効期限が切れました。再ログインしてください。')
    }
    const data: { user_token?: string } = await res.json()
    if (!data.user_token) {
      throw new Error('Unauthorized: ログインの有効期限が切れました。再ログインしてください。')
    }
    persistUserToken(data.user_token)
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

    if (typeof window !== 'undefined') {
      const sessionId = window.localStorage.getItem('chat_session_id')
      if (sessionId) {
        window.localStorage.removeItem(`chat_cache_${sessionId}`)
      }
      window.localStorage.removeItem('chat_session_id')
      clearChatSessionOnEnd({ sessionStorage: window.sessionStorage, localStorage: window.localStorage })
      window.localStorage.removeItem('currentSessionId')
    }

    // httpOnly Cookieを削除
    fetch('/api/auth/session', { method: 'DELETE' }).catch(() => {})
  },
}
