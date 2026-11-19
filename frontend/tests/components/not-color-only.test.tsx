/**
 * @jest-environment jsdom
 */
import { readFileSync } from 'fs'
import { join } from 'path'
import { fireEvent, render, screen } from '@testing-library/react'
import { APPLICATION_STATUSES, STATUS_ICONS, STATUS_LABELS } from '@/lib/application-status'
import { severityConfig, getSeverityConfig } from '@/lib/severity'
import FormatFindingsSection from '@/components/feedback/FormatFindingsSection'
import { LoginPage } from '@/components/auth/LoginPage'

jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), replace: jest.fn() }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => '/login',
}))

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
    const message = screen.getByText('メールアドレスの形式が正しくありません。@ を含めて入力してください')
    expect(message.querySelector('svg')).not.toBeNull()
  })
})

describe('カレンダーの帯', () => {
  // 画面のテストが無いので、ソースで確かめる（帯に種類の略称を出し、全部の種類に略称がある）
  const src = readFileSync(join(__dirname, '..', '..', 'app', 'schedule', 'page-content.tsx'), 'utf8')

  it('帯に種類の略称を出す', () => {
    expect(src).toMatch(/\{stageShort\(ev\.stage\)\}/)
  })

  it('全部の種類に略称がある', () => {
    const options = src.match(/const STAGE_OPTIONS = \[([\s\S]*?)\n\]/)?.[1] ?? ''
    const rows = options.split('\n').filter((l) => l.includes('value:'))
    expect(rows.length).toBeGreaterThan(0)
    for (const row of rows) expect(row).toMatch(/short: '[^']+'/)
  })
})
