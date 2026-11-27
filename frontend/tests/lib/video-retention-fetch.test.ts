/**
 * @jest-environment node
 */
import { fetchVideoRetention } from '@/lib/interview/video-retention'

jest.mock('@/lib/auth', () => ({
  authService: {
    getUserFetchHeaders: () => ({ 'X-Tenant': 'demo' }),
  },
}))

it('保持期間は BFF 経由で取り、Backend を直接叩かない', async () => {
  const fetchMock = jest.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ days: 90, unlimited: false }),
  })
  global.fetch = fetchMock as unknown as typeof fetch

  await expect(fetchVideoRetention()).resolves.toEqual({ days: 90, unlimited: false })
  expect(fetchMock).toHaveBeenCalledTimes(1)
  expect(String(fetchMock.mock.calls[0][0])).toBe('/api/bff/user/interview-video-retention')
})
