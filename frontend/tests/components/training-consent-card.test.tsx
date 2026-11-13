/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import {
  TRAINING_CONSENT_TEXT_VERSION,
  TrainingConsentCard,
} from '@/components/profile/TrainingConsentCard'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ Authorization: 'Bearer test' }),
  },
}))

type Reply = { ok: boolean; status?: number; body?: unknown }

const originalFetch = global.fetch
let getReply: Reply
let putReply: Reply
let putBodies: unknown[]

beforeEach(() => {
  getReply = { ok: true, body: { allow_training_use: false } }
  putReply = { ok: true, body: { allow_training_use: true } }
  putBodies = []
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
    expect(String(input)).toContain('/api/user/training-consent')
    const reply = init?.method === 'PUT' ? putReply : getReply
    if (init?.method === 'PUT') putBodies.push(JSON.parse(String(init.body)))
    return {
      ok: reply.ok,
      status: reply.status ?? (reply.ok ? 200 : 500),
      json: async () => reply.body,
    } as Response
  })
})

afterEach(() => {
  global.fetch = originalFetch
})

const consentSwitch = () => screen.findByRole('switch', { name: /採点モデルの学習に使うことに同意する/ })

describe('TrainingConsentCard', () => {
  it('初期値（未同意）はオフで表示する', async () => {
    render(<TrainingConsentCard />)
    expect(await consentSwitch()).not.toBeChecked()
  })

  it('同意済みならオンで表示する', async () => {
    getReply = { ok: true, body: { allow_training_use: true } }
    render(<TrainingConsentCard />)
    expect(await consentSwitch()).toBeChecked()
  })

  it('オンにすると、その時点で同意だけを保存し、保存できたことを表示する', async () => {
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText('学習への利用に同意しました')
    expect(putBodies).toEqual([
      { allow_training_use: true, consent_text_version: TRAINING_CONSENT_TEXT_VERSION },
    ])
    expect(await consentSwitch()).toBeChecked()
  })

  it('オフにすると、取り消しを保存する', async () => {
    getReply = { ok: true, body: { allow_training_use: true } }
    putReply = { ok: true, body: { allow_training_use: false } }
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText('学習への利用をオフにしました')
    expect(putBodies).toEqual([
      { allow_training_use: false, consent_text_version: TRAINING_CONSENT_TEXT_VERSION },
    ])
    expect(await consentSwitch()).not.toBeChecked()
  })

  it('保存に失敗したら、今の値を取り直して表示する（保存されていなければオフのまま）', async () => {
    putReply = { ok: false }
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText(/保存できたか確認できませんでした/)
    expect(await consentSwitch()).not.toBeChecked()
    expect(screen.queryByText('学習への利用に同意しました')).not.toBeInTheDocument()
  })

  it('保存の応答が失敗しても、実際は保存されていたらオンで表示する（保存前の値を決め打ちしない）', async () => {
    putReply = { ok: false }
    render(<TrainingConsentCard />)
    const sw = await consentSwitch()
    getReply = { ok: true, body: { allow_training_use: true } }
    fireEvent.click(sw)

    await screen.findByText(/保存できたか確認できませんでした/)
    expect(await consentSwitch()).toBeChecked()
  })

  it('保存も取り直しも失敗したら、スイッチを出さない（分からない状態を見せない）', async () => {
    putReply = { ok: false }
    render(<TrainingConsentCard />)
    const sw = await consentSwitch()
    getReply = { ok: false }
    fireEvent.click(sw)

    await screen.findByText(/設定の取得に失敗しました/)
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
  })

  it('表示している文言が古い（409）なら、保存されていないので元の値に戻し、再読み込みを促して操作を止める', async () => {
    putReply = { ok: false, status: 409, body: { error: 'conflict' } }
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText(/画面を再読み込みして/)
    const sw = await consentSwitch()
    expect(sw).not.toBeChecked()
    expect(sw).toBeDisabled()
    expect(screen.queryByText('学習への利用に同意しました')).not.toBeInTheDocument()
    // 「保存できたか分からない」ではない（拒否されたことが分かっている）ので、取り直しの案内は出さない
    expect(screen.queryByText(/保存できたか確認できませんでした/)).not.toBeInTheDocument()
    expect(putBodies).toHaveLength(1)
  })

  it('文言の版は日付（YYYY-MM-DD）で持つ', () => {
    expect(TRAINING_CONSENT_TEXT_VERSION).toMatch(/^\d{4}-\d{2}-\d{2}$/)
  })

  it('取得に失敗したら、スイッチを出さずにエラーを出す（不明な状態をオフに見せない）', async () => {
    getReply = { ok: false }
    render(<TrainingConsentCard />)

    await screen.findByText(/設定の取得に失敗しました/)
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
  })

  it('保存中はスイッチを押せない', async () => {
    let resolvePut: (r: Response) => void = () => {}
    global.fetch = jest.fn().mockImplementation(async (_input: RequestInfo | URL, init?: RequestInit) => {
      if (init?.method === 'PUT') return new Promise<Response>((r) => { resolvePut = r })
      return { ok: true, json: async () => ({ allow_training_use: false }) } as Response
    })
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await waitFor(async () => expect(await consentSwitch()).toBeDisabled())
    resolvePut({ ok: true, json: async () => ({ allow_training_use: true }) } as Response)
    await screen.findByText('学習への利用に同意しました')
    expect(await consentSwitch()).not.toBeDisabled()
  })
})
