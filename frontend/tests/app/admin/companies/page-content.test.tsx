/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import PageContent from '@/app/admin/companies/page-content'
import { GATEWAY_USER_MESSAGE } from '@/lib/user-facing-error'

// 企業一覧画面の主な表示・操作と API 呼び出しを固定する（#2569 の分割前後で挙動が変わらないことの確認用）。
// データはすべて合成。

const access = { restricted: false }

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: access.restricted, schools: [] }),
}))

jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  // 押すと学校 7 を選んだことにする
  SchoolFilterSelect: ({ onChange }: { onChange: (id: number | undefined) => void }) => (
    <button type="button" onClick={() => onChange(7)}>
      学校フィルタ
    </button>
  ),
}))

const FETCHED = '2099-01-01T00:00:00Z'

/** 公開前で主情報がそろっている（一括公開の対象） */
const READY_DRAFT = {
  id: 1,
  name: 'そろい株式会社',
  industry: '小売',
  location: '東京都',
  data_status: 'draft',
  is_verified: false,
  info_fetched_at: FETCHED,
  relations_fetched_at: FETCHED,
  jobs_fetched_at: FETCHED,
  tech_fetched_at: FETCHED,
  website_url: 'https://example.com',
  description: '合成データ',
  tech_stack: '["Go"]',
}

/** 主情報が足りない */
const MISSING = {
  id: 2,
  name: 'たりない株式会社',
  industry: '小売',
  data_status: 'draft',
}

/** 公開中 */
const PUBLISHED = { ...READY_DRAFT, id: 3, name: 'こうかい株式会社', data_status: 'published', is_verified: true }

type Call = { url: string; method: string; body?: string }

function jsonRes(body: unknown, ok = true, status = 200) {
  const raw = JSON.stringify(body)
  return { ok, status, text: async () => raw, json: async () => JSON.parse(raw) }
}

/** HTML のエラーページ（ALB 等）。json() は例外になる */
function htmlRes(status = 500) {
  const raw = '<!doctype html><html><body>error</body></html>'
  return {
    ok: false,
    status,
    text: async () => raw,
    json: async () => {
      throw new SyntaxError('Unexpected token <')
    },
  }
}

/** 通信断 */
const networkError = () => Promise.reject(new TypeError('Failed to fetch'))

const NETWORK_MESSAGE = '通信エラーが発生しました。時間をおいて再度お試しください。'

let calls: Call[] = []

function mockFetch(handler?: (c: Call) => unknown | undefined) {
  calls = []
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
    const c: Call = { url: String(input), method: (init?.method || 'GET').toUpperCase() }
    if (typeof init?.body === 'string') c.body = init.body
    calls.push(c)
    const custom = handler?.(c)
    if (custom !== undefined) return custom
    if (c.url.startsWith('/api/admin/companies?')) {
      return jsonRes({ companies: [READY_DRAFT, MISSING, PUBLISHED], total: 3 })
    }
    if (c.url === '/api/admin/companies/industries') return jsonRes({ industries: ['小売', '製造'] })
    if (c.url === '/api/admin/companies/l1-coverage') {
      return jsonRes({ published_total: 3, info_fresh: 1, has_profile: 1, needs_warm: 2, info_rate: 0.5, profile_rate: 0.25 })
    }
    return jsonRes({})
  })
}

const listCalls = () => calls.filter((c) => c.url.startsWith('/api/admin/companies?')).map((c) => c.url)

async function renderPage() {
  render(<PageContent />)
  await screen.findByText('そろい株式会社')
  // 権限の解決後に coverage が取得されるまで待つ（プラットフォーム管理者のとき）
  if (!access.restricted) {
    await waitFor(() => expect(calls.some((c) => c.url.endsWith('/l1-coverage'))).toBe(true))
  }
}

const originalFetch = global.fetch

describe('管理画面 企業一覧', () => {
  beforeEach(() => {
    access.restricted = false
  })
  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('初回表示の API 呼び出しの順序とパラメータ', async () => {
    mockFetch()
    await renderPage()
    expect(calls.map((c) => `${c.method} ${c.url}`)).toEqual([
      'GET /api/admin/companies?limit=50&offset=0',
      'GET /api/admin/companies/industries',
      'GET /api/admin/companies/l1-coverage',
    ])
    // 権限の解決後に業界一覧を取り直さない（#2573）
    await new Promise((r) => setTimeout(r, 50))
    expect(calls.filter((c) => c.url === '/api/admin/companies/industries')).toHaveLength(1)
  })

  it('行ごとに状態・足りない情報・操作ボタンを出し分ける', async () => {
    mockFetch()
    await renderPage()
    expect(screen.getByText('企業情報の管理')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '企業を追加' })).toBeInTheDocument()
    expect(screen.getAllByText('公開前')[0]).toBeInTheDocument()
    expect(screen.getByText('学生に公開中', { selector: '.MuiChip-label' })).toBeInTheDocument()
    expect(screen.getByText('学生検索 審査済')).toBeInTheDocument()
    expect(screen.getAllByText('公開の準備ができています')).toHaveLength(2)
    expect(screen.getByText('まだ足りない情報があります')).toBeInTheDocument()
    // 足りない項目のチップは各ページへのリンクになる
    expect(screen.getAllByRole('link', { name: '会社概要' }).map((a) => a.getAttribute('href'))).toContain('/admin/companies/2/info')
    expect(screen.getByRole('button', { name: '情報を取得' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '学生に公開' })).toBeInTheDocument()
    expect(screen.getByText('このページで公開できる企業（1 社）を選択')).toBeInTheDocument()
    // 高度な設定に coverage の値が出る
    expect(screen.getByText(/公開 3 社 \/ 会社情報 50% \/ マッチング情報 25%/)).toBeInTheDocument()
  })

  it('絞り込みと検索が一覧取得のパラメータに反映される', async () => {
    mockFetch()
    await renderPage()

    fireEvent.click(screen.getByRole('button', { name: '公開前' }))
    await waitFor(() => expect(listCalls()).toContain('/api/admin/companies?limit=50&offset=0&status=draft'))

    fireEvent.click(screen.getByRole('button', { name: '足りない' }))
    await waitFor(() =>
      expect(listCalls()).toContain('/api/admin/companies?limit=50&offset=0&status=draft&readiness=missing'),
    )

    fireEvent.change(screen.getByPlaceholderText('企業名で探す'), { target: { value: ' そろい ' } })
    await waitFor(() =>
      expect(listCalls()).toContain(
        `/api/admin/companies?limit=50&offset=0&name=${encodeURIComponent('そろい')}&status=draft&readiness=missing`,
      ),
    )
    expect(screen.getByText(/「そろい」 \/ 公開前 \/ 情報が足りない の結果 3 件/)).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('業界別に分けて表示'))
    await waitFor(() => expect(listCalls().at(-1)).toContain('&order=industry'))
    expect(screen.getByText('3 社')).toBeInTheDocument()

    fireEvent.click(screen.getAllByRole('button', { name: '絞り込みを解除' })[0])
    await waitFor(() => expect(listCalls().at(-1)).toBe('/api/admin/companies?limit=50&offset=0'))
  })

  it('業界チップを押すと業界で絞り込む', async () => {
    mockFetch()
    await renderPage()
    fireEvent.click(screen.getAllByText('小売', { selector: '.MuiChip-label' })[0])
    await waitFor(() =>
      expect(listCalls().at(-1)).toBe(`/api/admin/companies?limit=50&offset=0&industry=${encodeURIComponent('小売')}`),
    )
  })

  it('学生に公開: PATCH のあと一覧だけ取り直す', async () => {
    mockFetch()
    await renderPage()
    const before = calls.length
    fireEvent.click(screen.getByRole('button', { name: '学生に公開' }))
    await screen.findByText('企業を学生に公開しました。')
    expect(calls.slice(before).map((c) => `${c.method} ${c.url}`)).toEqual([
      'PATCH /api/admin/companies/1/publish',
      'GET /api/admin/companies?limit=50&offset=0',
    ])
  })

  it('一括公開: 確認後に選択した企業を順に公開する', async () => {
    mockFetch()
    await renderPage()
    fireEvent.click(screen.getByLabelText('このページで公開できる企業（1 社）を選択'))
    expect(screen.getByText('公開できる企業 1 社を選択中')).toBeInTheDocument()
    const before = calls.length
    fireEvent.click(screen.getByRole('button', { name: '選択した 1 社を学生に公開' }))
    // 確認は MUI のダイアログ（SOCAIAGENT-534）。確定するまで公開しない
    const dialog = await screen.findByRole('dialog', { name: '企業を一括公開' })
    expect(dialog).toHaveTextContent('選択した 1 社を学生に公開しますか？')
    expect(calls.length).toBe(before)
    fireEvent.click(within(dialog).getByRole('button', { name: '公開する' }))
    await screen.findByText('1 社を学生に公開しました。')
    expect(calls.slice(before).map((c) => `${c.method} ${c.url}`)).toEqual([
      'PATCH /api/admin/companies/1/publish',
      'GET /api/admin/companies?limit=50&offset=0',
    ])
  })

  it('その他の操作メニュー: 非公開にすると一覧と業界を取り直す', async () => {
    mockFetch()
    await renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'こうかい株式会社のその他の操作' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByText('情報を取得')).toBeInTheDocument()
    expect(within(menu).getByText('最新の情報に更新')).toBeInTheDocument()
    expect(within(menu).getByText('学生検索の審査を取り消す')).toBeInTheDocument()
    const before = calls.length
    fireEvent.click(within(menu).getByText('非公開にする'))
    await screen.findByText('企業を非公開にしました。学生には表示されません。')
    expect(calls.slice(before).map((c) => `${c.method} ${c.url}`)).toEqual([
      'PATCH /api/admin/companies/3/reject',
      'GET /api/admin/companies?limit=50&offset=0',
      'GET /api/admin/companies/industries',
    ])
  })

  it('その他の操作メニュー: 審査の完了は verified を送る', async () => {
    mockFetch()
    await renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'そろい株式会社のその他の操作' }))
    const menu = await screen.findByRole('menu')
    expect(within(menu).getByText('学生に公開')).toBeInTheDocument()
    expect(within(menu).getByText('公開しない')).toBeInTheDocument()
    fireEvent.click(within(menu).getByText('学生検索の審査を完了'))
    await screen.findByText('学生検索の審査を完了しました。この企業は学生の検索とスカウト送信ができます。')
    const verify = calls.find((c) => c.url.endsWith('/verify'))
    expect(verify).toEqual({ url: '/api/admin/companies/1/verify', method: 'PATCH', body: '{"verified":true}' })
  })

  it('まとめて取得の「対象を確認」は dry_run で問い合わせて件数を出す', async () => {
    mockFetch((c) =>
      c.url === '/api/admin/companies/fetch-missing-batch'
        ? jsonRes({ candidate_n: 2, items: [{ name: 'たりない株式会社' }] })
        : undefined,
    )
    await renderPage()
    fireEvent.click(screen.getAllByRole('button', { name: '対象を確認' })[0])
    await screen.findByText(/情報が足りない企業は/)
    const batch = calls.filter((c) => c.url === '/api/admin/companies/fetch-missing-batch')
    expect(batch).toHaveLength(1)
    expect(JSON.parse(batch[0].body ?? '{}')).toEqual({ limit: 50, dry_run: true, primary_only: true, concurrency: 8 })
  })

  it('情報を取得: 1社の取得 API を呼んで一覧・業界・coverage を取り直す', async () => {
    mockFetch((c) =>
      c.url.includes('/api/admin/companies/2/')
        ? jsonRes({ info_step: { status: 'fetched' }, tech_step: { status: 'fetched' }, relations_step: { status: 'fetched' } })
        : undefined,
    )
    await renderPage()
    const before = calls.length
    fireEvent.click(screen.getByRole('button', { name: '情報を取得' }))
    await screen.findByText(/情報を取得しました（3 項目）/)
    const after = calls.slice(before).map((c) => `${c.method} ${c.url.split('?')[0]}`)
    expect(after[0]).toMatch(/^POST \/api\/admin\/companies\/2\//)
    expect(after.slice(1)).toEqual([
      'GET /api/admin/companies',
      'GET /api/admin/companies/industries',
      'GET /api/admin/companies/l1-coverage',
    ])
  })

  it('担当校管理者には公開系の操作を出さない', async () => {
    access.restricted = true
    mockFetch()
    await renderPage()
    expect(screen.queryByRole('link', { name: '企業を追加' })).not.toBeInTheDocument()
    expect(screen.getByText('3. 学生への公開はシステム管理者が行います')).toBeInTheDocument()
    expect(screen.queryByText('足りない情報をまとめて取得')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '情報を取得' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /のその他の操作$/ })).not.toBeInTheDocument()
    expect(screen.queryByText('マッチング用データの更新')).not.toBeInTheDocument()
    expect(calls.some((c) => c.url.endsWith('/l1-coverage'))).toBe(false)
  })

  it('企業が0件なら空の状態を出す', async () => {
    mockFetch((c) => (c.url.startsWith('/api/admin/companies?') ? jsonRes({ companies: [], total: 0 }) : undefined))
    render(<PageContent />)
    expect(await screen.findByText('まだ企業が登録されていません')).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: '最初の企業を追加' })).toBeInTheDocument()
  })

  describe('通信失敗・不具合の修正 (#2573)', () => {
    const openAdvanced = () => fireEvent.click(screen.getByText('高度な設定（通常は使いません）'))

    it('一覧取得が HTML のエラーページを返したら、エラーを表示する', async () => {
      mockFetch((c) => (c.url.startsWith('/api/admin/companies?') ? htmlRes(500) : undefined))
      render(<PageContent />)
      expect(await screen.findByText(GATEWAY_USER_MESSAGE)).toBeInTheDocument()
    })

    it('業界一覧の取得に失敗したら、エラーを表示する', async () => {
      mockFetch((c) =>
        c.url === '/api/admin/companies/industries' ? jsonRes({ error: '業界一覧の取得エラー（合成）' }, false, 500) : undefined,
      )
      render(<PageContent />)
      expect(await screen.findByText('業界一覧の取得エラー（合成）')).toBeInTheDocument()
    })

    it('マッチング用データの状況が通信断で取れなかったら、エラーを表示する', async () => {
      mockFetch((c) => (c.url === '/api/admin/companies/l1-coverage' ? networkError() : undefined))
      render(<PageContent />)
      expect(await screen.findByText('通信エラーが発生しました。しばらくしてから再試行してください。')).toBeInTheDocument()
    })

    it('学生に公開: 通信断ならエラーを表示する', async () => {
      mockFetch((c) => (c.url.endsWith('/publish') ? networkError() : undefined))
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: '学生に公開' }))
      expect(await screen.findByText(`公開中に${NETWORK_MESSAGE}`)).toBeInTheDocument()
    })

    it('非公開にする: 通信断ならエラーを表示する', async () => {
      mockFetch((c) => (c.url.endsWith('/reject') ? networkError() : undefined))
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: 'こうかい株式会社のその他の操作' }))
      fireEvent.click(within(await screen.findByRole('menu')).getByText('非公開にする'))
      expect(await screen.findByText(`非公開への変更中に${NETWORK_MESSAGE}`)).toBeInTheDocument()
    })

    it('学生検索の審査: 通信断ならエラーを表示する', async () => {
      mockFetch((c) => (c.url.endsWith('/verify') ? networkError() : undefined))
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: 'そろい株式会社のその他の操作' }))
      fireEvent.click(within(await screen.findByRole('menu')).getByText('学生検索の審査を完了'))
      expect(await screen.findByText(`学生検索の審査の更新中に${NETWORK_MESSAGE}`)).toBeInTheDocument()
    })

    it('初期データを用意: 通信断ならエラーを表示し、ボタンを戻す', async () => {
      mockFetch((c) => (c.url === '/api/admin/companies/seed-l1' ? networkError() : undefined))
      await renderPage()
      openAdvanced()
      fireEvent.click(screen.getByRole('button', { name: '初期データを用意' }))
      expect(await screen.findByText(`マッチング用データの準備中に${NETWORK_MESSAGE}`)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: '初期データを用意' })).not.toBeDisabled()
    })

    it('一括公開: 途中で通信断になっても成否件数を出し、成功した分だけ選択から外す', async () => {
      const READY_2 = { ...READY_DRAFT, id: 4, name: 'そろい二号株式会社' }
      mockFetch((c) => {
        if (c.url.startsWith('/api/admin/companies?')) return jsonRes({ companies: [READY_DRAFT, READY_2], total: 2 })
        if (c.url === '/api/admin/companies/4/publish') return networkError()
        return undefined
      })
      jest.spyOn(window, 'confirm').mockReturnValue(true)
      await renderPage()
      fireEvent.click(screen.getByLabelText('このページで公開できる企業（2 社）を選択'))
      fireEvent.click(screen.getByRole('button', { name: '選択した 2 社を学生に公開' }))
      expect(await screen.findByText('1 社を公開しました。1 社は公開できませんでした。')).toBeInTheDocument()
      expect(screen.getByText('公開できる企業 1 社を選択中')).toBeInTheDocument()
      expect(screen.getByRole('checkbox', { name: 'そろい二号株式会社を選択' })).toBeChecked()
    })

    it('学校を切り替えても一覧を取り直さない', async () => {
      mockFetch()
      await renderPage()
      const before = listCalls().length
      fireEvent.click(screen.getByRole('button', { name: '学校フィルタ' }))
      await waitFor(() => expect(calls.some((c) => c.url === '/api/admin/schools/7/company-approvals')).toBe(true))
      expect(listCalls()).toHaveLength(before)
    })

    it('学校の承認状態: 読み込み中と取得失敗で表示を分ける', async () => {
      let failApprovals: () => void = () => {}
      mockFetch((c) =>
        c.url === '/api/admin/schools/7/company-approvals'
          ? new Promise((resolve) => {
              failApprovals = () => resolve(jsonRes({ error: '承認状態エラー（合成）' }, false, 500))
            })
          : undefined,
      )
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: '学校フィルタ' }))
      expect((await screen.findAllByText('承認状態 読み込み中')).length).toBeGreaterThan(0)
      expect(screen.queryByText('承認状態 不明')).not.toBeInTheDocument()
      failApprovals()
      expect(await screen.findByText('承認状態エラー（合成）')).toBeInTheDocument()
      expect(screen.getAllByText('承認状態 不明').length).toBeGreaterThan(0)
      expect(screen.queryByText('承認状態 読み込み中')).not.toBeInTheDocument()
    })

    it('マッチング情報を更新: 事前確認も concurrency を送る', async () => {
      mockFetch()
      await renderPage()
      openAdvanced()
      fireEvent.click(screen.getByRole('button', { name: 'マッチング情報を更新' }))
      await waitFor(() => expect(calls.filter((c) => c.url === '/api/admin/companies/warm-l1').length).toBeGreaterThan(1))
      const first = calls.find((c) => c.url === '/api/admin/companies/warm-l1')
      expect(JSON.parse(first?.body ?? '{}')).toEqual({
        limit: 100,
        dry_run: true,
        include_info: true,
        include_persona: true,
        concurrency: 8,
      })
    })

    it('マッチング情報を更新: 事前確認が失敗したらエラーを出し、更新を始めない', async () => {
      mockFetch((c) =>
        c.url === '/api/admin/companies/warm-l1' ? jsonRes({ error: '事前確認エラー（合成）' }, false, 500) : undefined,
      )
      await renderPage()
      openAdvanced()
      fireEvent.click(screen.getByRole('button', { name: 'マッチング情報を更新' }))
      expect(await screen.findByText('事前確認エラー（合成）')).toBeInTheDocument()
      const warm = calls.filter((c) => c.url === '/api/admin/companies/warm-l1')
      expect(warm).toHaveLength(1)
      expect(JSON.parse(warm[0].body ?? '{}').dry_run).toBe(true)
      expect(screen.getByRole('button', { name: 'マッチング情報を更新' })).not.toBeDisabled()
    })
  })

  describe('操作後の取り直しでエラー表示を消さない・まとめて取得の事前確認 (SOCAIAGENT-716)', () => {
    const bodyOf = (c: Call) => JSON.parse(c.body ?? '{}') as { dry_run?: boolean }

    it('一括公開: 失敗があったら、一覧を取り直したあともエラーを残す。絞り込みを変えたら消す', async () => {
      const READY_2 = { ...READY_DRAFT, id: 4, name: 'そろい二号株式会社' }
      mockFetch((c) => {
        if (c.url.startsWith('/api/admin/companies?')) return jsonRes({ companies: [READY_DRAFT, READY_2], total: 2 })
        if (c.url === '/api/admin/companies/4/publish') return jsonRes({}, false, 500)
        return undefined
      })
      jest.spyOn(window, 'confirm').mockReturnValue(true)
      await renderPage()
      fireEvent.click(screen.getByLabelText('このページで公開できる企業（2 社）を選択'))
      const before = listCalls().length
      fireEvent.click(screen.getByRole('button', { name: '選択した 2 社を学生に公開' }))
      await screen.findByText('1 社を公開しました。1 社は公開できませんでした。')
      expect(listCalls().length).toBeGreaterThan(before)
      expect(screen.getByText('1 社の公開に失敗しました。')).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: '公開前' }))
      await waitFor(() => expect(screen.queryByText('1 社の公開に失敗しました。')).not.toBeInTheDocument())
    })

    it('情報を取得: 一部失敗のエラーを、一覧を取り直したあとも残す', async () => {
      mockFetch((c) =>
        c.url.includes('/api/admin/companies/2/fetch-primary')
          ? jsonRes({
              info_step: { status: 'error' },
              tech_step: { status: 'fetched' },
              relations_step: { status: 'fetched' },
              errors: ['合成の失敗'],
            })
          : undefined,
      )
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: '情報を取得' }))
      await screen.findByText(/情報の取得が一部失敗しました/)
      await waitFor(() => expect(calls.at(-1)?.url).toBe('/api/admin/companies/l1-coverage'))
      expect(screen.getByText('一部うまくいきませんでした: 合成の失敗')).toBeInTheDocument()
    })

    it('情報を取得: 予算超過のエラーを、一覧を取り直したあとも残す', async () => {
      mockFetch((c) =>
        c.url.includes('/api/admin/companies/2/fetch-primary')
          ? jsonRes({
              info_step: { status: 'skipped', detail: 'budget' },
              tech_step: { status: 'skipped', detail: 'budget' },
              relations_step: { status: 'skipped', detail: 'budget' },
            })
          : undefined,
      )
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: '情報を取得' }))
      await waitFor(() => expect(calls.at(-1)?.url).toBe('/api/admin/companies/l1-coverage'))
      expect(await screen.findByText(/月次の情報取得上限に達しているため/)).toBeInTheDocument()
    })

    it('まとめて取得: 失敗があったら、一覧を取り直したあともエラーを残す', async () => {
      mockFetch((c) => {
        if (c.url !== '/api/admin/companies/fetch-missing-batch') return undefined
        if (bodyOf(c).dry_run) return jsonRes({ candidate_n: 1 })
        return jsonRes({ processed: 1, errors: 1, items: [{ name: 'たりない株式会社', error: '合成の失敗' }] })
      })
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: 'まとめて取得' }))
      await screen.findByText(/取得完了: 1 社/)
      await waitFor(() => expect(calls.at(-1)?.url).toBe('/api/admin/companies/l1-coverage'))
      expect(screen.getByText('1 社とも 合成の失敗。時間をおいて再度お試しください。')).toBeInTheDocument()
    })

    it('まとめて取得: 事前確認が失敗したらエラーを出し、取得を始めない', async () => {
      mockFetch((c) =>
        c.url === '/api/admin/companies/fetch-missing-batch' ? jsonRes({ error: '事前確認エラー（合成）' }, false, 500) : undefined,
      )
      await renderPage()
      fireEvent.click(screen.getByRole('button', { name: 'まとめて取得' }))
      expect(await screen.findByText('事前確認エラー（合成）')).toBeInTheDocument()
      const batch = calls.filter((c) => c.url === '/api/admin/companies/fetch-missing-batch')
      expect(batch).toHaveLength(1)
      expect(bodyOf(batch[0]).dry_run).toBe(true)
      expect(screen.getByRole('button', { name: 'まとめて取得' })).not.toBeDisabled()
    })
  })
})
