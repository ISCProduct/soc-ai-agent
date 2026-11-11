/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { NotificationBell } from '@/components/NotificationBell'
import { safeNotificationLink } from '@/lib/notifications'

// #2033: ヘッダのベルと未読バッジ。

const push = jest.fn()
jest.mock('next/navigation', () => ({
  usePathname: () => '/applications',
  useRouter: () => ({ push }),
}))
jest.mock('@/app/interview/sessionChrome', () => ({ useInterviewSessionActive: () => false }))

let storedUser: { is_admin?: boolean } | null = { is_admin: false }
jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => storedUser,
    getUserFetchHeaders: () => ({ 'X-User-Token': 'jwt' }),
  },
}))

const LIST = {
  unread_count: 2,
  notifications: [
    { id: 2, kind: 'application', title: '内定の連絡がありました', body: 'A社', link: '/applications', read_at: null, created_at: '2026-11-11T03:00:00Z' },
    { id: 1, kind: 'scout', title: '企業からスカウトが届きました', body: '', link: 'https://evil.example/', read_at: null, created_at: '2026-11-10T03:00:00Z' },
  ],
}

let calls: { url: string; method: string }[] = []
beforeEach(() => {
  storedUser = { is_admin: false }
  calls = []
  push.mockReset()
  global.fetch = jest.fn(async (url: RequestInfo | URL, init?: RequestInit) => {
    const u = String(url)
    calls.push({ url: u, method: init?.method ?? 'GET' })
    if (u.includes('unread-count')) return { ok: true, status: 200, json: async () => ({ unread_count: 2 }) }
    if (u.startsWith('/api/notifications?')) return { ok: true, status: 200, json: async () => LIST }
    return { ok: true, status: 204, json: async () => ({}) }
  }) as unknown as typeof fetch
})

test('通知の遷移先はアプリ内のパスだけ', () => {
  expect(safeNotificationLink('/applications')).toBe('/applications')
  expect(safeNotificationLink('https://evil.example/')).toBeNull()
  expect(safeNotificationLink('//evil.example/')).toBeNull()
})

test('未読の件数をバッジに出し、開くと一覧を出す', async () => {
  render(<NotificationBell />)
  const bell = await screen.findByRole('button', { name: '通知（未読 2 件）' })
  fireEvent.click(bell)
  expect(await screen.findByText('内定の連絡がありました')).toBeInTheDocument()
  expect(screen.getByText('企業からスカウトが届きました')).toBeInTheDocument()
})

test('通知を開くと既読にして、アプリ内のリンク先へ移る', async () => {
  render(<NotificationBell />)
  fireEvent.click(await screen.findByRole('button', { name: '通知（未読 2 件）' }))
  fireEvent.click(await screen.findByText('内定の連絡がありました'))
  await waitFor(() => expect(calls.some((c) => c.url === '/api/notifications/2/read' && c.method === 'POST')).toBe(true))
  expect(push).toHaveBeenCalledWith('/applications')

  // 外部 URL の通知は既読にするだけで、移動しない
  push.mockReset()
  fireEvent.click(screen.getByText('企業からスカウトが届きました'))
  await waitFor(() => expect(calls.some((c) => c.url === '/api/notifications/1/read')).toBe(true))
  expect(push).not.toHaveBeenCalled()
})

test('すべて既読にすると未読のバッジが消える', async () => {
  render(<NotificationBell />)
  fireEvent.click(await screen.findByRole('button', { name: '通知（未読 2 件）' }))
  await screen.findByText('内定の連絡がありました')
  fireEvent.click(screen.getByRole('button', { name: 'すべて既読にする' }))
  await waitFor(() => expect(calls.some((c) => c.url === '/api/notifications/read-all' && c.method === 'POST')).toBe(true))
  // ポップアップを開いている間、ベルは aria-hidden になる
  expect(await screen.findByRole('button', { name: '通知', hidden: true })).toBeInTheDocument()
})

test('管理者には出さない', () => {
  storedUser = { is_admin: true }
  render(<NotificationBell />)
  expect(screen.queryByRole('button', { name: /通知/ })).not.toBeInTheDocument()
})
