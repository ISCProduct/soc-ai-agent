import { readdirSync, readFileSync, statSync } from 'fs'
import { join } from 'path'

/**
 * リクエストID伝播のカバレッジガード（#1188）。
 *
 * Route Handler が Backend へ fetch するとき `X-Request-ID` を転送しないと、
 * Backend 側が別のIDを採番してしまい、ブラウザに返したIDでログを追えなくなる
 * （docs/wiki/operations.md の横断追跡手順が成立しない）。
 *
 * 共通ヘルパー（extractUserAuthHeaders / adminProxyHeaders /
 * getServerUserAuthHeaders）を使えば自動的に転送される。
 * ヘッダーを自前で組んでいる Handler は転送されないので、
 * その数がこれ以上増えないことを固定する。
 */
const FORWARDING_HELPERS = [
  'extractUserAuthHeaders',
  'adminProxyHeaders',
  'getServerUserAuthHeaders',
  'X-Request-ID',
]

// 現時点で自前ヘッダーのまま残っている Handler 数。
// 減らすのは歓迎。増やす場合は共通ヘルパーを使うか、この数を意図的に更新すること。
const KNOWN_NON_FORWARDING = 64

function walk(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name)
    return statSync(path).isDirectory() ? walk(path) : path.endsWith('.ts') ? [path] : []
  })
}

function nonForwardingHandlers(): string[] {
  return walk(join(__dirname, '..', '..', 'app', 'api'))
    .filter((path) => {
      const src = readFileSync(path, 'utf8')
      return src.includes('BACKEND_URL') && !FORWARDING_HELPERS.some((h) => src.includes(h))
    })
    .sort()
}

describe('リクエストID伝播のカバレッジ', () => {
  it('X-Request-ID を転送しない Route Handler が増えていない', () => {
    const offenders = nonForwardingHandlers()
    expect(offenders.length).toBeLessThanOrEqual(KNOWN_NON_FORWARDING)
  })

  it('共通ヘルパーが X-Request-ID を転送している', () => {
    const apiProxy = readFileSync(join(__dirname, '..', '..', 'lib', 'api-proxy.ts'), 'utf8')
    const adminProxy = readFileSync(join(__dirname, '..', '..', 'lib', 'admin-backend-proxy.ts'), 'utf8')
    expect(apiProxy).toContain('X-Request-ID')
    expect(adminProxy).toContain('X-Request-ID')
  })
})
