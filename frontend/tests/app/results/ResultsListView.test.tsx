/**
 * @jest-environment jsdom
 */
import { render, screen, fireEvent, within } from '@testing-library/react'
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

  it('「合う／合わない」は、押すと判定を渡し、回答済みは押された状態で示す。省略すると出さない（SOCAIAGENT-670）', () => {
    const onFeedback = jest.fn()
    const { rerender } = renderList({ onFeedback })
    fireEvent.click(screen.getByRole('button', { name: '合わない' }))
    expect(onFeedback).toHaveBeenCalledWith(expect.anything(), expect.objectContaining({ matchId: 10 }), 'unfit')
    expect(screen.getByRole('button', { name: '合う' })).toHaveAttribute('aria-pressed', 'false')

    rerender(
      <ResultsListView
        companies={[{ ...sampleCompany, feedback: 'fit' }]}
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
        onFeedback={onFeedback}
        onApply={noop}
        onNavigate={noop}
      />,
    )
    expect(screen.getByRole('button', { name: '合う' })).toHaveAttribute('aria-pressed', 'true')
  })

  it('onFeedback が無いときは、「合う／合わない」を出さない', () => {
    renderList()
    expect(screen.queryByRole('button', { name: '合わない' })).toBeNull()
  })

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
        { ...sampleCompany, matchedAxisCount: 9 },
        { ...sampleCompany, id: '2', name: '根拠が薄い株式会社', matchedAxisCount: 6 },
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

  it('CTA行はflexWrapで折り返し、狭い画面でもボタンがはみ出さない', () => {
    renderList()
    const ctaButton = screen.getByRole('button', { name: 'ES添削・リライト' })
    // Stack(direction="row") の直下の行コンテナに flexWrap が効いていること
    const ctaRow = ctaButton.closest('.MuiStack-root')
    expect(ctaRow).toHaveStyle({ flexWrap: 'wrap' })
  })

  it('「ES添削・リライト」CTAはラベルどおり /es-rewrite へ企業名付きで遷移する', () => {
    const onNavigate = jest.fn()
    renderList({ onNavigate })

    fireEvent.click(screen.getByRole('button', { name: 'ES添削・リライト' }))
    expect(onNavigate).toHaveBeenCalledWith(
      expect.stringMatching(/^\/es-rewrite\?company_name=/),
    )
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

  // #2088: 位置（index）を key にすると、並べ替えたときに React が位置で要素を使い回し、別の値の状態を引き継いでしまう
  it('技術スタックを並べ替えても、同じ値の要素は同じ DOM のまま移動する', () => {
    const { rerender } = renderList({ companies: [{ ...sampleCompany, techStack: ['Go', 'AWS'] }] })
    const awsBefore = screen.getByText('AWS')
    rerender(
      <ResultsListView
        companies={[{ ...sampleCompany, techStack: ['AWS', 'Go'] }]}
        isProvisional={false}
        analysisScores={null}
        scoreComment=""
        analysisError={null}
        onRetryAnalysis={() => {}}
        jobSuitabilityComment=""
        suggestedRoles={[]}
        emailSending={false}
        favoritingId={null}
        applyingId={null}
        snackbar={{ open: false, message: '', severity: 'success' }}
        isGuestUser={false}
        applyEnabled
        onCloseSnackbar={() => {}}
        onBack={() => {}}
        onReset={() => {}}
        onSendEmail={() => {}}
        onSelectCompany={() => {}}
        onToggleFavorite={() => {}}
        onApply={() => {}}
        onNavigate={() => {}}
      />,
    )
    expect(screen.getByText('AWS')).toBe(awsBefore)
  })

  // 値から作った key は、同じ値が重なっても重ならない（React の警告が出ない）
  it('技術スタック・タグ・職種の提案に同じ値が重なっても、key の重複の警告を出さずに全部描く', () => {
    const errors = jest.spyOn(console, 'error').mockImplementation(() => {})
    try {
      renderList({
        companies: [{ ...sampleCompany, techStack: ['Go', 'Go', 'AWS'], tags: ['成長企業', '成長企業'] }],
        suggestedRoles: [
          { title: 'バックエンド', reason: 'a' },
          { title: 'バックエンド', reason: 'b' },
        ],
      })
      expect(screen.getAllByText('Go')).toHaveLength(2)
      expect(screen.getAllByText('成長企業')).toHaveLength(2)
      expect(screen.getAllByText('バックエンド')).toHaveLength(2)
      const keyWarnings = errors.mock.calls.filter((args) => String(args[0]).includes('same key'))
      expect(keyWarnings).toEqual([])
    } finally {
      errors.mockRestore()
    }
  })
})
