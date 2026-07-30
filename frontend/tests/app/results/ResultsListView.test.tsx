/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent } from '@testing-library/react'
import ResultsListView from '@/app/results/components/ResultsListView'
import type { Company } from '@/app/results/types'

const sampleCompany: Company = {
  id: '1',
  matchId: 10,
  name: 'サンプル株式会社',
  industry: 'IT・ソフトウェア',
  location: '東京都',
  employees: '100名',
  description: 'マッチ理由テキスト',
  matchScore: 92,
  tags: ['成長企業'],
  techStack: ['TypeScript'],
  isFavorited: false,
  isApplied: false,
}

describe('ResultsListView', () => {
  const noop = () => {}

  function renderList(overrides: Partial<React.ComponentProps<typeof ResultsListView>> = {}) {
    return render(
      <ResultsListView
        companies={[sampleCompany]}
        isProvisional={false}
        analysisScores={null}
        scoreComment=""
        jobSuitabilityComment=""
        suggestedRoles={[]}
        emailSending={false}
        favoritingId={null}
        applyingId={null}
        snackbar={{ open: false, message: '', severity: 'success' }}
        isGuestUser={false}
        onCloseSnackbar={noop}
        onBack={noop}
        onReset={noop}
        onSendEmail={noop}
        onSelectCompany={noop}
        onToggleFavorite={noop}
        onApply={noop}
        onNavigate={noop}
        {...overrides}
      />,
    )
  }

  it('企業名と適合度・応募ボタンを表示する', () => {
    renderList()

    expect(screen.getByText('サンプル株式会社')).toBeInTheDocument()
    expect(screen.getByText('92')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '応募する' })).toBeInTheDocument()
    expect(screen.getByText(/適合企業を1社に絞り込みました/)).toBeInTheDocument()
  })

  it('企業カードクリックで onSelectCompany を呼ぶ', () => {
    const onSelectCompany = jest.fn()
    renderList({ onSelectCompany })

    fireEvent.click(screen.getByText('サンプル株式会社'))
    expect(onSelectCompany).toHaveBeenCalledWith(sampleCompany)
  })

  it('暫定評価チップを表示する', () => {
    renderList({ isProvisional: true })
    expect(screen.getByText('暫定評価')).toBeInTheDocument()
  })
})
