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

  it('DB一覧に名前一致があるとき id:0 の仮選択を登録企業へ昇格する', () => {
    const setInterviewCompany = jest.fn((updater) => {
      if (typeof updater === 'function') {
        return updater({ id: 0, name: '登録株式会社' })
      }
      return updater
    })
    const registered = { id: 42, name: '登録株式会社', industry: 'IT' }

    renderScreen({
      companySearch: '登録株式会社',
      interviewCompany: { id: 0, name: '登録株式会社' },
      allCompanies: [registered],
      setInterviewCompany,
    })

    expect(setInterviewCompany).toHaveBeenCalled()
    const updater = setInterviewCompany.mock.calls[0][0]
    expect(typeof updater).toBe('function')
    expect(updater({ id: 0, name: '登録株式会社' })).toEqual(registered)
  })

  it('開始ボタン押下時に id:0 なら企業解決を確定してから進む', async () => {
    const onStartInterview = jest.fn()
    const setInterviewCompany = jest.fn()
    const registered = { id: 7, name: '開始前解決株式会社' }

    global.fetch = jest.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ companies: [registered] }),
    }) as unknown as typeof fetch

    renderScreen({
      companySearch: '開始前解決株式会社',
      interviewCompany: { id: 0, name: '開始前解決株式会社' },
      allCompanies: [],
      setInterviewCompany,
      onStartInterview,
    })

    fireEvent.click(screen.getByRole('button', { name: '面接を開始する' }))

    await screen.findByRole('button', { name: '面接を開始する' })
    await Promise.resolve()
    await Promise.resolve()

    expect(setInterviewCompany).toHaveBeenCalledWith(
      expect.objectContaining({ id: 7, name: '開始前解決株式会社' }),
    )
    expect(onStartInterview).toHaveBeenCalled()
  })
})
