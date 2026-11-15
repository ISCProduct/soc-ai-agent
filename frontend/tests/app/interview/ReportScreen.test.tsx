/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import ReportScreen from '@/app/interview/components/ReportScreen'
import { UTTERANCE_SAVE_FAILED_MESSAGE } from '@/app/interview/utteranceSave'
import { REPORT_RETRY_FAILED_MESSAGE } from '@/app/interview/hooks/useInterviewSession'

describe('ReportScreen', () => {
  const noop = () => {}

  function renderScreen(
    overrides: Partial<React.ComponentProps<typeof ReportScreen>> = {},
  ) {
    render(
      <ReportScreen
        onPracticeAgain={noop}
        errorMessage={null}
        reportStatus="pending"
        report={null}
        scoresBefore={null}
        scoresAfter={null}
        session={null}
        userId={1}
        emailSending={false}
        emailSent={false}
        emailError=""
        onSendEmail={noop}
        isGuest={false}
        onRegisterClick={noop}
        videoUploadStatus="idle"
        videoUploadProgress={0}
        videoSizeWarning={null}
        {...overrides}
      />
    )
  }

  // #1476: 発話保存が落ちたまま面接が終わると、レポートが欠ける/生成されない。
  // その理由が画面に出ないとユーザーも運用側も原因に辿り着けない。
  it('発話保存に失敗していたら、その旨を警告として表示する', () => {
    renderScreen({ utteranceSaveFailed: true })

    expect(screen.getByText(UTTERANCE_SAVE_FAILED_MESSAGE)).toBeInTheDocument()
  })

  it('発話保存に失敗していなければ警告を出さない', () => {
    renderScreen()

    expect(screen.queryByText(UTTERANCE_SAVE_FAILED_MESSAGE)).not.toBeInTheDocument()
  })

  it('見出し「面接おつかれさまでした」と、取れた項目だけの概要行を出す', () => {
    renderScreen({ questionCount: 8, companyName: '港南システムズ' })

    expect(screen.getByText('面接おつかれさまでした')).toBeInTheDocument()
    // 時間・職種は取れていないので出さない（捏造しない）
    expect(screen.getByText('質問8問 港南システムズ')).toBeInTheDocument() // 全角空白は照合時に正規化される
  })

  it('面接履歴への導線を表示する（#1011: レポート画面から/interview/historyへ遷移できる）', () => {
    renderScreen()

    expect(screen.getByRole('link', { name: '面接履歴へ' })).toHaveAttribute(
      'href',
      '/interview/history',
    )
  })

  it('進捗が null（旧バックエンド）なら段階リスト無しの不定表示にする', () => {
    renderScreen({ reportStatus: 'pending', reportProgress: null })

    expect(screen.getByText('レポートを作っています')).toBeInTheDocument()
    expect(screen.queryByText('順番待ち')).not.toBeInTheDocument()
    expect(screen.queryByRole('progressbar', { name: 'レポート作成の進み具合' })).not.toBeInTheDocument()
  })

  it('作成中は%と段階を出し、「レポートを見る」は無効', () => {
    renderScreen({ reportProgress: { stage: 'generating', percent: 42, stalled: false } })

    expect(screen.getByText('42%')).toBeInTheDocument()
    expect(screen.getByRole('progressbar', { name: 'レポート作成の進み具合' })).toHaveAttribute('aria-valuenow', '42')
    expect(screen.getByText('回答の分析')).toBeInTheDocument()
    expect(screen.getByText('進行中')).toBeInTheDocument()
    expect(screen.getByText('分析中の%は、進み具合の目安です')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'レポートを見る' })).toBeDisabled()
  })

  it('停滞中は見出しと案内が変わる', () => {
    renderScreen({ reportProgress: { stage: 'generating', percent: 42, stalled: true } })

    expect(screen.getByText('時間がかかっています')).toBeInTheDocument()
    expect(screen.getByRole('status')).toHaveTextContent('いつもより時間がかかっています')
  })

  it('動画アップロードは進捗カードの中の1行で出す', () => {
    renderScreen({
      reportProgress: { stage: 'queued', percent: 0, stalled: false },
      videoUploadStatus: 'uploading',
      videoUploadProgress: 60,
    })

    expect(screen.getByText('動画のアップロード')).toBeInTheDocument()
    expect(screen.getByText('60%')).toBeInTheDocument()
  })

  it('サーバー失敗なら「もう一度試す」が出て、押すと onRetryReport が呼ばれる', () => {
    const onRetryReport = jest.fn()
    renderScreen({ reportStatus: 'error', reportFailedByServer: true, onRetryReport })

    expect(screen.getByText('レポートを作れませんでした')).toBeInTheDocument()
    expect(screen.getByText(/回答はすべて保存されています/)).toBeInTheDocument()
    screen.getByRole('button', { name: 'もう一度試す' }).click()
    expect(onRetryReport).toHaveBeenCalledTimes(1)
  })

  it('「もう一度練習する」で onPracticeAgain が呼ばれる', () => {
    const onPracticeAgain = jest.fn()
    renderScreen({ onPracticeAgain })

    screen.getByRole('button', { name: 'もう一度練習する' }).click()
    expect(onPracticeAgain).toHaveBeenCalledTimes(1)
  })

  it('動画サイズ警告を表示する', () => {
    renderScreen({ videoSizeWarning: '動画が大きいです' })

    expect(screen.getByText('動画が大きいです')).toBeInTheDocument()
  })

  it('timeout の場合はタイムアウト文言と再試行ボタンを表示する', () => {
    const onRetryReport = jest.fn()
    renderScreen({ reportStatus: 'timeout', onRetryReport })

    expect(
      screen.getByText('レポート生成がタイムアウトしました。時間をおいて再試行してください。'),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再試行' })).toBeInTheDocument()
  })

  it('error の場合は失敗文言と再試行ボタンを表示する', () => {
    renderScreen({ reportStatus: 'error', onRetryReport: jest.fn() })

    expect(screen.getByText('レポート生成に失敗しました。')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '再試行' })).toBeInTheDocument()
  })

  // #1476: 再生成リクエスト自体が失敗したのに黙ってポーリングへ戻すと、
  // ユーザーは「生成中」を見せられたまま再びタイムアウトするだけになる。
  it('再生成リクエストが失敗したら、そのエラーを表示する', () => {
    renderScreen({
      reportStatus: 'timeout',
      onRetryReport: jest.fn(),
      reportRetryError: REPORT_RETRY_FAILED_MESSAGE,
    })

    expect(screen.getByText(REPORT_RETRY_FAILED_MESSAGE)).toBeInTheDocument()
  })

  it('再試行ボタン押下で onRetryReport が呼ばれる', () => {
    const onRetryReport = jest.fn()
    renderScreen({ reportStatus: 'timeout', onRetryReport })

    screen.getByRole('button', { name: '再試行' }).click()
    expect(onRetryReport).toHaveBeenCalledTimes(1)
  })

  it('finishFailed の場合、finishSession失敗のエラー文言と再試行ボタンを表示する(#1015)', () => {
    const onRetryFinish = jest.fn()
    renderScreen({
      errorMessage: '面接の終了処理に失敗しました。お手数ですが再試行してください。',
      finishFailed: true,
      onRetryFinish,
    })

    expect(
      screen.getByText('面接の終了処理に失敗しました。お手数ですが再試行してください。'),
    ).toBeInTheDocument()
    const retryButton = screen.getByRole('button', { name: '再試行' })
    retryButton.click()
    expect(onRetryFinish).toHaveBeenCalledTimes(1)
  })

  it('finishFailed が false の場合、errorMessageがあっても再試行ボタンは出さない', () => {
    renderScreen({
      errorMessage: '時間上限に達したため面接を終了しました。',
      finishFailed: false,
    })

    expect(screen.getByText('時間上限に達したため面接を終了しました。')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '再試行' })).not.toBeInTheDocument()
  })
})

describe('ReportScreen レポート完成', () => {
  it('レポート本体と「レポートを見る」（有効）を出す', () => {
    render(
      <ReportScreen
        onPracticeAgain={() => {}}
        errorMessage={null}
        reportStatus="ready"
        report={{
          session_id: 1,
          summary_text: '結論から話せています。次は数字を入れましょう。',
          scores_json: '{}',
          evidence_json: '{}',
          created_at: '2026-09-07',
          updated_at: '2026-09-07',
        }}
        scoresBefore={null}
        scoresAfter={null}
        session={null}
        emailSending={false}
        emailSent={false}
        emailError=""
        onSendEmail={() => {}}
        isGuest
        onRegisterClick={() => {}}
        videoUploadStatus="idle"
        videoUploadProgress={0}
        videoSizeWarning={null}
      />,
    )

    expect(screen.getByText('レポートができました')).toBeInTheDocument()
    expect(screen.getByText('結論から話せています。')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'レポートを見る' })).toHaveAttribute('href', '#interview-report-body')
    // ゲストはメール送信不可＋登録導線
    expect(screen.getByRole('button', { name: 'レポートをメールで受け取る' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'アカウント登録して機能を解放' })).toBeInTheDocument()
  })
})

describe('ReportScreen メール送信エラー表示 (#1056)', () => {
  const noop = () => {}

  // メール送信ボタンは reportStatus==='ready' かつ report ありのときだけ描画される
  const READY_REPORT = {
    session_id: 1,
    summary_text: '要約',
    scores_json: '{}',
    evidence_json: '{}',
    created_at: '2026-09-07',
    updated_at: '2026-09-07',
  }

  function renderWithEmailError(emailError: string) {
    render(
      <ReportScreen
        onPracticeAgain={noop}
        errorMessage={null}
        reportStatus="ready"
        report={READY_REPORT}
        scoresBefore={null}
        scoresAfter={null}
        session={null}
        userId={1}
        emailSending={false}
        emailSent={false}
        emailError={emailError}
        onSendEmail={noop}
        onRetryReport={noop}
        onRegisterClick={noop}
        isGuest={false}
        videoUploadStatus="idle"
        videoUploadProgress={0}
        videoSizeWarning={null}
      />,
    )
  }

  it('emailError があればメッセージを表示する', () => {
    renderWithEmailError('メールの送信に失敗しました。時間をおいて再度お試しください。')

    expect(
      screen.getByText('メールの送信に失敗しました。時間をおいて再度お試しください。'),
    ).toBeInTheDocument()
  })

  it('emailError が空なら何も表示しない', () => {
    renderWithEmailError('')

    expect(screen.queryByText(/送信に失敗/)).not.toBeInTheDocument()
  })

  it('送信失敗後もボタンは押せる状態に戻っている', () => {
    renderWithEmailError('メールの送信に失敗しました。時間をおいて再度お試しください。')

    expect(screen.getByRole('button', { name: 'レポートをメールで受け取る' })).not.toBeDisabled()
  })
})
