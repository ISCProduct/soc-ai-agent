import { APPLY_CLOSED_REASON, fetchApplyEnabled, getApplyClosedButtonProps } from '@/lib/apply-availability'

describe('getApplyClosedButtonProps', () => {
  it('閉じている間は無効＋理由、開いていれば有効', () => {
    expect(getApplyClosedButtonProps(false)).toEqual({ disabled: true, title: APPLY_CLOSED_REASON })
    expect(getApplyClosedButtonProps(true)).toEqual({ disabled: false, title: '' })
  })
})

describe('fetchApplyEnabled', () => {
  const originalFetch = global.fetch
  afterEach(() => {
    global.fetch = originalFetch
  })

  const respond = (ok: boolean, body: unknown) =>
    (global.fetch = jest.fn(async () => ({ ok, json: async () => body })) as unknown as typeof fetch)

  it('enabled が true のときだけ開く', async () => {
    respond(true, { enabled: true })
    await expect(fetchApplyEnabled({})).resolves.toBe(true)
  })

  it.each([
    ['false', true, { enabled: false }],
    ['文字列の"true"', true, { enabled: 'true' }],
    ['キー無し', true, {}],
    ['HTTPエラー', false, { enabled: true }],
  ])('%s は閉じている扱い', async (_name, ok, body) => {
    respond(ok, body)
    await expect(fetchApplyEnabled({})).resolves.toBe(false)
  })

  it('通信エラーは閉じている扱い', async () => {
    global.fetch = jest.fn(async () => {
      throw new Error('network')
    }) as unknown as typeof fetch
    await expect(fetchApplyEnabled({})).resolves.toBe(false)
  })

  it('認証ヘッダーを付けて availability を呼ぶ', async () => {
    respond(true, { enabled: true })
    await fetchApplyEnabled({ 'X-User-Token': 't' })
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/applications/availability',
      expect.objectContaining({ headers: { 'X-User-Token': 't' } }),
    )
  })
})
