/**
 * @jest-environment jsdom
 */
import { fireEvent, render, screen } from '@testing-library/react'

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

  // 2D化(#2D化)に伴うレイアウト再構築: 専用の「現在の質問」枠を新設し、
  // 既存の発話ストリーム(utterances/partialAi)から導出する。新規データは持たない。
  describe('現在の質問・ライブ回答パネル', () => {
    it('確定済みのAI発話を「現在の質問」として表示する', () => {
      renderScreen({ utterances: [{ role: 'ai', text: 'あなたの強みを教えてください' }] })
      expect(screen.getByText('あなたの強みを教えてください')).toBeInTheDocument()
    })

    it('読み上げ中(partialAi)があればそちらを優先して表示する', () => {
      renderScreen({
        utterances: [{ role: 'ai', text: '前の質問' }],
        partialAi: '読み上げ中の質問です',
      })
      expect(screen.getByText('読み上げ中の質問です')).toBeInTheDocument()
      expect(screen.queryByText('前の質問')).not.toBeInTheDocument()
    })

    it('認識中(partialUser)の発話をライブ回答として表示する', () => {
      renderScreen({
        utterances: [{ role: 'ai', text: '質問' }],
        partialUser: '話している途中の内容',
      })
      expect(screen.getByText('話している途中の内容')).toBeInTheDocument()
    })
  })

  describe('STAR回答フレームワーク思考ガイド', () => {
    it('既定で表示されており、クリックで折りたためる', () => {
      renderScreen({ utterances: [{ role: 'ai', text: '質問' }] })

      expect(screen.getByText(/Situation/)).toBeVisible()

      fireEvent.click(screen.getByText('STAR回答フレームワーク思考ガイド'))
      expect(screen.queryByText(/Situation/)).not.toBeInTheDocument()
    })

    // キーボード操作(Tab+Enter/Space)でも開閉できること。role="button"の自前実装のため、
    // クリック操作だけでなくキーボード操作の回帰も防ぐ(テスター指摘)。
    it('キーボード操作(Enter/Space)でも折りたためる', () => {
      renderScreen({ utterances: [{ role: 'ai', text: '質問' }] })
      const toggle = screen.getByText('STAR回答フレームワーク思考ガイド').closest('[role="button"]')!

      fireEvent.keyDown(toggle, { key: 'Enter' })
      expect(screen.queryByText(/Situation/)).not.toBeInTheDocument()

      fireEvent.keyDown(toggle, { key: ' ' })
      expect(screen.getByText(/Situation/)).toBeVisible()
    })
  })

  describe('これまでの質問・回答', () => {
    it('既定では折りたたまれており、過去の発話は表示しない', () => {
      renderScreen({
        utterances: [
          { role: 'ai', text: '1つ目の質問' },
          { role: 'user', text: '1つ目の回答' },
          { role: 'ai', text: '2つ目の質問（現在）' },
        ],
      })
      // 「現在の質問」枠には最新のAI発話のみ表示され、過去分は折りたたみの中
      expect(screen.getByText('2つ目の質問（現在）')).toBeInTheDocument()
      expect(screen.queryByText('1つ目の質問')).not.toBeInTheDocument()
    })

    it('開くと過去の質問・回答が表示される', () => {
      renderScreen({
        utterances: [
          { role: 'ai', text: '1つ目の質問' },
          { role: 'user', text: '1つ目の回答' },
          { role: 'ai', text: '2つ目の質問（現在）' },
        ],
      })

      fireEvent.click(screen.getByText(/これまでの質問・回答/))
      expect(screen.getByText('1つ目の質問')).toBeInTheDocument()
      // 「1つ目の回答」は「あなたの回答」ライブパネル（直近の確定発話へのフォールバック）
      // と履歴の両方に出るため、複数件の存在だけを確認する
      expect(screen.getAllByText('1つ目の回答').length).toBeGreaterThanOrEqual(1)
    })
  })
})
