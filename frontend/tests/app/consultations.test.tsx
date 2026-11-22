/**
 * @jest-environment jsdom
 */
import { act, render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConsultationThread } from '@/components/consultation/ConsultationThread'
import ConsultationsPage from '@/app/consultations/page-content'
import AdminConsultationsPage from '@/app/admin/consultations/page-content'
import type { ConsultationThread as Thread } from '@/lib/consultation'

// #2028: 教員と学生の相談。

jest.mock('next/navigation', () => ({ useRouter: () => ({ push: jest.fn() }), usePathname: () => '/' }))
jest.mock('@/lib/auth', () => ({
  authService: {
    ensureFreshUserToken: async () => undefined,
    getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }),
    getAdminFetchHeaders: () => ({ Authorization: 'Bearer admin' }),
  },
}))

// 学校の解決は、テストから終わらせる
let resolveScope: ((failure?: string) => void) | null = null
let changeSchool: ((id: number | undefined) => void) | null = null
jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  SchoolFilterSelect: ({ onChange, onResolved }: { onChange: (id: number | undefined) => void; onResolved?: (error?: string) => void }) => {
    resolveScope = onResolved ?? null
    changeSchool = onChange
    return null
  },
}))

const THREAD: Thread = {
  guidance_id: 1,
  student_user_id: 10,
  kind: 'resume',
  message: '履歴書を見直しましょう',
  created_at: '2026-11-13T01:00:00Z',
  messages: [
    { id: 1, guidance_id: 1, author_role: 'student', body: 'どこを直せばよいですか？', read_at: null, created_at: '2026-11-13T02:00:00Z' },
  ],
}

type Pending = { url: string; resolve: (status: number, body: unknown) => void }

/** fetch を、テストから1件ずつ返せるようにする */
function deferredFetch(): Pending[] {
  const pending: Pending[] = []
  global.fetch = jest.fn(
    (url: RequestInfo | URL) =>
      new Promise((resolve) =>
        pending.push({ url: String(url), resolve: (status, body) => resolve({ ok: status < 400, status, json: async () => body }) }),
      ),
  ) as unknown as typeof fetch
  return pending
}

function respondWith(routes: (url: string) => [number, unknown]) {
  const urls: string[] = []
  global.fetch = jest.fn(async (url: RequestInfo | URL) => {
    urls.push(String(url))
    const [status, body] = routes(String(url))
    return { ok: status < 400, status, json: async () => body }
  }) as unknown as typeof fetch
  return urls
}

describe('スレッド', () => {
  test('案内と、その後のやり取りを古い順に出し、返信を送ると表示を更新する。見ていた最後の発言の ID も渡す', async () => {
    const send = jest.fn(async (body: string) => ({
      ...THREAD,
      messages: [...THREAD.messages, { id: 2, guidance_id: 1, author_role: 'teacher' as const, body, read_at: null, created_at: '2026-11-13T03:00:00Z' }],
    }))
    const onChanged = jest.fn()
    render(<ConsultationThread me="teacher" load={async () => THREAD} send={send} onChanged={onChanged} />)

    expect(await screen.findByText('履歴書を見直しましょう')).toBeInTheDocument()
    expect(screen.getByText('どこを直せばよいですか？')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('学生への返信'), { target: { value: '自己PRを具体的にしましょう' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    expect(await screen.findByText('自己PRを具体的にしましょう')).toBeInTheDocument()
    expect(send).toHaveBeenCalledWith('自己PRを具体的にしましょう', 1)
    expect(onChanged).toHaveBeenCalled()
  })

  test('送信に失敗しても入力は残し、エラーを出す', async () => {
    const send = jest.fn(async () => { throw new Error('返信を送れませんでした') })
    render(<ConsultationThread me="student" load={async () => THREAD} send={send} />)
    await screen.findByText('履歴書を見直しましょう')
    fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '確認します' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    expect(await screen.findByText('返信を送れませんでした')).toBeInTheDocument()
    expect(screen.getByLabelText('先生への返信')).toHaveValue('確認します')
  })

  test('送信中は入力欄を止める（送信後に書き足した文章を消さない）', async () => {
    let finish: (t: Thread) => void = () => {}
    const send = jest.fn(() => new Promise<Thread>((r) => { finish = r }))
    render(<ConsultationThread me="student" load={async () => THREAD} send={send} />)
    await screen.findByText('履歴書を見直しましょう')
    fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '確認します' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    expect(screen.getByLabelText('先生への返信')).toBeDisabled()
    await act(async () => finish(THREAD))
    expect(screen.getByLabelText('先生への返信')).toHaveValue('')
    expect(screen.getByLabelText('先生への返信')).not.toBeDisabled()
  })

  test('保存できたが読み直しに失敗した応答（stale）なら、送信は成功として入力を消し、スレッドを読み直す', async () => {
    const load = jest.fn(async () => THREAD)
    const send = jest.fn(async () => ({ ...THREAD, messages: [], stale: true }))
    render(<ConsultationThread me="student" load={load} send={send} />)
    await screen.findByText('履歴書を見直しましょう')
    fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '確認します' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    await waitFor(() => expect(load).toHaveBeenCalledTimes(2))
    expect(screen.getByLabelText('先生への返信')).toHaveValue('')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  test('読み直しの途中は送信できない（古い取得結果で新しい発言を上書きしない）', async () => {
    let finishReload: (t: Thread) => void = () => {}
    const load = jest.fn()
      .mockResolvedValueOnce(THREAD)
      .mockImplementationOnce(() => new Promise<Thread>((r) => { finishReload = r }))
    const send = jest.fn(async () => ({ ...THREAD, messages: [], stale: true }))
    render(<ConsultationThread me="student" load={load} send={send} />)
    await screen.findByText('履歴書を見直しましょう')
    fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '1つ目' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    await waitFor(() => expect(load).toHaveBeenCalledTimes(2))
    fireEvent.change(screen.getByLabelText('先生への返信'), { target: { value: '2つ目' } })
    expect(screen.getByRole('button', { name: '送信する' })).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    expect(send).toHaveBeenCalledTimes(1)
    await act(async () => finishReload(THREAD))
    expect(screen.getByRole('button', { name: '送信する' })).not.toBeDisabled()
  })

  test('文字数は Backend と同じく、前後の空白を除いたコードポイントで数える', async () => {
    render(<ConsultationThread me="student" load={async () => THREAD} send={jest.fn()} />)
    await screen.findByText('履歴書を見直しましょう')
    const input = screen.getByLabelText('先生への返信')
    fireEvent.change(input, { target: { value: 'あ'.repeat(1001) } })
    expect(screen.getByRole('button', { name: '送信する' })).toBeDisabled()
    fireEvent.change(input, { target: { value: `  ${'あ'.repeat(1000)}\n` } })
    expect(screen.getByRole('button', { name: '送信する' })).not.toBeDisabled()
    // 絵文字は UTF-16 では2つ分だが、1文字と数える
    fireEvent.change(input, { target: { value: '😀'.repeat(1000) } })
    expect(screen.getByText('1000 / 1000')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '送信する' })).not.toBeDisabled()
  })

  test('呼び出し側が毎回新しい onChanged を渡しても、読み込みを繰り返さない', async () => {
    const load = jest.fn(async () => THREAD)
    const { rerender } = render(<ConsultationThread me="teacher" load={load} send={jest.fn()} onChanged={() => {}} />)
    await screen.findByText('履歴書を見直しましょう')
    rerender(<ConsultationThread me="teacher" load={load} send={jest.fn()} onChanged={() => {}} />)
    await waitFor(() => expect(load).toHaveBeenCalledTimes(1))
  })

  test('学生の質問のスレッドは、本文を最初の発言と重ねて出さない', async () => {
    const question: Thread = { ...THREAD, kind: 'question', message: '面接の練習量は？', messages: [{ id: 1, guidance_id: 1, author_role: 'student', body: '面接の練習量は？', read_at: null, created_at: '2026-11-13T02:00:00Z' }] }
    render(<ConsultationThread me="student" load={async () => question} send={jest.fn()} />)
    expect(await screen.findAllByText('面接の練習量は？')).toHaveLength(1)
  })
})

const ITEM = { guidance_id: 3, kind: 'resume', message: '閉じた案内', created_at: '2026-11-01T00:00:00Z', updated_at: '2026-11-01T00:00:00Z', unread: 2 }

describe('学生の相談の画面', () => {
  test('相談の一覧を同一オリジンの API から取り、閉じた案内も出し、続きを読み込める', async () => {
    const urls = respondWith((url) => {
      if (url.includes('cursor=')) return [200, { items: [{ ...ITEM, guidance_id: 1, message: '古い質問', kind: 'question', unread: 0 }], next_cursor: '' }]
      return [200, { items: [ITEM], next_cursor: '100_3' }]
    })
    render(<ConsultationsPage />)
    expect(await screen.findByText('閉じた案内')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '先生からの案内を開く（未読 2 件）' })).toBeInTheDocument()
    expect(urls[0]).toBe('/api/user/consultations')

    fireEvent.click(screen.getByRole('button', { name: 'さらに読み込む' }))
    expect(await screen.findByText('古い質問')).toBeInTheDocument()
    expect(urls.at(-1)).toBe('/api/user/consultations?cursor=100_3')
    expect(screen.queryByRole('button', { name: 'さらに読み込む' })).not.toBeInTheDocument()
  })

  test('未読が取れなかったら、一覧は出したまま知らせ、再取得できる', async () => {
    let unreadOK = false
    respondWith((url) => {
      if (url.endsWith('/unread')) return unreadOK ? [200, { by_guidance: { '3': 5 }, total: 5 }] : [500, {}]
      if (url.endsWith('/3')) return [200, { ...THREAD, guidance_id: 3 }]
      return [200, { items: [ITEM], next_cursor: '' }]
    })
    render(<ConsultationsPage />)
    fireEvent.click(await screen.findByRole('button', { name: '先生からの案内を開く（未読 2 件）' }))
    expect(await screen.findByText('未読の数を読み込めませんでした。')).toBeInTheDocument()
    expect(screen.getByText('閉じた案内')).toBeInTheDocument()

    unreadOK = true
    fireEvent.click(screen.getByRole('button', { name: '閉じる' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '再取得' }))
    await waitFor(() => expect(screen.queryByText('未読の数を読み込めませんでした。')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: '先生からの案内を開く（未読 5 件）' })).toBeInTheDocument()
  })

  test('学校が未登録などで質問を受け付けられないときは、理由を出して入力を残す', async () => {
    respondWith((url) => {
      if (url === '/api/user/consultations') return [200, { items: [], next_cursor: '' }]
      return [400, { message: '学校が登録されていないため、先生に質問を送れません。プロフィールで学校を設定してください' }]
    })
    render(<ConsultationsPage />)
    await screen.findByText(/まだやり取りはありません/)
    fireEvent.change(screen.getByLabelText('質問の内容'), { target: { value: '面接について' } })
    // 一覧の取得と区別するため、POST だけ失敗させる
    global.fetch = jest.fn(async () => ({ ok: false, status: 400, json: async () => ({ message: '学校が登録されていないため、先生に質問を送れません。プロフィールで学校を設定してください' }) })) as unknown as typeof fetch
    fireEvent.click(screen.getByRole('button', { name: '質問を送る' }))
    expect(await screen.findByText(/学校が登録されていないため/)).toBeInTheDocument()
    expect(screen.getByLabelText('質問の内容')).toHaveValue('面接について')
  })
})

const INBOX_ITEM = { guidance_id: 1, student_user_id: 10, student_name: '山田', kind: 'question', first_message: '質問です', unread: 1, last_message_at: '2026-11-13T00:00:00Z', last_message_id: 50 }

describe('先生の受信一覧', () => {
  test('担当校の解決が終わるまで取りに行かず、取得に成功して 0 件のときだけ「まだありません」を出す', async () => {
    const pending = deferredFetch()
    render(<AdminConsultationsPage />)
    expect(pending).toHaveLength(0)
    expect(screen.queryByText('相談はまだありません')).not.toBeInTheDocument()

    await act(async () => {
      changeSchool?.(5)
      resolveScope?.()
    })
    await waitFor(() => expect(pending).toHaveLength(1))
    expect(pending[0].url).toBe('/api/admin/teacher/consultations?school_id=5')
    await act(async () => pending[0].resolve(500, { message: '相談の一覧を読み込めませんでした' }))
    expect(await screen.findByText('相談の一覧を読み込めませんでした')).toBeInTheDocument()
    expect(screen.queryByText('相談はまだありません')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    await waitFor(() => expect(pending).toHaveLength(2))
    await act(async () => pending[1].resolve(200, { items: [] }))
    expect(await screen.findByText('相談はまだありません')).toBeInTheDocument()
  })

  test('担当校の解決に失敗したら、エラーと再試行を出す', async () => {
    deferredFetch()
    render(<AdminConsultationsPage />)
    await act(async () => resolveScope?.('学校アクセス情報の取得に失敗しました'))
    expect(await screen.findByText(/担当校の情報を読み込めませんでした/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再試行' })).toBeInTheDocument()
    expect(screen.queryByText('相談はまだありません')).not.toBeInTheDocument()
  })

  test('学校を切り替えたあとに前の学校の遅い応答が届いても、一覧を上書きしない', async () => {
    const pending = deferredFetch()
    render(<AdminConsultationsPage />)
    await act(async () => {
      changeSchool?.(5)
      resolveScope?.()
    })
    await waitFor(() => expect(pending).toHaveLength(1))
    await act(async () => changeSchool?.(6))
    await waitFor(() => expect(pending).toHaveLength(2))
    expect(pending[1].url).toContain('school_id=6')

    await act(async () => pending[1].resolve(200, { items: [{ ...INBOX_ITEM, student_name: '学校6の学生' }] }))
    expect(await screen.findByText('学校6の学生')).toBeInTheDocument()
    await act(async () => pending[0].resolve(200, { items: [{ ...INBOX_ITEM, student_name: '学校5の学生' }] }))
    expect(screen.queryByText('学校5の学生')).not.toBeInTheDocument()
  })

  test('続きを before_id で読み込む', async () => {
    const urls = respondWith((url) =>
      url.includes('before_id=50')
        ? [200, { items: [{ ...INBOX_ITEM, guidance_id: 2, student_name: '佐藤', last_message_id: 40 }] }]
        : [200, { items: [INBOX_ITEM], next_before_id: 50 }],
    )
    render(<AdminConsultationsPage />)
    await act(async () => resolveScope?.())
    expect(await screen.findByText('山田')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'さらに読み込む' }))
    expect(await screen.findByText('佐藤')).toBeInTheDocument()
    expect(urls.at(-1)).toBe('/api/admin/teacher/consultations?before_id=50')
  })

  test('返信のあとの一覧の取り直しに失敗したら、警告と再試行を出す（スレッドの操作は成功のまま）', async () => {
    let inboxCalls = 0
    respondWith((url) => {
      if (url.startsWith('/api/admin/teacher/consultations/1')) {
        return [200, { ...THREAD, messages: [...THREAD.messages, { id: 3, guidance_id: 1, author_role: 'teacher', body: '返信です', read_at: null, created_at: '2026-11-13T03:00:00Z' }] }]
      }
      inboxCalls++
      // 1回目（最初の一覧）と、スレッドを開いたあとの取り直しは成功、返信のあとの取り直しは失敗、再試行は成功
      if (inboxCalls === 3) return [500, { message: 'down' }]
      return [200, { items: [{ ...INBOX_ITEM, unread: inboxCalls >= 4 ? 0 : 1 }] }]
    })
    render(<AdminConsultationsPage />)
    await act(async () => resolveScope?.())
    fireEvent.click(await screen.findByRole('button', { name: /山田さんの相談を開く/ }))
    await waitFor(() => expect(inboxCalls).toBe(2))
    fireEvent.change(await screen.findByLabelText('学生への返信'), { target: { value: '返信です' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    expect(await screen.findByText(/一覧を最新にできませんでした/)).toBeInTheDocument()
    expect(screen.getAllByText('返信です').length).toBeGreaterThan(0)
    fireEvent.click(screen.getByRole('button', { name: '閉じる' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: '再試行' }))
    await waitFor(() => expect(screen.queryByText(/一覧を最新にできませんでした/)).not.toBeInTheDocument())
    expect(inboxCalls).toBe(4)
  })

  test('スレッドで返信したあとも、「さらに読み込む」で足した続きを消さない', async () => {
    const urls = respondWith((url) => {
      if (url.includes('before_id=50')) return [200, { items: [{ ...INBOX_ITEM, guidance_id: 2, student_name: '佐藤', last_message_id: 40 }] }]
      if (url.startsWith('/api/admin/teacher/consultations/1')) {
        return [200, { ...THREAD, messages: [...THREAD.messages, { id: 3, guidance_id: 1, author_role: 'teacher', body: '返信です', read_at: null, created_at: '2026-11-13T03:00:00Z' }] }]
      }
      return [200, { items: [{ ...INBOX_ITEM, unread: 0 }], next_before_id: 50 }]
    })
    render(<AdminConsultationsPage />)
    await act(async () => resolveScope?.())
    expect(await screen.findByText('山田')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'さらに読み込む' }))
    expect(await screen.findByText('佐藤')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: /山田さんの相談を開く/ }))
    fireEvent.change(await screen.findByLabelText('学生への返信'), { target: { value: '返信です' } })
    fireEvent.click(screen.getByRole('button', { name: '送信する' }))
    await waitFor(() => expect(urls.filter((u) => u === '/api/admin/teacher/consultations').length).toBeGreaterThanOrEqual(2))
    fireEvent.click(screen.getByRole('button', { name: '閉じる' }))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(screen.getByText('佐藤')).toBeInTheDocument()
    expect(screen.getByText('山田')).toBeInTheDocument()
  })
})
