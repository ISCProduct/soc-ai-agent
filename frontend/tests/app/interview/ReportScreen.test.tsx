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
})
