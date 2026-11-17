/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, within } from '@testing-library/react'
import ResultsListView from '@/app/results/components/ResultsListView'
import type { CategoryScores, Company } from '@/app/results/types'

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

const axisScores: CategoryScores = {
  technical: 10, teamwork: 95, leadership: 10, creativity: 10, stability: 10,
  growth: 97, work_life: 10, challenge: 77, detail: 10, communication: 10,
}

/** jsdom には matchMedia が無いので、min-width クエリを desktop かどうかで答える */
function setDesktop(desktop: boolean) {
  window.matchMedia = ((query: string) => ({
    matches: desktop && query.includes('min-width'),
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    onchange: null,
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

describe('ResultsListView', () => {
  const noop = () => {}
  beforeEach(() => setDesktop(true))

  function renderList(overrides: Partial<React.ComponentProps<typeof ResultsListView>> = {}) {
    return render(
      <ResultsListView
        companies={[sampleCompany]}
        isProvisional={false}
        analysisScores={null}
        scoreComment=""
        analysisError={null}
        onRetryAnalysis={noop}
        jobSuitabilityComment=""
        suggestedRoles={[]}
        emailSending={false}
        favoritingId={null}
        applyingId={null}
        snackbar={{ open: false, message: '', severity: 'success' }}
        isGuestUser={false}
        applyEnabled
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
    expect(screen.getByText(/あなたに近い1社/)).toBeInTheDocument()
  })

  it('「詳しく見る」と企業名のどちらからも詳細を開ける', () => {
    const onSelectCompany = jest.fn()
    renderList({ onSelectCompany })

    fireEvent.click(screen.getByRole('button', { name: 'サンプル株式会社の詳細を見る' }))
    fireEvent.click(screen.getByText('サンプル株式会社'))
    expect(onSelectCompany).toHaveBeenCalledTimes(2)
    expect(onSelectCompany).toHaveBeenCalledWith(sampleCompany)
  })

  it('絞り込みのタブや位置バーは出さない（Figma v4）', () => {
    renderList()
    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByRole('img', { name: /候補全体の幅の中での位置/ })).not.toBeInTheDocument()
  })

  // チップは廃止した。同じことを注意書きが書いており、
  // 色のついたピルは情報を足していなかった。注意書き本文で確認する。
  it('根拠が薄いときは注意書きを表示する', () => {
    renderList({ isProvisional: true, diagnosisSummary: '選択のみの回答が多いです' })
    expect(screen.getByText(/選択のみの回答が多いです/)).toBeInTheDocument()
    expect(screen.getByText(/選択のみの回答が多いです/)).toBeInTheDocument()
    expect(screen.getByText(/会話を続けると根拠が増え/)).toBeInTheDocument()
  })

  // 全行に根拠軸数を出すのはやめた。ほとんどの行が同じ値になり
  // 読み飛ばされるため、相対的に薄い行だけ印を出す。
  it('他より根拠が薄い行にだけ印を出す', () => {
    renderList({
      companies: [
        { ...sampleCompany, matchedAxisCount: 9, categoryScores: axisScores },
        { ...sampleCompany, id: '2', name: '根拠が薄い株式会社', matchedAxisCount: 6, categoryScores: axisScores },
      ],
    })
    expect(screen.getByText(/根拠にできた項目が6つです（他の候補より少なめ）/)).toBeInTheDocument()
    // 同じ値が並ぶだけの行には印を出さない
    expect(screen.queryByText(/根拠にできた項目が9つ/)).not.toBeInTheDocument()
  })

  it('分析データ取得失敗時に警告と再読み込みボタンを表示する', () => {
    const onRetryAnalysis = jest.fn()
    renderList({ analysisError: '分析データの取得に失敗しました', onRetryAnalysis })

    expect(screen.getByText(/分析データの取得に失敗しました/)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '再読み込み' }))
    expect(onRetryAnalysis).toHaveBeenCalledTimes(1)
  })

  it('分析エラーがない場合は警告を表示しない', () => {
    renderList()
    expect(screen.queryByRole('button', { name: '再読み込み' })).not.toBeInTheDocument()
  })

  it('「その他」メニューから関連企業・ES添削へ遷移し、行の詳細は開かない', () => {
    const onNavigate = jest.fn()
    const onSelectCompany = jest.fn()
    renderList({ onNavigate, onSelectCompany })

    fireEvent.click(screen.getByRole('button', { name: /のその他の操作/ }))
    fireEvent.click(screen.getByRole('menuitem', { name: 'ES添削・リライト' }))
    expect(onNavigate).toHaveBeenCalledWith(expect.stringMatching(/^\/es-rewrite\?company_name=/))

    fireEvent.click(screen.getByRole('button', { name: /のその他の操作/ }))
    fireEvent.click(screen.getByRole('menuitem', { name: '関連企業を見る' }))
    expect(onNavigate).toHaveBeenCalledWith('/correlation-diagram?company_id=1')
    expect(onSelectCompany).not.toHaveBeenCalled()
  })

  it('「面接を練習する」は /interview へ遷移し、行の詳細は開かない', () => {
    const onNavigate = jest.fn()
    const onSelectCompany = jest.fn()
    renderList({ onNavigate, onSelectCompany })

    fireEvent.click(screen.getByRole('button', { name: '面接を練習する' }))
    expect(onNavigate).toHaveBeenCalledWith(expect.stringMatching(/^\/interview\?company_id=1/))
    expect(onSelectCompany).not.toHaveBeenCalled()
  })

  it('一致している項目を、上位3つの名前と点数で出す。categoryScores が無ければ出さない', () => {
    const { unmount } = renderList({ companies: [{ ...sampleCompany, categoryScores: axisScores }] })
    expect(screen.getByText('一致している項目')).toBeInTheDocument()
    expect(screen.getByText(/成長意欲 97\s+チームワーク 95\s+挑戦意欲 77/)).toBeInTheDocument()
    unmount()

    renderList()
    expect(screen.queryByText('一致している項目')).not.toBeInTheDocument()
  })

  describe('次にやること', () => {
    it('いちばん聞けていない軸を挙げ、チャットで続ける導線を出す', () => {
      const onBack = jest.fn()
      renderList({ analysisScores: { job: 100, interest: 100, aptitude: 55, future: 20 }, onBack })

      expect(screen.getByText('将来の話がまだ少ないため、チャットで続けると順位が変わることがあります。')).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: 'チャットで続きを話す' }))
      expect(onBack).toHaveBeenCalledTimes(1)
    })

    it('全軸が「よく聞けています」なら、続ける導線は出さず企業の確認を促す', () => {
      renderList({ analysisScores: { job: 80, interest: 90, aptitude: 100, future: 80 } })
      expect(screen.getByText(/気になる企業を「詳しく見る」で確認しましょう/)).toBeInTheDocument()
      expect(screen.queryByRole('button', { name: 'チャットで続きを話す' })).not.toBeInTheDocument()
    })

    it('暫定評価なら、会話を続けると順位が変わることを伝える', () => {
      renderList({ isProvisional: true, analysisScores: { job: 100, interest: 100, aptitude: 100, future: 100 } })
      expect(screen.getAllByText(/会話を続けると根拠が増え、順番も変わります/).length).toBeGreaterThan(0)
      expect(screen.getByRole('button', { name: 'チャットで続きを話す' })).toBeInTheDocument()
    })

    it('分析を取得できていなくて暫定評価でもなければ、出さない', () => {
      renderList()
      expect(screen.queryByText('次にやること')).not.toBeInTheDocument()
    })
  })

  it('「いまの回答でわかっていること」を、字形とことばで4軸とも示す', () => {
    renderList({ analysisScores: { job: 100, interest: 85, aptitude: 55, future: 20 } })

    const panel = screen.getByRole('region', { name: 'いまの回答でわかっていること' })
    expect(within(panel).getAllByText('よく聞けています')).toHaveLength(2)
    expect(within(panel).getByText('やや少ない')).toBeInTheDocument()
    expect(within(panel).getByText('まだ少ない')).toBeInTheDocument()
    // 棒グラフの数値は出さない
    expect(within(panel).queryByText('20')).not.toBeInTheDocument()
  })

  it('向いていそうな職種は、コメントも職種も無ければ出さない', () => {
    renderList()
    expect(screen.queryByText('向いていそうな職種')).not.toBeInTheDocument()
    renderList({ suggestedRoles: [{ title: 'データ基盤エンジニア', reason: '粘り強さ' }] })
    expect(screen.getByText('データ基盤エンジニア')).toBeInTheDocument()
  })

  it('ゲストはメール送信と選考管理が無効になる', () => {
    renderList({ isGuestUser: true })
    expect(screen.getByRole('button', { name: '結果をメールで受け取る' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '選考管理を見る' })).toBeDisabled()
  })

  describe('SP', () => {
    beforeEach(() => setDesktop(false))

    it('わかっていること → 企業 → 向いていそうな職種 の順に縦に並ぶ', () => {
      renderList({
        analysisScores: { job: 100, interest: 100, aptitude: 55, future: 20 },
        suggestedRoles: [{ title: 'データ基盤エンジニア', reason: '粘り強さ' }],
      })
      const coverage = screen.getByRole('region', { name: 'いまの回答でわかっていること' })
      const roles = screen.getByRole('region', { name: '向いていそうな職種' })
      const list = screen.getByRole('region', { name: 'あなたに近い企業' })
      expect(coverage.compareDocumentPosition(list) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
      expect(list.compareDocumentPosition(roles) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    it('「わかっていること」は4軸を横に並べ、狭い列に合わせて短い文言にする', () => {
      renderList({ analysisScores: { job: 100, interest: 100, aptitude: 55, future: 20 } })
      const panel = screen.getByRole('region', { name: 'いまの回答でわかっていること' })
      expect(within(panel).getAllByText('よく聞けた')).toHaveLength(2)
      expect(within(panel).queryByText('よく聞けています')).not.toBeInTheDocument()
    })

    it('メール送信ボタンは1つだけ出る', () => {
      renderList()
      expect(screen.getAllByRole('button', { name: '結果をメールで受け取る' })).toHaveLength(1)
    })
  })

  it('応募成功snackbarに選考管理(/applications)へのリンクがある', () => {
    const onNavigate = jest.fn()
    renderList({
      onNavigate,
      snackbar: {
        open: true,
        message: 'サンプル株式会社 に応募しました',
        severity: 'success',
        actionHref: '/applications',
        actionLabel: '選考管理を見る',
      },
    })

    const alert = screen.getByRole('alert')
    const linkButton = within(alert).getByRole('button', { name: '選考管理を見る' })
    fireEvent.click(linkButton)
    expect(onNavigate).toHaveBeenCalledWith('/applications')
  })

  it('actionHrefがないsnackbarにはリンクボタンを表示しない', () => {
    renderList({
      snackbar: { open: true, message: 'お気に入りに追加しました', severity: 'success' },
    })
    const alert = screen.getByRole('alert')
    expect(within(alert).queryByRole('button', { name: '選考管理を見る' })).not.toBeInTheDocument()
  })

  // SOCAIAGENT-403: 応募を閉じている間は「準備中」で無効にする。応募済みの表示は変えない。
  it('応募を閉じている間は「応募は準備中」で無効になり、押しても onApply を呼ばない', () => {
    const onApply = jest.fn()
    renderList({ applyEnabled: false, onApply })

    const button = screen.getByRole('button', { name: '応募は準備中' })
    expect(button).toBeDisabled()
    expect(screen.queryByRole('button', { name: '応募する' })).not.toBeInTheDocument()
    fireEvent.click(button)
    expect(onApply).not.toHaveBeenCalled()
  })

  it('応募を閉じていても、応募済みの企業は「応募済み」のまま表示する', () => {
    renderList({ applyEnabled: false, companies: [{ ...sampleCompany, isApplied: true }] })

    expect(screen.getByRole('button', { name: '応募済み' })).toBeDisabled()
    expect(screen.queryByText('応募は準備中')).not.toBeInTheDocument()
  })
})
