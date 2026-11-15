/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'
import LobbyScreen from '@/app/interview/components/LobbyScreen'

describe('LobbyScreen', () => {
  const noop = () => {}

  function renderScreen(overrides: Partial<React.ComponentProps<typeof LobbyScreen>> = {}) {
    return render(
      <LobbyScreen
        userName="テストユーザー"
        companyName="テスト株式会社"
        positionTitle="SE"
        totalQuestionCount={8}
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
        consentGiven={false}
        onConsentChange={noop}
        onJoin={noop}
        {...overrides}
      />,
    )
  }

  it('見出し・企業名／職種・質問数を表示する', () => {
    renderScreen()

    expect(screen.getByText('面接の準備ができました')).toBeInTheDocument()
    expect(screen.getByText('テスト株式会社 ／ SE')).toBeInTheDocument()
    expect(screen.getByText(/質問8問/)).toBeInTheDocument()
  })

  it('未同意だと「面接を始める」は押せない', () => {
    const onJoin = jest.fn()
    renderScreen({ onJoin })

    const button = screen.getByRole('button', { name: '面接を始める' })
    expect(button).toBeDisabled()
    fireEvent.click(button)
    expect(onJoin).not.toHaveBeenCalled()
  })

  it('同意のチェックは onConsentChange に伝わる', () => {
    const onConsentChange = jest.fn()
    renderScreen({ onConsentChange })

    fireEvent.click(screen.getByRole('checkbox'))
    expect(onConsentChange).toHaveBeenCalledWith(true)
  })

  it('同意済みなら押せて onJoin が呼ばれる', () => {
    const onJoin = jest.fn()
    renderScreen({ consentGiven: true, onJoin })

    fireEvent.click(screen.getByRole('button', { name: '面接を始める' }))
    expect(onJoin).toHaveBeenCalledTimes(1)
  })

  it('「練習内容を変える」と「戻る」は onBack を呼ぶ', () => {
    const onBack = jest.fn()
    renderScreen({ onBack })

    fireEvent.click(screen.getByRole('button', { name: '練習内容を変える' }))
    fireEvent.click(screen.getByRole('button', { name: '前の画面へ戻る' }))
    expect(onBack).toHaveBeenCalledTimes(2)
  })

  it('「詳しく見る」で同意内容が開閉する', () => {
    renderScreen()

    const toggle = screen.getByRole('button', { name: '詳しく見る' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(toggle)
    expect(screen.getByRole('button', { name: '閉じる' })).toHaveAttribute('aria-expanded', 'true')
  })

  it('今回の目標は値があるときだけ出す', () => {
    const { unmount } = renderScreen()
    expect(screen.queryByText('今回の目標')).toBeNull()
    unmount()

    renderScreen({ focusGoal: { text: '経験に数字を入れて話す' } })
    expect(screen.getByText('今回の目標')).toBeInTheDocument()
    expect(screen.getByText('経験に数字を入れて話す')).toBeInTheDocument()
  })

  it('未登録企業は警告を出す', () => {
    renderScreen({ interviewCompany: { id: 0, name: '未登録会社' } })
    expect(screen.getByText('カスタム質問・深掘りは無効です')).toBeInTheDocument()
  })

  it('マイク／カメラの切替は状態を文言と aria-pressed で示す', () => {
    const onToggleMic = jest.fn()
    renderScreen({ micEnabled: false, onToggleMic })

    const mic = screen.getByRole('button', { name: /マイク.*現在オフ/ })
    expect(mic).toHaveAttribute('aria-pressed', 'false')
    expect(mic).toHaveTextContent('マイク オフ')
    fireEvent.click(mic)
    expect(onToggleMic).toHaveBeenCalled()
  })

  it('許可エラー時もエラーと再試行を表示し、video 要素を残す', () => {
    const onRetryPermissions = jest.fn()
    const { container } = renderScreen({
      lobbyPermissionError: 'カメラへのアクセスが拒否されました。',
      onRetryPermissions,
    })

    expect(screen.getByRole('alert')).toHaveTextContent('カメラへのアクセスが拒否されました')
    fireEvent.click(screen.getByRole('button', { name: /再試行/ }))
    expect(onRetryPermissions).toHaveBeenCalled()
    expect(container.querySelector('video')).not.toBeNull()
  })

  // #1058: リンク風の見た目だけで押しても何も起きない項目を置いていた。
  // 「実在するページだけを本物のリンクで出す」を守れているかを見る。
  it('フッターには実在するページへのリンクだけが出る', () => {
    renderScreen()

    const footer = screen.getByRole('contentinfo')
    const links = Array.from(footer.querySelectorAll('a'))
    expect(links.map((a) => a.getAttribute('href'))).toEqual(['/privacy'])
    expect(footer.textContent).not.toContain('利用規約')
    expect(footer.textContent).not.toContain('ヘルプ')
  })
})
