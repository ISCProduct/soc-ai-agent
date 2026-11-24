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
        favoriteCompanies={[]}
        favoritesLoading={false}
        favoritesError={null}
        onRetryFavorites={noop}
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
        onUnfavoriteCompany={noop}
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
        favoriteCompanies={[]}
        favoritesLoading={false}
        favoritesError={null}
        onRetryFavorites={() => {}}
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
        onUnfavoriteCompany={() => {}}
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

  it('お気に入りだけを選ぶと、診断をまたいだ企業を業界と職種で並べる', () => {
    const favorited = {
      ...sampleCompany,
      id: '2',
      matchId: 11,
      name: 'お気に入り株式会社',
      industry: '製造',
      jobTitle: '生産管理',
      matchScore: 70,
      isFavorited: true,
      categoryScores: {
        technical: 81.2,
        teamwork: 0,
        leadership: 0,
        creativity: 0,
        stability: 0,
        growth: 0,
        work_life: 0,
        challenge: 0,
        detail: 0,
        communication: 0,
      },
    }
    const noJob = {
      ...sampleCompany,
      id: '3',
      matchId: 12,
      name: '職種なし株式会社',
      isFavorited: true,
      matchScore: 60,
    }
    renderList({
      companies: [sampleCompany],
      favoriteCompanies: [favorited, noJob],
    })

    fireEvent.click(screen.getByRole('button', { name: 'お気に入りだけ（2）' }))

    expect(screen.queryByText('サンプル株式会社')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'お気に入り株式会社' })).toBeInTheDocument()
    expect(screen.getByText('製造')).toBeInTheDocument()
    expect(screen.getByText('生産管理')).toBeInTheDocument()
    expect(screen.getByText('職種なし株式会社')).toBeInTheDocument()
    expect(screen.getByText('技術力')).toBeInTheDocument()
    expect(screen.getByText('81')).toBeInTheDocument()
    expect(screen.getAllByText('未設定').length).toBeGreaterThan(0)
    // 渡された順のまま。適合度の高いお気に入りが先。
    const names = screen.getAllByRole('button', { name: /株式会社$/ }).map((node) => node.textContent)
    expect(names).toEqual(['お気に入り株式会社', '職種なし株式会社'])
  })

  it('100社を超えるお気に入りは、次の100社へ進める', () => {
    const onFavoritesOffsetChange = jest.fn()
    renderList({
      favoriteCompanies: [sampleCompany],
      favoritesTotal: 130,
      favoritesOffset: 0,
      onFavoritesOffsetChange,
    })

    fireEvent.click(screen.getByRole('button', { name: 'お気に入りだけ（130）' }))
    expect(screen.getByText('お気に入りにした130社')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '次の100社' }))
    expect(onFavoritesOffsetChange).toHaveBeenCalledWith(100)
  })

  it('お気に入りが無いときは、すべての企業へ戻れる', () => {
    renderList()

    fireEvent.click(screen.getByRole('button', { name: 'お気に入りだけ（0）' }))
    expect(screen.getByText('お気に入りにした企業はまだありません')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'すべての企業を見る' }))
    expect(screen.getByText('サンプル株式会社')).toBeInTheDocument()
  })

  // #2088: 位置（index）を key にすると、並べ替えたときに React が位置で要素を使い回し、別の値の状態を引き継いでしまう
  it('技術スタックを並べ替えても、同じ値の要素は同じ DOM のまま移動する', () => {
    const { rerender } = renderList({ companies: [{ ...sampleCompany, techStack: ['Go', 'AWS'] }] })
    const awsBefore = screen.getByText('AWS')
    rerender(
      <ResultsListView
        companies={[{ ...sampleCompany, techStack: ['AWS', 'Go'] }]}
        favoriteCompanies={[]}
        favoritesLoading={false}
        favoritesError={null}
        onRetryFavorites={() => {}}
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
        onUnfavoriteCompany={() => {}}
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

  // #2221: 押せない理由は吹き出し（Tooltip）だけでなく、常に見える文で出し、aria-describedby で結ぶ
  describe('押せない理由の文', () => {
    const describedText = (button: HTMLElement) => {
      const id = button.getAttribute('aria-describedby')
      expect(id).toBeTruthy()
      return document.getElementById(id as string)?.textContent
    }

    it('ゲストには、メール送信と選考管理が使えない理由を画面の文で出す', () => {
      renderList({ isGuestUser: true })
      const emailButtons = screen.getAllByRole('button', { name: '結果をメールで受け取る' })
      expect(emailButtons).toHaveLength(2)
      for (const button of emailButtons) {
        expect(button).toBeDisabled()
        expect(describedText(button)).toBe('ゲストユーザーはメール送信できません。アカウント登録後にご利用ください。')
      }
      // 上下の2つのボタンは、別の文を指す（id が重ならない）
      expect(emailButtons[0].getAttribute('aria-describedby')).not.toBe(emailButtons[1].getAttribute('aria-describedby'))
      const applications = screen.getByRole('button', { name: '選考管理を見る' })
      expect(describedText(applications)).toContain('応募・選考管理を利用できません')
      // 登録への導線も近くにある
      expect(screen.getAllByRole('button', { name: 'アカウント登録して機能を解放' }).length).toBeGreaterThan(0)
    })

    it('ゲストの「応募する」は、行の近くの文で理由を示す', () => {
      renderList({ isGuestUser: true })
      const apply = screen.getByRole('button', { name: '応募する' })
      expect(apply).toBeDisabled()
      expect(describedText(apply)).toContain('応募・選考管理を利用できません')
    })

    it('応募が準備中なら、「応募は準備中」の理由を行の近くの文で示す', () => {
      renderList({ applyEnabled: false })
      const apply = screen.getByRole('button', { name: '応募は準備中' })
      expect(describedText(apply)).toBe('応募機能は現在準備中です。公開までお待ちください。')
      expect(screen.getByText('応募機能は現在準備中です。公開までお待ちください。')).toBeVisible()
    })

    it('通常のユーザーには理由の文を出さない（aria-describedby も付けない）', () => {
      renderList()
      expect(screen.queryByText(/アカウント登録後にご利用ください/)).not.toBeInTheDocument()
      expect(screen.queryByText(/準備中です/)).not.toBeInTheDocument()
      for (const name of ['結果をメールで受け取る', '応募する', '選考管理を見る']) {
        for (const button of screen.getAllByRole('button', { name })) {
          expect(button).not.toHaveAttribute('aria-describedby')
        }
      }
    })

    it('応募済みの企業には、応募が準備中でも理由の文を出さない', () => {
      renderList({ applyEnabled: false, companies: [{ ...sampleCompany, isApplied: true }] })
      expect(screen.getByRole('button', { name: '応募済み' })).not.toHaveAttribute('aria-describedby')
      expect(screen.queryByText('応募機能は現在準備中です。公開までお待ちください。')).not.toBeInTheDocument()
    })
  })
})
