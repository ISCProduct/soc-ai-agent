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

  it('許可エラー時も video 要素を残し再試行ボタンを表示する', () => {
    const { container } = render(
      <LobbyScreen
        userName="テストユーザー"
        companyName="テスト株式会社"
        interviewCompany={{ id: 1, name: 'テスト株式会社' }}
        fromMatchingResults={false}
        lobbyPermissionError="カメラへのアクセスが拒否されました。"
        onRetryPermissions={noop}
        micEnabled={true}
        cameraEnabled={true}
        onToggleMic={noop}
        onToggleCamera={noop}
        lobbyVideoRef={{ current: null }}
        onBack={noop}
        onJoinWithConsent={noop}
        consentDialog={null}
      />,
    )

    expect(screen.getByText(/カメラへのアクセスが拒否されました/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /再試行/ })).toBeInTheDocument()
    expect(container.querySelector('video')).not.toBeNull()
  })
  // #1058: リンク風の見た目だけで押しても何も起きない項目を置いていた。
  // 「実在するページだけを本物のリンクで出す」を守れているかを見る。
  it('フッターには実在するページへのリンクだけが出る', () => {
    renderScreen()

    const footer = screen.getByRole('contentinfo')
    const links = Array.from(footer.querySelectorAll('a'))
    expect(links.map((a) => a.getAttribute('href'))).toEqual(['/privacy'])
    // ページの無い項目は、リンクでもテキストでも置かない（押せそうに見せない）
    expect(footer.textContent).not.toContain('利用規約')
    expect(footer.textContent).not.toContain('ヘルプ')
  })
})
