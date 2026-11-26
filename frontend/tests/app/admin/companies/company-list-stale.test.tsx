/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import PageContent from '@/app/admin/companies/page-content'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'admin@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

jest.mock('@/components/admin/SchoolFilterSelect', () => ({
  SchoolFilterSelect: () => null,
}))

function jsonRes(body: unknown) {
  const raw = JSON.stringify(body)
  return { ok: true, status: 200, text: async () => raw, json: async () => JSON.parse(raw) }
}

describe('企業一覧の後着レスポンス', () => {
  it('絞り込みを変えたあと、古い一覧が新しい結果を上書きしない', async () => {
    let releaseFirst: (value: unknown) => void = () => {}
    let listCalls = 0
    global.fetch = jest.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/admin/companies?')) {
        listCalls += 1
        if (listCalls === 1) {
          return new Promise((resolve) => {
            releaseFirst = resolve
          })
        }
        return jsonRes({
          companies: [{ id: 2, name: '新しい企業', industry: '製造', data_status: 'draft', is_active: true }],
          total: 1,
        })
      }
      return jsonRes({})
    }) as unknown as typeof fetch

    render(<PageContent />)
    await waitFor(() => expect(listCalls).toBeGreaterThanOrEqual(1))

    fireEvent.click(screen.getByRole('button', { name: '公開前' }))
    expect(await screen.findByText('新しい企業')).toBeInTheDocument()

    await act(async () => {
      releaseFirst(jsonRes({
        companies: [{ id: 1, name: '古い企業', industry: 'IT', data_status: 'published', is_active: true }],
        total: 1,
      }))
    })

    expect(screen.getByText('新しい企業')).toBeInTheDocument()
    expect(screen.queryByText('古い企業')).not.toBeInTheDocument()
  })
})
