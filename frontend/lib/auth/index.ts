import { BACKEND_URL } from '../backend-url'
import { AUTH_REFRESH_TIMEOUT_MS, fetchAndReadWithTimeout, fetchWithTimeout } from '../fetch-timeout'
import { extractTenantSlug } from '../tenant'
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

// トークンは httpOnly Cookie にだけ置く（SOCAIAGENT-683）。以前の版がストレージに残した値を消す。
// 学生の user_token は1時間で切れ、セッションの本体は以前から Cookie（refresh_token）にあるので、
// 消してもログアウトにはならない。
function clearLegacyTokens() {
  for (const storage of [getSessionStorage(), getLocalStorage()]) {
    storage?.removeItem(AUTH_TOKEN_KEY)
    storage?.removeItem(AUTH_USER_TOKEN_KEY)
  }
}

let adminTokenRefreshInFlight: Promise<void> | null = null
// 管理者トークンを確かめた時刻からこの間は問い合わせない（画面遷移のたびに往復させない）
const ADMIN_TOKEN_CHECK_INTERVAL_MS = 10 * 60 * 1000
let adminTokenCheckedAt = 0

/**
 * Cookie のログインセッションが生きているか（middleware が必要なら更新したうえで）。
 * 通信断・タイムアウトは「生きていない」とみなす。
 */
async function hasLiveSession(): Promise<boolean> {
  try {
    return await fetchAndReadWithTimeout(
      '/api/auth/session',
      { method: 'GET', cache: 'no-store', credentials: 'same-origin' },
      AUTH_REFRESH_TIMEOUT_MS,
      async (res) => res.ok,
    )
  } catch {
    return false
  }
}

/**
 * 管理者トークンの Cookie を、無い・期限が近ければ middleware に更新させる（SOCAIAGENT-476, 683）。
 * トークンの値は JS に返らない。返るのは Cookie に管理者トークンがあるかどうかだけ。
 */
async function refreshAdminTokenCookie(): Promise<void> {
  const data = await fetchAndReadWithTimeout(
    '/api/auth/session',
    {
      method: 'GET',
      cache: 'no-store',
      credentials: 'same-origin',
      headers: { 'X-Refresh-Admin-Token': '1' },
    },
    AUTH_REFRESH_TIMEOUT_MS,
    async (res): Promise<{ has_admin_token?: boolean }> => {
      if (!res.ok) {
        throw new Error('管理者セッションを更新できませんでした。再ログインしてください。')
      }
      return res.json()
    },
  )
  if (!data.has_admin_token) {
    throw new Error('管理者セッションを更新できませんでした。再ログインしてください。')
  }
}


export interface User {
  user_id: number
  email: string
  name: string
  is_guest: boolean
  role?: 'student' | 'teacher' | 'staff'
  target_level?: string
  school_name?: string
  is_admin?: boolean
  is_staff?: boolean
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
  is_staff?: boolean
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
    const res = await fetch('/api/bff/auth/login', {
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
    // POST + Next プロキシ経由。BEは POST のみ。トークンは URL に載せない（#1079）。
    const res = await fetch('/api/auth/verify-registration', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getTenantHeaders() },
      body: JSON.stringify({ token }),
    })
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
    if (res.status === 429) {
      throw new Error('送信の回数が上限に達しました。時間をおいて再度お試しください。')
    }
    if (!res.ok) {
      throw new Error(await extractErrorMessage(res, '確認メールの送信に失敗しました。時間をおいて再度お試しください。'))
    }
  },

  /**
   * 認証メールを再送する（期限切れ・未着のとき）。
   * アカウントの有無にかかわらず BE は同じ 200 を返すので、成功時は「送った」とは言い切れない文言で案内する。
   */
  async resendVerification(email: string): Promise<void> {
    const res = await fetch(`${BACKEND_URL}/api/auth/resend-verification`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', ...getTenantHeaders() },
      body: JSON.stringify({ email }),
    })
    if (res.status === 429) {
      throw new Error('再送の回数が上限に達しました。しばらく待ってからもう一度お試しください。')
    }
    if (!res.ok) {
      throw new Error('認証メールの再送に失敗しました。時間をおいて再度お試しください。')
    }
  },

  /**
   * 本登録する。
   *
   * ゲストとして診断まで進んでいる場合は、その結果を引き継ぐ。
   * 引き継がないと user_id が変わり、診断結果・マッチ結果・チャット履歴が
   * 取り残される。教員向けの一覧は本登録済みの生徒しか見ないため、
   * 取り残された結果は誰にも届かない（#1374）。
   *
   * 引き継ぎ対象は X-User-Token から特定される。promote_guest は
   * 「引き継ぐ意思」の表明で、誰を昇格するかはサーバが決める。
   */
  async register(
    email: string,
    password: string,
    name: string,
    targetLevel: string,
    certificationsAcquired: string,
    certificationsInProgress: string,
    registrationToken?: string,
  ): Promise<AuthResponse> {
    let promoteGuest = this.isGuestSession()
    if (promoteGuest) {
      // ゲストのセッションは Cookie にある。本登録は必ずメール往復を挟むので、
      // 「夜に診断 → 翌朝メールから登録」でアクセストークンは切れているが、
      // middleware が30日のリフレッシュトークンで入れ直す。リフレッシュも切れている
      // （30日超・Cookie削除など）ときはサーバ側でもゲスト行を特定できず、引き継げない。
      // 登録自体を止める理由は無いので、引き継ぎを諦めて通常登録として続ける。
      // 「ゲストは特定できるが昇格できない」場合（本登録済み・管理者等）は
      // サーバが 409 を返す。そちらを黙って新規作成に倒してはいけない。
      if (!(await hasLiveSession())) {
        promoteGuest = false
      }
    }
    const res = await fetch('/api/bff/auth/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        ...getTenantHeaders(),
        // ゲストのセッション（Cookie）は promote_guest のときだけ中継ルートが Backend へ渡す
      },
      body: JSON.stringify({
        email,
        password,
        name,
        target_level: targetLevel,
        certifications_acquired: certificationsAcquired,
        certifications_in_progress: certificationsInProgress,
        registration_token: registrationToken,
        promote_guest: promoteGuest,
      }),
    })
    if (!res.ok) {
      throw new Error(await extractErrorMessage(res, '登録に失敗しました。時間をおいて再度お試しください。'))
    }
    return res.json()
  },

  async createGuest(): Promise<AuthResponse> {
    // セッションを発行する POST は JSON 形式のみ受け付ける（他サイトからのフォーム送信を落とすため）
    const res = await fetch('/api/bff/auth/guest', {
      method: 'POST',
      headers: { ...getTenantHeaders(), 'Content-Type': 'application/json' },
      body: '{}',
    })
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'ゲストログインに失敗しました。時間をおいて再度お試しください。'))
    return res.json()
  },

  async getUser(): Promise<User> {
    const res = await fetch('/api/bff/auth/user', {
      headers: this.getUserFetchHeaders(),
    })
    if (!res.ok) throw new Error(await extractErrorMessage(res, 'ユーザー情報の取得に失敗しました。'))
    const data = await res.json()
    // is_admin 付与後に再ログインしていなくても、管理画面を開いたときに
    // ensureFreshAdminToken が Cookie を取り直す (#857)。ここでは JS に保存しない。
    return {
      user_id: typeof data.user_id === 'string' ? Number(data.user_id) : data.user_id,
      email: data.email,
      name: fixMojibake(data.name),
      is_guest: data.is_guest,
      target_level: data.target_level,
      school_name: data.school_name,
      is_admin: data.is_admin,
      is_staff: data.is_staff,
      certifications_acquired: data.certifications_acquired,
      certifications_in_progress: data.certifications_in_progress,
      oauth_provider: data.oauth_provider,
      avatar_url: data.avatar_url,
    }
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

  /**
   * ログイン結果を保存する。
   *
   * httpOnly Cookie の設定(POST /api/auth/session)まで待ってから返す。
   * 以前は投げっぱなしにしていたため、呼び出し側が直後に画面遷移すると
   * リクエストが破棄され、Cookie が設定されないままになっていた。
   * サーバーコンポーネントの requireSessionUser() は Cookie しか見ないので、
   * ログインには成功しているのに保護された画面へ入れない(#1547 で塞ぎ切れなかった残り)。
   *
   * Cookie の設定に失敗してもここでは投げない。localStorage には保存済みで、
   * 一時障害で logout() すると有効なセッションを捨てることになるため。
   * 待つことで、通信が成功している限り確実に Cookie が入る。
   */
  async saveAuth(authResponse: AuthResponse): Promise<void> {
    const user: User = {
      user_id: typeof authResponse.user_id === 'string' ? Number(authResponse.user_id) : authResponse.user_id,
      email: authResponse.email,
      name: fixMojibake(authResponse.name),
      is_guest: authResponse.is_guest,
      target_level: authResponse.target_level,
      school_name: authResponse.school_name,
      is_admin: authResponse.is_admin,
      is_staff: authResponse.is_staff,
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
    // 管理者トークンは下の POST で httpOnly Cookie にだけ入れる
    clearLegacyTokens()
    adminTokenCheckedAt = 0

    // ログイン・本登録・ゲスト作成・OAuth のコード交換は、中継ルートがすでに httpOnly Cookie を張っていて
    // トークンを返さない。トークンが届くのは OAuth の旧方式（user=）だけで、そのときだけ Cookie にする。
    // 旧方式の削除は SOCAIAGENT-707。
    if (authResponse.user_token) {
      const userId = typeof authResponse.user_id === 'string' ? Number(authResponse.user_id) : authResponse.user_id
      // 遷移で打ち切られないよう待つ。ただし無期限には待たない。
      // Backend が半開きで固まるとログイン操作が戻らず、利用者は何が
      // 起きたか分からないまま放置されるため。
      try {
        await fetchWithTimeout(
          '/api/auth/session',
          {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
              userId,
              userToken: authResponse.user_token,
              refreshToken: authResponse.refresh_token,
              ...(authResponse.token ? { adminToken: authResponse.token } : {}),
            }),
          },
          AUTH_REFRESH_TIMEOUT_MS,
        )
      } catch {
        // 通信断・タイムアウト。Cookie が張れず、次の画面でログインし直しになる。
      }
    }
  },

  /**
   * 現在の利用者がゲストかどうか。
   *
   * 本登録時に診断結果を引き継ぐかの判定に使う（#1374）。
   * セッションが生きているかは register が Cookie で確かめる。
   */
  isGuestSession(): boolean {
    return Boolean(this.getStoredUser()?.is_guest)
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

  // ユーザーAPIリクエスト用のヘッダーを返す。X-User-Token は middleware が httpOnly Cookie から付ける
  getUserFetchHeaders(): Record<string, string> {
    return getTenantHeaders()
  },

  /**
   * 管理者トークンの Cookie が無い・期限が近ければ取り直す（SOCAIAGENT-476, 683）。
   * 管理APIの X-Admin-Token は middleware が Cookie から付けるので、画面はトークンを持たない。
   */
  async ensureFreshAdminToken(): Promise<void> {
    clearLegacyTokens()
    if (Date.now() - adminTokenCheckedAt < ADMIN_TOKEN_CHECK_INTERVAL_MS) return
    if (!adminTokenRefreshInFlight) {
      adminTokenRefreshInFlight = refreshAdminTokenCookie()
        .then(() => {
          adminTokenCheckedAt = Date.now()
        })
        .finally(() => {
          adminTokenRefreshInFlight = null
        })
    }
    await adminTokenRefreshInFlight
  },

  // 管理者APIリクエスト用のヘッダーを返す。X-Admin-Token は middleware が httpOnly Cookie から付ける
  getAdminFetchHeaders(): Record<string, string> {
    const user = this.getStoredUser()
    return {
      'X-Admin-Email': user?.email || '',
    }
  },

  logout() {
    adminTokenCheckedAt = 0
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
