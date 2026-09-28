/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import PageContent from '@/app/admin/student-insights/page-content'

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, email: 'teacher@example.com', is_admin: true }),
    getAdminFetchHeaders: () => ({}),
  },
}))

jest.mock('@/lib/admin/school-access', () => ({
  getAdminSchoolAccess: async () => ({ restricted: false, schools: [] }),
}))

const STUDENT = {
  user_id: 3,
  name: '生徒A',
  email: 'a@example.com',
  type_label: '安定志向',
  top_categories: [],
  suited_industries: [],
  low_match_applications: [{ company_name: 'X社', match_score: 20, status: 'applied' }],
  resume_status: { needs_attention: true, has_document: false, reason: '未提出' },
  data_available: true,
}

const originalFetch = global.fetch

afterEach(() => {
  global.fetch = originalFetch
})

test('片方の案内を送っても、もう片方の案内は送信できる', async () => {
  const posts: string[] = []
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url.includes('/guidances')) {
      posts.push(JSON.parse(String(init?.body)).kind)
      return { ok: true, status: 201, json: async () => ({}), text: async () => '{}' }
    }
    return { ok: true, status: 200, json: async () => ({ students: [STUDENT], total: 1 }) }
  })

  render(<PageContent />)
  fireEvent.click(await screen.findByRole('button', { name: '軌道修正' }))

  await waitFor(() => expect(screen.getByRole('button', { name: '送信済' })).toBeDisabled())
  const resumeButton = screen.getByRole('button', { name: '履歴書案内' })
  expect(resumeButton).toBeEnabled()

  fireEvent.click(resumeButton)
  await waitFor(() => expect(screen.getAllByRole('button', { name: '送信済' })).toHaveLength(2))
  expect(posts).toEqual(['low_match', 'resume'])
})
