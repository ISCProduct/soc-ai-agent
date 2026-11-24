import { runCompanyListReload } from '@/app/admin/companies/company-list-reload'

describe('企業一覧の再取得', () => {
  const hang = () => new Promise<void>(() => {})

  it('公開・審査のあとは企業一覧だけ取り直す', async () => {
    const calls: string[] = []
    await runCompanyListReload('status', {
      companies: async () => {
        calls.push('companies')
      },
      industries: hang,
      coverage: hang,
    })
    expect(calls).toEqual(['companies'])
  })

  it('非公開のあとは企業一覧と業種を同時に取り直す', async () => {
    const seen = await trackParallel('reject')
    expect(seen.calls.sort()).toEqual(['companies', 'industries'])
    expect(seen.maxInFlight).toBe(2)
  })

  it('情報取得のあとは企業一覧・業種・カバレッジを同時に取り直す', async () => {
    const seen = await trackParallel('catalog')
    expect(seen.calls.sort()).toEqual(['companies', 'coverage', 'industries'])
    expect(seen.maxInFlight).toBe(3)
    // 80+40+40 の直列なら約160ms。同時なら最長の80ms前後。
    expect(seen.elapsedMs).toBeLessThan(140)
  })
})

async function trackParallel(kind: 'reject' | 'catalog') {
  const calls: string[] = []
  let inFlight = 0
  let maxInFlight = 0
  const task = (name: string, ms: number) => async () => {
    calls.push(name)
    inFlight += 1
    maxInFlight = Math.max(maxInFlight, inFlight)
    await new Promise((resolve) => setTimeout(resolve, ms))
    inFlight -= 1
  }
  const started = Date.now()
  await runCompanyListReload(kind, {
    companies: task('companies', 80),
    industries: task('industries', 40),
    coverage: task('coverage', 40),
  })
  return { calls, maxInFlight, elapsedMs: Date.now() - started }
}
