/**
 * @jest-environment jsdom
 */
import { readFileSync } from 'fs'
import { join } from 'path'
import { fireEvent, render, screen, within } from '@testing-library/react'
import { APPLICATION_STATUSES, STATUS_ICONS, STATUS_LABELS } from '@/lib/application-status'
import { severityConfig, getSeverityConfig } from '@/lib/severity'
import FormatFindingsSection from '@/components/feedback/FormatFindingsSection'
import { LoginPage } from '@/components/auth/LoginPage'
import ApplicationsPage from '@/app/applications/page-content'
import SchedulePage from '@/app/schedule/page-content'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => '/login',
}))

jest.mock('@/lib/auth', () => ({
  authService: {
    getStoredUser: () => ({ user_id: 1, is_guest: false }),
    getUserFetchHeaders: () => ({}),
  },
}))

const originalFetch = global.fetch
afterEach(() => {
  global.fetch = originalFetch
})

function mockFetchJson(byUrl: Record<string, unknown>) {
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    const body = byUrl[String(input)] ?? {}
    return { ok: true, status: 200, json: async () => body }
  }) as unknown as typeof fetch
}

// #2218: 状態を色だけでなく「色＋字形＋文字」で伝える（白黒・高コントラストでも区別できる）
describe('応募のステータスの字形', () => {
  it('すべてのステータスに字形と文字がある', () => {
    for (const s of APPLICATION_STATUSES) {
      expect(STATUS_ICONS[s]).toBeDefined()
      expect(STATUS_LABELS[s]).toBeTruthy()
    }
  })

  it('段階の違うステータスは、違う字形にする', () => {
    const stages = ['applied', 'document_screening', 'document_passed', 'interview_scheduled', 'interview_in_progress', 'offered', 'accepted', 'withdrawn', 'rejected']
    const icons = new Set(stages.map((s) => STATUS_ICONS[s]))
    expect(icons.size).toBe(stages.length)
  })

  it('応募一覧の札に、ステータスの文字と字形（svg）を出す', async () => {
    mockFetchJson({
      '/api/applications': {
        applications: [
          { id: 1, company_id: 10, company_name: 'A社', status: 'document_passed', applied_at: '2026-11-01T00:00:00Z' },
          { id: 2, company_id: 20, company_name: 'B社', status: 'offered', applied_at: '2026-11-02T00:00:00Z' },
        ],
      },
    })
    render(<ApplicationsPage />)
    for (const status of ['document_passed', 'offered']) {
      const label = await screen.findByText(STATUS_LABELS[status])
      const chip = label.closest('.MuiChip-root')
      expect(chip).not.toBeNull()
      expect(chip?.querySelector('svg')).not.toBeNull()
    }
  })
})

describe('指摘の重要度の字形', () => {
  it('重大・注意・情報は、それぞれ違う字形にする', () => {
    const icons = new Set(Object.values(severityConfig).map((c) => c.Icon))
    expect(icons.size).toBe(3)
    expect(getSeverityConfig('unknown').Icon).toBeDefined()
  })

  it('指摘の札に字形（svg）と文字がある', () => {
    render(
      <FormatFindingsSection
        findings={[
          { rule: 'r', severity: 'critical', start: 0, end: 1, message: 'm1', fix: '' },
          { rule: 'r', severity: 'warning', start: 1, end: 2, message: 'm2', fix: '' },
          { rule: 'r', severity: 'info', start: 2, end: 3, message: 'm3', fix: '' },
        ]}
      />,
    )
    for (const label of ['重大', '注意', '情報']) {
      const chip = screen.getByText(label).closest('.MuiChip-root')
      expect(chip?.querySelector('svg')).not.toBeNull()
    }
  })
})

describe('メールの形式の誤り', () => {
  it('入力欄を離れると、次にすることを含む文と字形を出す', () => {
    render(<LoginPage onAuthSuccess={() => {}} />)
    const email = screen.getAllByLabelText(/メールアドレス/)[0]
    fireEvent.change(email, { target: { value: 'student.example.com' } })
    fireEvent.blur(email)
    const message = screen.getByText('メールアドレスの形式が正しくありません。name@example.com のように入力してください')
    expect(message.querySelector('svg')).not.toBeNull()
  })

  it('@ はあってもドメインのドットが無い入力にも、同じ正しい形の例を出す', () => {
    render(<LoginPage onAuthSuccess={() => {}} />)
    const email = screen.getAllByLabelText(/メールアドレス/)[0]
    fireEvent.change(email, { target: { value: 'student@example' } })
    fireEvent.blur(email)
    expect(screen.getByText('メールアドレスの形式が正しくありません。name@example.com のように入力してください')).toBeInTheDocument()
  })
})

describe('カレンダーの帯', () => {
  it('帯に、色だけでなく種類の略称を出す', async () => {
    // 今月の 10 日・11 日（UTC 03:00。日本時間でも UTC でも同じ日になる）
    const now = new Date()
    const day = (d: number) => new Date(Date.UTC(now.getFullYear(), now.getMonth(), d, 3, 0)).toISOString()
    mockFetchJson({
      '/api/schedule': [
        { id: 1, user_id: 1, company_name: 'A社', stage: '1次面接', title: '', scheduled_at: day(10), notes: '' },
        { id: 2, user_id: 1, company_name: 'B社', stage: '内定', title: '', scheduled_at: day(11), notes: '' },
      ],
    })
    render(<SchedulePage />)
    const shorts = await screen.findAllByTestId('schedule-stage-short')
    expect(shorts.map((el) => el.textContent).sort()).toEqual(['1次', '内定'])
    // 略称は帯の中で、企業名と一緒に出る
    expect(within(shorts.find((el) => el.textContent === '1次')!.parentElement!).getByText(/A社/)).toBeInTheDocument()
  })

  // 新しい種類を足したときに略称を付け忘れないよう、定義はソースで確かめる
  const src = readFileSync(join(__dirname, '..', '..', 'app', 'schedule', 'page-content.tsx'), 'utf8')

  it('全部の種類に略称がある', () => {
    const options = src.match(/const STAGE_OPTIONS = \[([\s\S]*?)\n\]/)?.[1] ?? ''
    const rows = options.split('\n').filter((l) => l.includes('value:'))
    expect(rows.length).toBeGreaterThan(0)
    for (const row of rows) expect(row).toMatch(/short: '[^']+'/)
  })
})
