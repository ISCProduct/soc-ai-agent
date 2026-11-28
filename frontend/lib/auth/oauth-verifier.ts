import { createHash, randomBytes } from 'node:crypto'

// OAuth ログインを始めたブラウザだけが持つ値 (SOCAIAGENT-683)。
// httpOnly Cookie に入れ、Backend へはハッシュ（bind）だけを渡す。完了時に Backend が照合する。
export const OAUTH_VERIFIER_COOKIE = 'oauth_verifier'

// Backend の OAuth state と同じ10分。プロバイダの同意画面で待つ時間を含む。
export const OAUTH_VERIFIER_COOKIE_OPTIONS = {
  httpOnly: true,
  secure: process.env.NODE_ENV === 'production',
  // プロバイダ → Backend → /auth/callback の遷移をまたぐので strict にしない
  sameSite: 'lax' as const,
  path: '/',
  maxAge: 60 * 10,
}

export function createOAuthVerifier(): string {
  return randomBytes(32).toString('base64url')
}

// Backend の authsvc.OAuthBindHash と同じ: base64url(SHA-256(verifier))、パディングなし
export function oauthBindOf(verifier: string): string {
  return createHash('sha256').update(verifier).digest('base64url')
}
