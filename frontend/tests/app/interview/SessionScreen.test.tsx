/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen, within } from '@testing-library/react'

import SessionScreen from '@/app/interview/components/SessionScreen'
import { UTTERANCE_SAVE_FAILED_MESSAGE } from '@/app/interview/utteranceSave'
import { setInterviewSessionActive, useInterviewSessionActive } from '@/app/interview/sessionChrome'
import { renderHook } from '@testing-library/react'

describe('SessionScreen', () => {
  const noop = () => {}

  function renderScreen(
    overrides: Partial<React.ComponentProps<typeof SessionScreen>> = {},
  ) {
    return render(
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

  it('面接官の名札を表示する', () => {
    renderScreen()
    expect(screen.getByText('テスト株式会社 面接官')).toBeInTheDocument()
  })

  it('status=connecting の場合は接続中オーバーレイを表示する', () => {
    renderScreen({ status: 'connecting', isConnected: false })
    expect(screen.getByText('接続中...')).toBeInTheDocument()
  })

  it('エラー時はメッセージと「再接続する」を出し、押すと onJoin が呼ばれる', () => {
    const onJoin = jest.fn()
    renderScreen({ status: 'error', isConnected: false, errorMessage: '接続に失敗しました', onJoin })
    expect(screen.getByText('接続に失敗しました')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '再接続する' }))
    expect(onJoin).toHaveBeenCalledTimes(1)
  })

  it('接続済みの場合は「話す」ボタンを表示し、押すと録音を始める', () => {
    const onStartRecording = jest.fn()
    renderScreen({ onStartRecording })
    fireEvent.click(screen.getByRole('button', { name: '話す' }))
    expect(onStartRecording).toHaveBeenCalledTimes(1)
  })

  it('録音中は「話し終わった」になり、押すと録音を止める', () => {
    const onStopRecording = jest.fn()
    renderScreen({ isRecording: true, onStopRecording })
    expect(screen.queryByRole('button', { name: '話す' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '話し終わった' }))
    expect(onStopRecording).toHaveBeenCalledTimes(1)
  })

  it('面接官が話している間は録音ボタンを押せない', () => {
    renderScreen({ aiSpeaking: true })
    expect(screen.getByRole('button', { name: '質問を聞いています' })).toBeDisabled()
  })

  it('面接官が考えている間は録音ボタンを押せない', () => {
    renderScreen({ turnPending: true })
    expect(screen.getByRole('button', { name: '考えています' })).toBeDisabled()
  })

  it('終了ボタンは確認を挟み、確認して初めて onStop(false) が呼ばれる', () => {
    const onStop = jest.fn()
    renderScreen({ onStop })
    fireEvent.click(screen.getByRole('button', { name: '終了' }))
    expect(onStop).not.toHaveBeenCalled()
    expect(screen.getByRole('dialog', { name: '面接を終了しますか？' })).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: '終了してレポートを見る' }))
    expect(onStop).toHaveBeenCalledWith(false)
  })

  it('確認ダイアログで「面接を続ける」を選ぶと onStop は呼ばれない', () => {
    const onStop = jest.fn()
    renderScreen({ onStop })
    fireEvent.click(screen.getByRole('button', { name: '終了' }))
    fireEvent.click(screen.getByRole('button', { name: '面接を続ける' }))
    expect(onStop).not.toHaveBeenCalled()
  })

  it('transcriptEndRef が DOM に付く（useInterviewSession の自動スクロール先）', () => {
    const transcriptEndRef = { current: null as HTMLDivElement | null }
    renderScreen({ transcriptEndRef })
    expect(transcriptEndRef.current).toBeInstanceOf(HTMLDivElement)
  })

  it('aiAudioRef の audio と、自分の映像の video ref が付く', () => {
    const aiAudioRef = { current: null as HTMLAudioElement | null }
    const sessionVideoCallbackRef = jest.fn()
    renderScreen({ aiAudioRef, sessionVideoCallbackRef })
    expect(aiAudioRef.current).toBeInstanceOf(HTMLAudioElement)
    expect(sessionVideoCallbackRef).toHaveBeenCalledWith(expect.any(HTMLVideoElement))
  })

  it('深掘り質問のときはその旨を表示する', () => {
    renderScreen({ isDeepeningQuestion: true })
    expect(screen.getByText('深掘り質問')).toBeInTheDocument()
  })

  it('字幕オフでも質問文だけを隠し、状況行は残す', () => {
    renderScreen({ captionsVisible: false, utterances: [{ role: 'ai', text: '強みは？' }] })
    expect(screen.queryByText('強みは？')).not.toBeInTheDocument()
    expect(screen.getByText(/あなたの番です/)).toBeInTheDocument()
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

  it('テキスト継続時は文字回答欄を出し、面接官発話中は送信できない', () => {
    const onSubmitTextAnswer = jest.fn()
    renderScreen({
      textContinueMode: true,
      voiceDegradeText: '面接官の音声を再生できません。',
      aiSpeaking: true,
      onSubmitTextAnswer,
    })
    expect(screen.getByText('面接官の音声を再生できません。')).toBeInTheDocument()
    expect(screen.getByLabelText('文字で回答')).toBeDisabled()
    expect(screen.getByRole('button', { name: '送信' })).toBeDisabled()
  })

  it('テキスト継続で待機中なら文字回答を送れる', () => {
    const onSubmitTextAnswer = jest.fn()
    renderScreen({ textContinueMode: true, onSubmitTextAnswer })
    const input = screen.getByLabelText('文字で回答')
    fireEvent.change(input, { target: { value: 'チームで開発しました' } })
    fireEvent.click(screen.getByRole('button', { name: '送信' }))
    expect(onSubmitTextAnswer).toHaveBeenCalledWith('チームで開発しました')
  })

  // #1569: セッション画面表示中は学生下部ナビを隠す。SelectionScreen/LobbyScreenと
  // 同一パス(/interview)のため、マウント中かどうかで判定する。
  it('マウント中はsetInterviewSessionActiveをtrueにし、アンマウントでfalseに戻す', () => {
    setInterviewSessionActive(false)
    const flag = renderHook(() => useInterviewSessionActive())
    expect(flag.result.current).toBe(false)

    const { unmount } = renderScreen()
    expect(flag.result.current).toBe(true)

    unmount()
    expect(flag.result.current).toBe(false)
  })

  describe('現在の質問', () => {
    it('確定済みのAI発話を現在の質問として表示する', () => {
      renderScreen({ utterances: [{ role: 'ai', text: 'あなたの強みを教えてください' }] })
      expect(screen.getByText('あなたの強みを教えてください')).toBeInTheDocument()
    })

    it('読み上げ中(partialAi)があればそちらを優先して表示する', () => {
      renderScreen({ utterances: [{ role: 'ai', text: '前の質問' }], partialAi: '読み上げ中の質問です' })
      expect(screen.getByText('読み上げ中の質問です')).toBeInTheDocument()
      expect(screen.queryByText('前の質問')).not.toBeInTheDocument()
    })
  })

  describe('サイドパネル', () => {
    it('既定では閉じている', () => {
      renderScreen()
      expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
    })

    it('「ヒント」でSTARの4項目を表示し、閉じるで消える', () => {
      renderScreen()
      fireEvent.click(screen.getByRole('button', { name: 'ヒント' }))
      const panel = screen.getByRole('complementary', { name: 'ヒント' })
      for (const label of ['状況', '課題', '行動', '結果']) expect(within(panel).getByText(label)).toBeInTheDocument()
      fireEvent.click(screen.getByRole('button', { name: '閉じる' }))
      expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
    })

    it('「記録」でこれまでの質問・回答を表示する', () => {
      renderScreen({
        utterances: [{ role: 'ai', text: '1つ目の質問' }, { role: 'user', text: '1つ目の回答' }],
      })
      fireEvent.click(screen.getByRole('button', { name: '記録' }))
      const panel = screen.getByRole('complementary', { name: '会話の記録' })
      expect(within(panel).getByText('1つ目の質問')).toBeInTheDocument()
      expect(within(panel).getByText('1つ目の回答')).toBeInTheDocument()
    })

    it('開いているパネルのボタンをもう一度押すと閉じる', () => {
      renderScreen()
      fireEvent.click(screen.getByRole('button', { name: '記録' }))
      fireEvent.click(screen.getByRole('button', { name: '記録' }))
      expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
    })
  })
})
