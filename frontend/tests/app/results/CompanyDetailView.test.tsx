/**
 * @jest-environment jsdom
 */
import { useState } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import CompanyDetailView from '@/app/results/components/CompanyDetailView'
import type { Company } from '@/app/results/types'

// 関連図（React Flow）は描かず、どの図が出ているかだけ分かる代わりの部品にする
jest.mock('next/dynamic', () => () => {
  function DiagramStub({ type }: { type: string }) {
    return <div data-testid="relation-diagram">{type}</div>
  }
  return DiagramStub
})

const sampleCompany: Company = {
  id: '1',
  name: 'サンプル株式会社',
  industry: 'IT・ソフトウェア',
  location: '東京都',
  employees: '100名',
  description: 'マッチ理由テキスト',
  matchScore: 92,
  tags: ['成長企業'],
  techStack: ['TypeScript'],
}

function ControlledDetailView() {
  const [detailTab, setDetailTab] = useState(0)
  return (
    <CompanyDetailView
      company={sampleCompany}
      detailTab={detailTab}
      onDetailTabChange={setDetailTab}
      onBack={() => {}}
      relations={[]}
      marketInfo={[]}
      diagramLoading={false}
      diagramError={null}
    />
  )
}

function panelOf(tab: HTMLElement): HTMLElement {
  const panelId = tab.getAttribute('aria-controls')
  expect(panelId).toBeTruthy()
  const panel = document.getElementById(panelId as string)
  expect(panel).not.toBeNull()
  return panel as HTMLElement
}

describe('CompanyDetailView のタブと中身の対応', () => {
  it('各タブの aria-controls が tabpanel を指し、tabpanel の aria-labelledby がそのタブを指す', () => {
    render(<ControlledDetailView />)

    const tabs = screen.getAllByRole('tab')
    expect(tabs.map((t) => t.textContent)).toEqual(['基本情報', '資本関連図', 'ビジネス関連図'])

    const ids = new Set<string>()
    for (const tab of tabs) {
      const panel = panelOf(tab)
      expect(panel).toHaveAttribute('role', 'tabpanel')
      expect(panel).toHaveAttribute('aria-labelledby', tab.id)
      ids.add(tab.id)
      ids.add(panel.id)
    }
    // タブ3つと中身3つの id がすべて別
    expect(ids.size).toBe(6)
  })

  it('選ばれているタブの中身だけが表示され、タブを切り替えると対応する中身に替わる', () => {
    render(<ControlledDetailView />)
    const [basicTab, capitalTab, businessTab] = screen.getAllByRole('tab')

    expect(basicTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(basicTab)).toBeVisible()
    expect(panelOf(basicTab)).toHaveTextContent('この企業をすすめる理由')
    expect(panelOf(capitalTab)).not.toBeVisible()
    expect(panelOf(businessTab)).not.toBeVisible()
    expect(screen.queryByTestId('relation-diagram')).toBeNull()

    fireEvent.click(capitalTab)
    expect(capitalTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(capitalTab)).toBeVisible()
    expect(panelOf(capitalTab)).toHaveTextContent('capital')
    expect(panelOf(basicTab)).not.toBeVisible()
    expect(screen.queryByText('この企業をすすめる理由')).toBeNull()

    fireEvent.click(businessTab)
    expect(businessTab).toHaveAttribute('aria-selected', 'true')
    expect(panelOf(businessTab)).toBeVisible()
    expect(panelOf(businessTab)).toHaveTextContent('business')
    expect(panelOf(capitalTab)).not.toBeVisible()
    expect(screen.getAllByTestId('relation-diagram')).toHaveLength(1)
  })
})
