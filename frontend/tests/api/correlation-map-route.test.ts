import { NextRequest } from 'next/server'
import { GET as mapGET } from '@/app/api/companies/[id]/correlation-map/route'
import { GET as compareGET } from '@/app/api/companies/[id]/correlation-map/compare/route'

afterEach(() => jest.restoreAllMocks())
test.each(['0', '-1', '1x', '1/../../auth', '9007199254740993'])('invalid company ID %s never reaches the backend', async (id) => {
  const fetchMock = jest.spyOn(global, 'fetch')
  const response = await mapGET(new NextRequest('http://localhost/api/companies/1/correlation-map'), { params: Promise.resolve({ id }) })
  expect(response.status).toBe(400)
  expect(fetchMock).not.toHaveBeenCalled()
})
test('missing compare target is rejected; valid response and failure status pass through', async () => {
  const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValueOnce(new Response(JSON.stringify({ focus_company_id: 1 }), { status: 200 })).mockResolvedValueOnce(new Response(JSON.stringify({ error: 'not found' }), { status: 404 }))
  const params = { params: Promise.resolve({ id: '1' }) }
  expect((await compareGET(new NextRequest('http://localhost/api/companies/1/correlation-map/compare'), params)).status).toBe(400)
  const response = await compareGET(new NextRequest('http://localhost/api/companies/1/correlation-map/compare?with=2'), params)
  expect(await response.json()).toEqual({ focus_company_id: 1 })
  expect(fetchMock).toHaveBeenCalledWith(expect.stringContaining('/api/companies/1/correlation-map/compare?with=2'), expect.objectContaining({ cache: 'no-store' }))
  expect((await mapGET(new NextRequest('http://localhost/api/companies/1/correlation-map'), params)).status).toBe(404)
})
