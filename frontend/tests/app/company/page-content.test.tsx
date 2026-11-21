/**
 * @jest-environment jsdom
 */
import { useEffect } from 'react'
import { fireEvent, render, screen } from '@testing-library/react'
import PageContent from '@/app/company/[id]/page-content'

jest.mock('next/navigation', () => ({
  useParams: () => ({ id: '1' }),
  useRouter: () => ({ back: jest.fn(), push: jest.fn() }),
}))

let mockDiagramMounts = 0

// 相関図（React Flow とデータ取得）は描かず、どの図が出ているかと作り直しの回数だけ分かる代わりの部品にする
jest.mock('next/dynamic', () => () => {
  function DiagramStub({ diagramType }: { diagramType: string }) {
    useEffect(() => {
      mockDiagramMounts += 1
    }, [])
    return <div data-testid="company-diagram">{diagramType}</div>
  }
  return DiagramStub
})

const originalFetch = global.fetch

beforeEach(() => {
  mockDiagramMounts = 0
  global.fetch = jest.fn().mockImplementation(async (input: RequestInfo | URL) => {
    if (String(input) === '/api/companies/1') {
      return { ok: true, status: 200, json: async () => ({ id: 1, name: 'サンプル株式会社' }) }
    }
    // 募集職種は無くてよい
    return { ok: false, status: 404, json: async () => ({}) }
  }) as unknown as typeof fetch
})

afterEach(() => {
  global.fetch = originalFetch
})

describe('企業ページ（/company/[id]）の関連図タブと中身の対応', () => {
  it('各タブの aria-controls が tabpanel を指し、tabpanel の aria-labelledby が選ばれているタブを指す', async () => {
    render(<PageContent />)
    await screen.findByText('サンプル株式会社')

    const tabs = screen.getAllByRole('tab')
    expect(tabs.map((t) => t.textContent)).toEqual(['資本関連図', 'ビジネス関連図'])
    expect(tabs[0].id).not.toBe(tabs[1].id)

    for (const tab of tabs) {
      const panelId = tab.getAttribute('aria-controls')
      expect(panelId).toBeTruthy()
      expect(document.getElementById(panelId as string)).toHaveAttribute('role', 'tabpanel')
    }

    const panel = screen.getByRole('tabpanel')
    expect(tabs[0]).toHaveAttribute('aria-selected', 'true')
    expect(panel).toHaveAttribute('aria-labelledby', tabs[0].id)
  })

  it('タブを切り替えると中身が対応する図に替わり、tabpanel の aria-labelledby も替わる。図は作り直さない', async () => {
    render(<PageContent />)
    await screen.findByText('サンプル株式会社')
    const [capitalTab, businessTab] = screen.getAllByRole('tab')
    const panel = screen.getByRole('tabpanel')

    expect(panel).toHaveTextContent('実線：子会社')
    expect(screen.getByTestId('company-diagram')).toHaveTextContent('capital')

    fireEvent.click(businessTab)
    expect(businessTab).toHaveAttribute('aria-selected', 'true')
    expect(capitalTab).toHaveAttribute('aria-selected', 'false')
    expect(screen.getByRole('tabpanel')).toBe(panel)
    expect(panel).toHaveAttribute('aria-labelledby', businessTab.id)
    expect(panel).toHaveTextContent('青い矢印：ビジネス取引関係')
    expect(screen.getByTestId('company-diagram')).toHaveTextContent('business')
    // 切り替えでデータの取得と「読み込み中」がやり直しにならない
    expect(mockDiagramMounts).toBe(1)
  })
})
