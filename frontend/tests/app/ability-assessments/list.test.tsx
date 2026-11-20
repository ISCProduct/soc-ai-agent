/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import PageContent from '@/app/ability-assessments/page-content'
import { abilityApi, type AbilitySummary } from '@/lib/ability'

const back = jest.fn()
jest.mock('next/navigation', () => ({
  useRouter: () => ({ push: jest.fn(), back }),
}))
jest.mock('@/lib/auth', () => ({ authService: { getUserFetchHeaders: () => ({}) } }))
jest.mock('@/lib/ability', () => ({
  ...jest.requireActual('@/lib/ability'),
  abilityApi: jest.fn(),
}))
jest.mock('@/components/common/BottomNavSpacer', () => ({ BottomNavSpacer: () => null }))

const api = abilityApi as jest.Mock

const item: AbilitySummary = {
  template_id: 1,
  title: '能力検査（言語・非言語）',
  description: '',
  question_count: 10,
  status: 'not_started',
  answered_count: 0,
}

describe('能力検査の一覧', () => {
  it('前の画面へ戻れる', async () => {
    api.mockResolvedValue({ assessments: [item] })
    render(<PageContent />)
    expect(await screen.findByRole('heading', { name: '能力検査' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '前の画面へ戻る' }))
    expect(back).toHaveBeenCalled()
  })
})
