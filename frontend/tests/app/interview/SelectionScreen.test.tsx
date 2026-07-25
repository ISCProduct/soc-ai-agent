/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import SelectionScreen from '@/app/interview/components/SelectionScreen'
import { POSITIONS } from '@/app/interview/constants'
import type { User } from '@/lib/auth'

describe('SelectionScreen', () => {
  const user: User = { user_id: 1, name: 'テストユーザー' } as User

  const noop = () => {}

  function renderScreen(
    overrides: Partial<React.ComponentProps<typeof SelectionScreen>> = {},
  ) {
    const setSelectedPosition = jest.fn()
    render(
      <SelectionScreen
        user={user}
        onBack={noop}
        interviewCompany={null}
        setInterviewCompany={noop}
        companySourceTab="db"
        setCompanySourceTab={noop}
        companySearch=""
        setCompanySearch={noop}
        allCompanies={[]}
        setAllCompanies={noop}
        companiesLoading={false}
        webSearchResults={[]}
        setWebSearchResults={noop}
        webSearchLoading={false}
        positionCategory="general"
        setPositionCategory={noop}
        selectedPosition={POSITIONS[0]}
        setSelectedPosition={setSelectedPosition}
        companyHints={null}
        hintsLoading={false}
        questionDurationSeconds={180}
        onStartInterview={noop}
        {...overrides}
      />
    )
    return { setSelectedPosition }
  }

  it('最小限の props でステップタイトルと企業名を表示する', () => {
    renderScreen()

    expect(screen.getByText('InterviewAI')).toBeInTheDocument()
    expect(screen.getByText('練習する企業・職種を選ぶ')).toBeInTheDocument()
    expect(screen.getByText('企業未選択')).toBeInTheDocument()
    expect(screen.getAllByText(POSITIONS[0].title).length).toBeGreaterThan(0)
  })

  it('職種カードをキーボード（Enter）で選択できる', () => {
    const { setSelectedPosition } = renderScreen()
    const radios = screen.getAllByRole('radio')
    expect(radios.length).toBeGreaterThan(1)

    fireEvent.keyDown(radios[1], { key: 'Enter' })
    expect(setSelectedPosition).toHaveBeenCalled()
  })
})
