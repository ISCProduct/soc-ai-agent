import { NextRequest } from 'next/server'
import * as scheduleId from '@/app/api/schedule/[id]/route'
import * as chatHistory from '@/app/api/chat/history/route'
import * as resumeAnnotated from '@/app/api/resume/annotated/route'
import * as resumeReview from '@/app/api/resume/review/route'
import * as resumeReviewStream from '@/app/api/resume/review/stream/route'

// パスパラメータ・クエリ値に区切り文字が入っても、Backend の別の経路や別のパラメータへ化けないこと
const injected = '1&user_id=999'
const auth = { 'X-User-Token': 'user-jwt' }

describe('プロキシのパラメータエンコード', () => {
  beforeEach(() => {
    jest.spyOn(global, 'fetch').mockResolvedValue(new Response('{}', { status: 200 }))
  })
  afterEach(() => jest.restoreAllMocks())

  const lastUrl = () => (global.fetch as jest.Mock).mock.calls[0][0] as string

  it.each([
    ['chat/history', 'session_id', 'GET', chatHistory.GET],
    ['resume/annotated', 'document_id', 'GET', resumeAnnotated.GET],
    ['resume/review', 'document_id', 'POST', resumeReview.POST],
    ['resume/review/stream', 'document_id', 'POST', resumeReviewStream.POST],
  ])('%s は %s の値をエンコードする', async (path, key, method, handler) => {
    const url = `http://localhost:3000/api/${path}?${key}=${encodeURIComponent(injected)}`
    await handler(new NextRequest(url, { method, headers: auth, ...(method === 'POST' ? { body: '{}' } : {}) }))
    const sent = new URL(lastUrl())
    expect(sent.searchParams.get(key)).toBe(injected)
    expect(sent.searchParams.get('user_id')).toBeNull()
  })

  it.each([
    ['GET', scheduleId.GET],
    ['PUT', scheduleId.PUT],
    ['DELETE', scheduleId.DELETE],
  ])('schedule/[id] %s は id をエンコードする', async (method, handler) => {
    const request = new NextRequest('http://localhost:3000/api/schedule/x', {
      method,
      headers: { ...auth, 'Content-Type': 'application/json' },
      ...(method === 'PUT' ? { body: '{}' } : {}),
    })
    await handler(request, { params: Promise.resolve({ id: '../admin?x=1' }) })
    expect(lastUrl()).toMatch(/\/api\/schedule\/\.\.%2Fadmin%3Fx%3D1$/)
  })
})
