// 学園マルチテナント: <学園slug>.shukatsu-ai.jp のサブドメインラベルを抽出する。
// apex/www/api/stg/api-stg 等の非テナント用ホスト名は undefined を返す。
const NON_TENANT_HOSTS = new Set(['shukatsu-ai', 'www', 'api', 'stg', 'api-stg', 'admin', 'localhost'])

export function extractTenantSlug(host: string): string | undefined {
  const hostname = host.split(':')[0]
  const labels = hostname.split('.')
  if (labels.length < 2) return undefined

  const label = labels[0]
  if (NON_TENANT_HOSTS.has(label)) return undefined
  return label
}

// admin.shukatsu-ai.jp からのアクセスかどうか(管理画面サブドメイン)。
// 完全一致のみ許可する(admin.example.com や admin.foo.shukatsu-ai.jp、
// administrator.shukatsu-ai.jp のような紛らわしいホストを誤って管理画面扱いしないため)。
export function isAdminHost(host: string): boolean {
  return host.split(':')[0] === 'admin.shukatsu-ai.jp'
}
