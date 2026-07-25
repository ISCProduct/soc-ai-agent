/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'
import LobbyScreen from '@/app/interview/components/LobbyScreen'

describe('LobbyScreen', () => {
  const noop = () => {}

  function renderScreen(
    overrides: Partial<React.ComponentProps<typeof LobbyScreen>> = {},
  ) {
    render(
      <LobbyScreen
        userName="テストユーザー"
        companyName="テスト株式会社"
        interviewCompany={{ id: 1, name: 'テスト株式会社', industry: 'IT' }}
        fromMatchingResults={false}
        lobbyPermissionError={null}
        onRetryPermissions={noop}
        micEnabled={true}
        cameraEnabled={true}
        onToggleMic={noop}
        onToggleCamera={noop}
        lobbyVideoRef={{ current: null }}
        onBack={noop}
        onJoinWithConsent={noop}
        consentDialog={null}
        {...overrides}
      />
    )
  }

  it('準備確認のタイトルと参加ボタンを表示する', () => {
    renderScreen()

    expect(screen.getByText('準備はできましたか？')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '面接に参加' })).toBeInTheDocument()
  })
})
