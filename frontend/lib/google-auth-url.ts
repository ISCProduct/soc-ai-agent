/** Google カレンダー連携の遷移先として許可する URL か（https かつ accounts.google.com 完全一致）。 */
export function isGoogleAuthUrl(value: unknown): value is string {
  if (typeof value !== 'string') return false
  try {
    const url = new URL(value)
    return url.protocol === 'https:' && url.hostname === 'accounts.google.com'
  } catch {
    return false
  }
}
