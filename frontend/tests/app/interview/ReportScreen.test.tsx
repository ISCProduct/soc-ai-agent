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
        onBack={noop}
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

  it('タイトル「面接レポート」を表示する', () => {
    renderScreen()

    expect(screen.getByText('面接レポート')).toBeInTheDocument()
  })

  it('面接履歴への導線を表示する（#1011: レポート画面から/interview/historyへ遷移できる）', () => {
    renderScreen()

    expect(screen.getByRole('link', { name: /面接履歴を見る/ })).toHaveAttribute(
      'href',
      '/interview/history',
    )
  })

  it('pending の場合はレポート生成中の文言を表示する', () => {
    renderScreen({ reportStatus: 'pending' })

    expect(screen.getByText('レポートを生成中です...')).toBeInTheDocument()
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
        onBack={noop}
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
