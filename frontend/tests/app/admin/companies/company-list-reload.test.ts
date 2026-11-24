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
  })

  it('1本が先に失敗しても、残りの取得が終わってから失敗を返す', async () => {
    let releaseSlow: () => void = () => {}
    const slow = new Promise<void>((resolve) => {
      releaseSlow = resolve
    })
    let settled = false
    const pending = runCompanyListReload('catalog', {
      companies: () => slow,
      industries: async () => {
        throw new Error('業種の取得に失敗しました')
      },
      coverage: async () => {},
    }).then(
      () => {
        settled = true
      },
      (err: unknown) => {
        settled = true
        throw err
      },
    )

    // Promise.all は先に落ちた時点で拒否する。マイクロタスクを何回か流しても、
    // 遅い取得が終わる前に settled が true になってはいけない。
    for (let i = 0; i < 20; i += 1) {
      await Promise.resolve()
    }
    expect(settled).toBe(false)

    releaseSlow()
    await expect(pending).rejects.toThrow('業種の取得に失敗しました')
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
  await runCompanyListReload(kind, {
    companies: task('companies', 80),
    industries: task('industries', 40),
    coverage: task('coverage', 40),
  })
  return { calls, maxInFlight }
}
