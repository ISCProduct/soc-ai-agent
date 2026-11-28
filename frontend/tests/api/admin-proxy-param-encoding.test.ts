import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import { NextRequest } from 'next/server'
import { proxyAdminBackend } from '@/lib/admin/backend-proxy'
import * as usersId from '@/app/api/admin/users/[id]/route'
import * as auditLogs from '@/app/api/admin/audit-logs/route'
import * as crawlRuns from '@/app/api/admin/crawl-runs/route'
import * as videoUrl from '@/app/api/admin/interviews/[id]/videos/[video_id]/url/route'
import * as costsDaily from '@/app/api/admin/costs/daily/route'
import * as costsMonthly from '@/app/api/admin/costs/monthly/route'
import * as schoolApproval from '@/app/api/admin/schools/[id]/company-approvals/[companyId]/route'

jest.mock('@/lib/admin/backend-proxy', () => ({
  ...jest.requireActual('@/lib/admin/backend-proxy'),
  proxyAdminBackend: jest.fn(),
}))

const injected = '1&role=admin'
const adminHeaders = { 'X-Admin-Email': 'a@example.com', 'X-Admin-Token': 't' }

describe('admin 系プロキシのパラメータエンコード', () => {
  beforeEach(() => {
    jest.clearAllMocks()
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))
    jest.mocked(proxyAdminBackend).mockResolvedValue({ status: 200, text: '{}' } as never)
  })
  afterEach(() => jest.restoreAllMocks())

  const fetchedUrl = () => (global.fetch as jest.Mock).mock.calls[0][0] as string
  const proxiedPath = () => jest.mocked(proxyAdminBackend).mock.calls[0][1]

  it('users/[id] は id をエンコードする', async () => {
    const req = new NextRequest('http://localhost/api/admin/users/x', { method: 'PUT', headers: adminHeaders, body: '{}' })
    await usersId.PUT(req, { params: Promise.resolve({ id: '../schools' }) })
    expect(fetchedUrl()).toMatch(/\/api\/admin\/users\/\.\.%2Fschools$/)
  })

  it('interviews/[id]/videos/[video_id]/url は両方の id をエンコードする', async () => {
    const req = new NextRequest('http://localhost/api/admin/interviews/x', { headers: adminHeaders })
    await videoUrl.GET(req, { params: Promise.resolve({ id: 'a/b', video_id: 'c?d' }) })
    expect(fetchedUrl()).toMatch(/\/api\/admin\/interviews\/a%2Fb\/videos\/c%3Fd\/url$/)
  })

  it.each([
    ['audit-logs', 'limit', auditLogs.GET],
    ['crawl-runs', 'source_id', crawlRuns.GET],
  ])('%s は %s の値をエンコードする', async (path, key, handler) => {
    await handler(new NextRequest(`http://localhost/api/admin/${path}?${key}=${encodeURIComponent(injected)}`, { headers: adminHeaders }))
    const sent = new URL(fetchedUrl())
    expect(sent.searchParams.get(key)).toBe(injected)
    expect(sent.searchParams.get('role')).toBeNull()
  })

  it.each([
    ['costs/daily', 'days', costsDaily.GET],
    ['costs/monthly', 'months', costsMonthly.GET],
  ])('%s は %s の値をエンコードする（proxyAdminBackend 経由）', async (path, key, handler) => {
    await handler(new NextRequest(`http://localhost/api/admin/${path}?${key}=${encodeURIComponent(injected)}`, { headers: adminHeaders }))
    const sent = new URL(proxiedPath(), 'http://backend')
    expect(sent.searchParams.get(key)).toBe(injected)
    expect(sent.searchParams.get('role')).toBeNull()
  })

  it('schools/[id]/company-approvals/[companyId] は両方の id をエンコードする（proxyAdminBackend 経由）', async () => {
    const req = new NextRequest('http://localhost/api/admin/schools/x', { method: 'DELETE', headers: adminHeaders })
    await schoolApproval.DELETE(req, { params: Promise.resolve({ id: 'a/b', companyId: '../c' }) })
    expect(proxiedPath()).toBe('/api/admin/schools/a%2Fb/company-approvals/..%2Fc')
  })
})

// 全 admin route を走査し、Backend URL にパスパラメータを生のまま埋め込む箇所が無いことを確かめる
describe('admin 系 route の静的検査', () => {
  const adminDir = join(__dirname, '..', '..', 'app', 'api', 'admin')
  const routeFiles = (dir: string): string[] =>
    readdirSync(dir).flatMap((name) => {
      const p = join(dir, name)
      return statSync(p).isDirectory() ? routeFiles(p) : name === 'route.ts' ? [p] : []
    })
  // 生のまま埋め込んでよいもの: 事前に許可リスト・数値で検証済み、または組み立て時にエンコード済み
  const allowed = new Set(['scope', 'scopeID', 'suffix', 'BACKEND_URL', 'ADMIN_BACKEND_URL'])

  it('`/api/admin/...` の URL に `/${変数}` を encodeURIComponent なしで埋め込まない', () => {
    const offenders = routeFiles(adminDir).flatMap((file) =>
      readFileSync(file, 'utf8')
        .split('\n')
        .filter((line) => line.includes('/api/admin/'))
        .flatMap((line) => [...line.matchAll(/\/\$\{([\w.]+)\}/g)].map((m) => m[1]))
        .filter((name) => !allowed.has(name))
        .map((name) => `${relative(adminDir, file)}: ${name}`),
    )
    expect(offenders).toEqual([])
  })
})
