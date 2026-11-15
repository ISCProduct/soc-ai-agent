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

  it('企業カードクリックで onSelectCompany を呼ぶ', () => {
    const onSelectCompany = jest.fn()
    renderList({ onSelectCompany })

    fireEvent.click(screen.getByText('サンプル株式会社'))
    expect(onSelectCompany).toHaveBeenCalledWith(sampleCompany)
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
    expect(screen.getByText(/根拠 6軸/)).toBeInTheDocument()
    // 同じ値が並ぶだけの行には印を出さない
    expect(screen.queryByText(/根拠 9軸/)).not.toBeInTheDocument()
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

  it('タブで お気に入り / 応募済み に絞り込める。0件なら案内を出す', () => {
    renderList({
      companies: [
        { ...sampleCompany, name: 'A社', isFavorited: true },
        { ...sampleCompany, id: '2', name: 'B社', isApplied: true },
        { ...sampleCompany, id: '3', name: 'C社' },
      ],
    })
    expect(screen.getByRole('tab', { name: 'すべて 3' })).toHaveAttribute('aria-selected', 'true')

    fireEvent.click(screen.getByRole('tab', { name: 'お気に入り 1' }))
    expect(screen.getByText('A社')).toBeInTheDocument()
    expect(screen.queryByText('B社')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: '応募済み 1' }))
    expect(screen.getByText('B社')).toBeInTheDocument()
    expect(screen.queryByText('A社')).not.toBeInTheDocument()
  })

  it('絞り込み結果が0件のとき「該当する企業がありません」を出す', () => {
    renderList()
    fireEvent.click(screen.getByRole('tab', { name: 'お気に入り 0' }))
    expect(screen.getByText('該当する企業がありません')).toBeInTheDocument()
  })

  it('噛み合った軸を上位3つ表示する。categoryScores が無ければ出さない', () => {
    renderList({ companies: [{ ...sampleCompany, categoryScores: axisScores }] })
    expect(screen.getByText('噛み合った軸')).toBeInTheDocument()
    expect(screen.getByText('成長意欲 97')).toBeInTheDocument()
    expect(screen.getByText('チームワーク 95')).toBeInTheDocument()
    expect(screen.getByText('挑戦意欲 77')).toBeInTheDocument()
  })

  it('categoryScores が無い行には「噛み合った軸」を出さない', () => {
    renderList()
    expect(screen.queryByText('噛み合った軸')).not.toBeInTheDocument()
  })

  it('50未満の軸があるとき、最低の軸名でチャットへ戻る導線を出す', () => {
    const onBack = jest.fn()
    renderList({ analysisScores: { job: 100, interest: 100, aptitude: 55, future: 20 }, scoreComment: 'コメント', onBack })
    fireEvent.click(screen.getByRole('button', { name: /チャットで将来の話を続ける/ }))
    expect(onBack).toHaveBeenCalledTimes(1)
  })

  it('全軸が50以上なら続ける導線を出さない', () => {
    renderList({ analysisScores: { job: 80, interest: 80, aptitude: 80, future: 80 }, scoreComment: 'コメント' })
    expect(screen.queryByRole('button', { name: /の話を続ける/ })).not.toBeInTheDocument()
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

    it('「どこまで聞けているか」は閉じており、開閉でき、閉じていても軸の値が見える', () => {
      renderList({
        analysisScores: { job: 100, interest: 100, aptitude: 55, future: 20 },
        scoreComment: 'コメント',
        suggestedRoles: [{ title: 'データ基盤エンジニア', reason: '粘り強さ' }],
      })
      const toggle = screen.getByRole('button', { name: 'どこまで聞けているか' })
      expect(toggle).toHaveAttribute('aria-expanded', 'false')
      expect(screen.getByText('将来 20')).toBeInTheDocument()
      expect(screen.queryByText('データ基盤エンジニア')).not.toBeInTheDocument()

      fireEvent.click(toggle)
      expect(toggle).toHaveAttribute('aria-expanded', 'true')
      expect(screen.getByText('データ基盤エンジニア')).toBeInTheDocument()
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
