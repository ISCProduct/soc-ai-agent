/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import PageContent from '@/app/admin/companies/[id]/interview-questions/page-content'

jest.mock('next/navigation', () => ({ useParams: () => ({ id: '10' }) }))

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

const QUESTION = {
  id: 1,
  company_id: 10,
  position: '',
  category: '志望動機',
  question_text: '志望理由を教えてください',
  priority: 0,
  is_required: true,
  evaluation_intent: '困難に直面したときの粘り強さ',
  created_at: '',
  updated_at: '',
}

function jsonRes(body: unknown) {
  const raw = JSON.stringify(body)
  return { ok: true, status: 200, text: async () => raw, json: async () => JSON.parse(raw) }
}

type Sent = { method: string; url: string; body: Record<string, unknown> }
const originalFetch = global.fetch
let sent: Sent[] = []

beforeEach(() => {
  sent = []
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    const method = init?.method ?? 'GET'
    if (method !== 'GET') {
      sent.push({ method, url, body: init?.body ? JSON.parse(String(init.body)) : {} })
      return jsonRes({ ...QUESTION })
    }
    if (url.includes('interview-questions')) return jsonRes({ questions: [QUESTION] })
    return jsonRes({ id: 10, name: 'テスト株式会社' })
  })
})

afterEach(() => {
  global.fetch = originalFetch
})

/** ダイアログの保存ボタン（キャンセルの次にある最後のボタン）を押す */
function clickSave() {
  const buttons = within(screen.getByRole('dialog')).getAllByRole('button')
  fireEvent.click(buttons[buttons.length - 1])
}

// #1092: 評価したい観点を、一覧で確認できる。
test('一覧に評価したい観点が表示される', async () => {
  render(<PageContent />)
  expect((await screen.findAllByText('観点: 困難に直面したときの粘り強さ')).length).toBeGreaterThan(0)
})

test('新しい質問を作るとき、評価したい観点を evaluation_intent として送る', async () => {
  render(<PageContent />)
  await screen.findAllByText('志望理由を教えてください')
  fireEvent.click(screen.getByRole('button', { name: /質問を追加/ }))
  fireEvent.change(screen.getByLabelText(/質問文/), { target: { value: '最近学んだことを教えてください' } })
  fireEvent.change(screen.getByLabelText('評価したい観点（意図）'), { target: { value: '学び続ける姿勢' } })
  clickSave()
  await waitFor(() => expect(sent).toHaveLength(1))
  expect(sent[0].method).toBe('POST')
  expect(sent[0].body.evaluation_intent).toBe('学び続ける姿勢')
})

test('編集では今の観点がフォームに入り、空にして保存すると空文字を送って消す', async () => {
  render(<PageContent />)
  fireEvent.click((await screen.findAllByRole('button', { name: '質問を編集' }))[0])
  const intent = screen.getByLabelText('評価したい観点（意図）') as HTMLTextAreaElement
  expect(intent.value).toBe('困難に直面したときの粘り強さ')
  fireEvent.change(intent, { target: { value: '' } })
  clickSave()
  await waitFor(() => expect(sent).toHaveLength(1))
  expect(sent[0].method).toBe('PUT')
  expect(sent[0].body.evaluation_intent).toBe('')
})
