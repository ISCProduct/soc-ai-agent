import { cookies, headers } from 'next/headers'
import { redirect } from 'next/navigation'
import { SERVER_BACKEND_URL } from '@/lib/session-cookies'
import { extractTenantSlug } from '@/lib/tenant'
import type { User } from '@/lib/auth'

export interface SessionCredentials {
  userId: string
  userToken: string
}

function fixMojibake(s: string): string {
  return /[Ãå][^\s]/.test(s)
    ? (() => {
        try {
          const bytes = new Uint8Array([...s].map((c) => c.charCodeAt(0)))
          return new TextDecoder('utf-8').decode(bytes)
        } catch {
          return s
        }
      })()
    : s
}

function mapUser(data: Record<string, unknown>): User {
  const userId = data.user_id
  return {
    user_id: typeof userId === 'string' ? Number(userId) : (userId as number),
    email: String(data.email ?? ''),
    name: fixMojibake(String(data.name ?? '')),
    is_guest: Boolean(data.is_guest),
    target_level: typeof data.target_level === 'string' ? data.target_level : undefined,
    school_name: typeof data.school_name === 'string' ? data.school_name : undefined,
    is_admin: typeof data.is_admin === 'boolean' ? data.is_admin : undefined,
    certifications_acquired:
      typeof data.certifications_acquired === 'string' ? data.certifications_acquired : undefined,
    certifications_in_progress:
      typeof data.certifications_in_progress === 'string' ? data.certifications_in_progress : undefined,
    oauth_provider: typeof data.oauth_provider === 'string' ? data.oauth_provider : undefined,
    avatar_url: typeof data.avatar_url === 'string' ? data.avatar_url : undefined,
  }
}

export async function getSessionCredentials(): Promise<SessionCredentials | null> {
  const cookieStore = await cookies()
  const userId = cookieStore.get('user_id')?.value
  const userToken = cookieStore.get('user_token')?.value
  if (!userId || !userToken) return null
  return { userId, userToken }
}

export function buildUserAuthHeaders(
  creds: SessionCredentials,
  tenantSlug?: string,
): Record<string, string> {
  const authHeaders: Record<string, string> = {
    'X-User-ID': creds.userId,
    'X-User-Token': creds.userToken,
  }
  if (tenantSlug) authHeaders['X-Tenant-Slug'] = tenantSlug
  return authHeaders
}

export async function getServerUserAuthHeaders(): Promise<Record<string, string> | null> {
  const creds = await getSessionCredentials()
  if (!creds) return null
  const headerStore = await headers()
  const tenantSlug = extractTenantSlug(headerStore.get('host') ?? '')
  return buildUserAuthHeaders(creds, tenantSlug || undefined)
}

export async function getSessionUser(): Promise<User | null> {
  const authHeaders = await getServerUserAuthHeaders()
  if (!authHeaders?.['X-User-Token']) return null

  const res = await fetch(`${SERVER_BACKEND_URL}/api/auth/user`, {
    headers: authHeaders,
    cache: 'no-store',
  })
  if (!res.ok) return null
  const data: Record<string, unknown> = await res.json()
  return mapUser(data)
}

export async function requireSessionUser(): Promise<User> {
  const user = await getSessionUser()
  if (!user) redirect('/login')
  return user
}

export async function requireAdminUser(): Promise<User> {
  const user = await requireSessionUser()
  if (!user.is_admin) redirect('/')
  return user
}
