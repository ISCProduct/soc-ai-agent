/**
 * @jest-environment jsdom
 */
import { render, screen } from '@testing-library/react'

jest.mock('@/app/interview/components/ThreeAvatar', () => ({
  __esModule: true,
  default: () => <div data-testid="three-avatar-mock" />,
}))

import SessionScreen from '@/app/interview/components/SessionScreen'
import { UTTERANCE_SAVE_FAILED_MESSAGE } from '@/app/interview/utteranceSave'

describe('SessionScreen', () => {
  const noop = () => {}

  function renderScreen(
    overrides: Partial<React.ComponentProps<typeof SessionScreen>> = {},
  ) {
    render(
      <SessionScreen
        status="connected"
        companyName="テスト株式会社"
        userName="テストユーザー"
        isActive={true}
        isConnected={true}
        remainingSeconds={600}
        sessionWarningShown={false}
        currentQuestionIndex={1}
        totalQuestionCount={8}
        questionProgress={0}
        questionRemainingSeconds={180}
        questionRemainingLabel="あと3分で次の質問へ"
        isDeepeningQuestion={false}
        questionCategory={null}
        avatarGender="male"
        aiLevel={0}
        aiSpeaking={false}
        cameraEnabled={true}
        captionsVisible={true}
        handsFreeMode={false}
        utterances={[]}
        partialAi=""
        partialUser=""
        isRecording={false}
        turnPending={false}
        errorMessage={null}
        sessionVideoCallbackRef={noop}
        transcriptEndRef={{ current: null }}
        aiAudioRef={{ current: null }}
        consentDialog={null}
        onToggleCamera={noop}
        onToggleCaptions={noop}
        onToggleHandsFree={noop}
        onStartRecording={noop}
        onStopRecording={noop}
        onJoin={noop}
        onStop={noop}
        {...overrides}
      />
    )
  }

  it('企業名と面接官ラベルを表示する', () => {
    renderScreen()

    expect(screen.getByText('テスト株式会社')).toBeInTheDocument()
    expect(screen.getByText('テスト株式会社 面接官')).toBeInTheDocument()
  })

  it('status=connecting の場合は接続中オーバーレイを表示する', () => {
    renderScreen({ status: 'connecting', isConnected: false })

    expect(screen.getByText('接続中...')).toBeInTheDocument()
  })

  it('接続済みの場合は「話す」ボタンを表示する', () => {
    renderScreen()

    expect(screen.getByRole('button', { name: '話す' })).toBeInTheDocument()
  })

  // #1476: 保存に失敗したまま無言で面接が続くと、ユーザーは空のレポートを受け取るまで気づけない
  it('発話保存に失敗したら面接中でも警告を表示する', () => {
    renderScreen({ utteranceSaveFailed: true })

    expect(screen.getByText(UTTERANCE_SAVE_FAILED_MESSAGE)).toBeInTheDocument()
  })

  it('発話保存に失敗していなければ警告を出さない', () => {
    renderScreen()

    expect(screen.queryByText(UTTERANCE_SAVE_FAILED_MESSAGE)).not.toBeInTheDocument()
  })
})
