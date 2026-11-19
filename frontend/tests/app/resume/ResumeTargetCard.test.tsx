/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import { ResumeTargetCard } from '@/app/resume/components/ResumeTargetCard'

const noop = () => {}

function renderCard(companyQuery: string) {
  return render(
    <ResumeTargetCard
      mode="company"
      onModeChange={noop}
      companyQuery={companyQuery}
      onCompanyQueryChange={noop}
      companyName=""
      companyValidated={false}
      companySearchLoading={false}
      companySearchError=""
      selectedCompanyMeta={null}
      companyCandidates={[]}
      onSearchCompanies={noop}
      onSelectCompany={noop}
      jobTitle=""
      onJobTitleChange={noop}
      candidateType="new_grad"
      onCandidateTypeChange={noop}
    />,
  )
}

describe('ResumeTargetCard', () => {
  // 掲載企業で見つからなかったとき（候補0件）に、Web 検索へ進む入口が消えると、未登録の企業を選べない
  it('企業名を入力していて候補が0件でも「Web から探す」を出す', () => {
    renderCard('未登録の会社')
    expect(screen.getByRole('button', { name: 'Web から探す' })).toBeInTheDocument()
  })

  it('何も入力していないうちは、職種だけで見る案内を出す', () => {
    renderCard('')
    expect(screen.queryByRole('button', { name: 'Web から探す' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '職種だけで見る' })).toBeInTheDocument()
  })
})
