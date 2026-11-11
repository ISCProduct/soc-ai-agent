/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { TrainingConsentCard } from '@/components/profile/TrainingConsentCard'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ Authorization: 'Bearer test' }),
  },
}))

type Reply = { ok: boolean; body?: unknown }

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
    return { ok: reply.ok, json: async () => reply.body } as Response
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
    expect(putBodies).toEqual([{ allow_training_use: true }])
    expect(await consentSwitch()).toBeChecked()
  })

  it('オフにすると、取り消しを保存する', async () => {
    getReply = { ok: true, body: { allow_training_use: true } }
    putReply = { ok: true, body: { allow_training_use: false } }
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText('学習への利用をオフにしました')
    expect(putBodies).toEqual([{ allow_training_use: false }])
    expect(await consentSwitch()).not.toBeChecked()
  })

  it('保存に失敗したら、スイッチを元に戻してエラーを出す（保存できていないのにオンに見せない）', async () => {
    putReply = { ok: false }
    render(<TrainingConsentCard />)
    fireEvent.click(await consentSwitch())

    await screen.findByText(/保存に失敗しました/)
    expect(await consentSwitch()).not.toBeChecked()
    expect(screen.queryByText('学習への利用に同意しました')).not.toBeInTheDocument()
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
