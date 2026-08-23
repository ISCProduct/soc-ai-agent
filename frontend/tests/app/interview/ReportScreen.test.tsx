/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import ReportScreen from '@/app/interview/components/ReportScreen'

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

  it('タイトル「面接レポート」を表示する', () => {
    renderScreen()

    expect(screen.getByText('面接レポート')).toBeInTheDocument()
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
