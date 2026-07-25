/**
 * @jest-environment jsdom
 */
import { resolveCompanyByName, getNextAvatarGender, mergeResolvedCompany } from '@/app/interview/utils'

describe('resolveCompanyByName', () => {
  const originalFetch = global.fetch

  afterEach(() => {
    global.fetch = originalFetch
    jest.restoreAllMocks()
  })

  it('空文字の場合は id=0 の空エントリを返す', async () => {
    const result = await resolveCompanyByName('  ')
    expect(result).toEqual({ id: 0, name: '' })
  })

  it('DB に完全一致する企業が見つかれば id を解決する', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        companies: [
          { id: 42, name: '株式会社テスト', industry: 'IT' },
        ],
      }),
    }) as unknown as typeof fetch

    const result = await resolveCompanyByName('株式会社テスト')

    expect(global.fetch).toHaveBeenCalledWith(
      expect.stringContaining('/api/companies?'),
      expect.objectContaining({ cache: 'no-store' }),
    )
    expect(result).toEqual({ id: 42, name: '株式会社テスト', industry: 'IT' })
  })

  it('fallback は解決済みデータより優先されない基本フィールド以外を補完する', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        companies: [{ id: 7, name: '未来株式会社' }],
      }),
    }) as unknown as typeof fetch

    const result = await resolveCompanyByName('未来株式会社', { industry: '製造業' })

    expect(result).toEqual({ id: 7, name: '未来株式会社', industry: '製造業' })
  })

  it('一致する企業が見つからない場合は id=0 で fallback を保持する', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ companies: [] }),
    }) as unknown as typeof fetch

    const result = await resolveCompanyByName('未登録企業', { description: '説明' })

    expect(result).toEqual({ id: 0, name: '未登録企業', description: '説明' })
  })

  it('fetch が失敗した場合も id=0 のフォールバックを返す', async () => {
    global.fetch = jest.fn().mockRejectedValue(new Error('network error')) as unknown as typeof fetch

    const result = await resolveCompanyByName('エラー企業')

    expect(result).toEqual({ id: 0, name: 'エラー企業' })
  })

  it('レスポンスが ok でない場合も id=0 のフォールバックを返す', async () => {
    global.fetch = jest.fn().mockResolvedValue({ ok: false }) as unknown as typeof fetch

    const result = await resolveCompanyByName('失敗企業')

    expect(result).toEqual({ id: 0, name: '失敗企業' })
  })
})

describe('mergeResolvedCompany', () => {
  it('選択名が変わっている場合は prev を維持する', () => {
    // await 中にユーザーが別企業へ切り替えたケース
    expect(
      mergeResolvedCompany({ id: 0, name: 'B社' }, 'A社', { id: 2, name: 'A社' }),
    ).toEqual({ id: 0, name: 'B社' })
  })

  it('正の id を id:0 の失敗結果で上書きしない', () => {
    const prev = { id: 99, name: '登録株式会社' }
    const resolved = { id: 0, name: '登録株式会社' }
    expect(mergeResolvedCompany(prev, '登録株式会社', resolved)).toEqual(prev)
  })

  it('名前が一致し resolved が正の id なら更新する', () => {
    const prev = { id: 0, name: '登録株式会社' }
    const resolved = { id: 42, name: '登録株式会社', industry: 'IT' }
    expect(mergeResolvedCompany(prev, '登録株式会社', resolved)).toEqual(resolved)
  })

  it('prev が null なら null のまま', () => {
    expect(mergeResolvedCompany(null, 'X', { id: 1, name: 'X' })).toBeNull()
  })
})

describe('getNextAvatarGender', () => {
  const KEY = 'interview_avatar_index'

  beforeEach(() => {
    localStorage.clear()
  })

  it('呼び出しごとに male / female を交互に返す', () => {
    expect(getNextAvatarGender()).toBe('male')
    expect(getNextAvatarGender()).toBe('female')
    expect(getNextAvatarGender()).toBe('male')
  })

  it('localStorage にインデックスを永続化する', () => {
    getNextAvatarGender()
    expect(localStorage.getItem(KEY)).toBe('1')
    getNextAvatarGender()
    expect(localStorage.getItem(KEY)).toBe('2')
  })

  it('localStorage が使用できない場合は male を返す', () => {
    const spy = jest.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('unavailable')
    })
    expect(getNextAvatarGender()).toBe('male')
    spy.mockRestore()
  })
})
