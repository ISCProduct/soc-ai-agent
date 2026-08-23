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

  it('再試行ボタン押下で onRetryReport が呼ばれる', () => {
    const onRetryReport = jest.fn()
    renderScreen({ reportStatus: 'timeout', onRetryReport })

    screen.getByRole('button', { name: '再試行' }).click()
    expect(onRetryReport).toHaveBeenCalledTimes(1)
  })
})
