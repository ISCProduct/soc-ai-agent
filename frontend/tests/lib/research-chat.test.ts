import { askResearchChat, researchChatAnswerSchema } from '@/lib/company/research-chat'

describe('researchChatAnswerSchema', () => {
  it('accepts answered payload with nullable arrays', () => {
    const got = researchChatAnswerSchema.parse({
      status: 'answered',
      answer: '従業員数は100人です',
      company_ids: null,
      sources: null,
    })
    expect(got.company_ids).toEqual([])
    expect(got.sources).toEqual([])
  })
})

describe('askResearchChat', () => {
  const originalFetch = global.fetch
  afterEach(() => {
    global.fetch = originalFetch
  })

  it('posts question and returns parsed answer', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        status: 'answered',
        answer: '売上高は10億円です',
        company_ids: [1],
        sources: [{ company_id: 1, type: 'edinet', title: '有価証券報告書' }],
      }),
    }) as unknown as typeof fetch
    const got = await askResearchChat(1, '売上高は？')
    expect(got.status).toBe('answered')
    expect(got.answer).toContain('10億円')
    expect(global.fetch).toHaveBeenCalledWith(
      '/api/companies/1/research-chat',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('surfaces backend error message', async () => {
    global.fetch = jest.fn().mockResolvedValue({
      ok: false,
      status: 429,
      json: async () => ({ error: '今日の質問回数の上限に達しました。明日また利用できます。' }),
    }) as unknown as typeof fetch
    await expect(askResearchChat(1, '質問')).rejects.toMatchObject({
      status: 429,
      message: '今日の質問回数の上限に達しました。明日また利用できます。',
    })
  })
})
