import { NextRequest } from 'next/server'
import { GET as getAxes } from '@/app/api/chat/matches/[id]/axes/route'
import { GET as getAxisScores } from '@/app/api/chat/axis-scores/route'

// SOCAIAGENT-611: 軸ごとの内訳と「あなたの10の傾向」を Backend へ中継する route。

const ok = () => new Response(JSON.stringify({ axes: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } })
const withToken = (url: string) => new NextRequest(url, { headers: { 'X-User-Token': 'jwt' } })
const params = (id: string) => ({ params: Promise.resolve({ id }) })

describe('GET /api/chat/matches/[id]/axes', () => {
  afterEach(() => jest.restoreAllMocks())

  it.each(['0', 'abc', '-1', '1.5'])('id が正の整数でなければ 400（%s）', async (id) => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const res = await getAxes(withToken(`http://localhost:3000/api/chat/matches/${id}/axes`), params(id))
    expect(res.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('トークンが無ければ 401 で、Backend を呼ばない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const res = await getAxes(new NextRequest('http://localhost:3000/api/chat/matches/10/axes'), params('10'))
    expect(res.status).toBe(401)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('Backend の推薦の内訳へ、トークン付きで中継する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(ok())
    const res = await getAxes(withToken('http://localhost:3000/api/chat/matches/10/axes'), params('10'))
    expect(res.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/chat\/matches\/10\/axes$/),
      expect.objectContaining({ headers: expect.objectContaining({ 'X-User-Token': 'jwt' }), cache: 'no-store' }),
    )
  })
})

describe('GET /api/chat/axis-scores', () => {
  afterEach(() => jest.restoreAllMocks())

  it('session_id が無ければ 400', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const res = await getAxisScores(withToken('http://localhost:3000/api/chat/axis-scores'))
    expect(res.status).toBe(400)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('トークンが無ければ 401 で、Backend を呼ばない', async () => {
    const fetchMock = jest.spyOn(global, 'fetch')
    const res = await getAxisScores(new NextRequest('http://localhost:3000/api/chat/axis-scores?session_id=s1'))
    expect(res.status).toBe(401)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('session_id をエンコードして、トークン付きで中継する', async () => {
    const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValue(ok())
    const res = await getAxisScores(withToken('http://localhost:3000/api/chat/axis-scores?session_id=s%201&x=1'))
    expect(res.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringMatching(/\/api\/chat\/axis-scores\?session_id=s\+1$/),
      expect.objectContaining({ headers: expect.objectContaining({ 'X-User-Token': 'jwt' }) }),
    )
  })
})
