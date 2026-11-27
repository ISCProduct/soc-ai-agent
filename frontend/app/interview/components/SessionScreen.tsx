'use client'

import { useEffect, useState, type ReactNode, type RefCallback, type RefObject } from 'react'
import { Box } from '@mui/material'

import { CALL } from '../callTokens'
import { setInterviewSessionActive } from '../sessionChrome'
import type { Utterance } from '../types'
import ControlBar, { type SidePanelKind } from './session/ControlBar'
import EndConfirmDialog from './session/EndConfirmDialog'
import InterviewerTile from './session/InterviewerTile'
import NoticeBanners from './session/NoticeBanners'
import QuestionCaption from './session/QuestionCaption'
import SelfView from './session/SelfView'
import SidePanel from './session/SidePanel'
import StatusOverlay from './session/StatusOverlay'
import TextAnswerBar from './session/TextAnswerBar'
import { deriveTranscriptView, sessionWarningText, urgencyOf } from './session/sessionDerive'
import { talkDisabled, turnPhaseOf } from './session/turnPhase'

export interface SessionScreenProps {
  status: string
  companyName: string
  userName: string
  isActive: boolean
  isConnected: boolean
  remainingSeconds: number
  sessionWarningShown: boolean
  currentQuestionIndex: number
  totalQuestionCount: number
  questionProgress: number
  questionRemainingSeconds: number
  questionRemainingLabel: string
  isDeepeningQuestion: boolean
  questionCategory: string | null
  avatarGender: 'male' | 'female'
  aiLevel: number
  aiSpeaking: boolean
  /** 学生の発話の区切りの回数。変わるたびに面接官が相槌を打つ。 */
  studentPauseCount?: number
  cameraEnabled: boolean
  captionsVisible: boolean
  handsFreeMode: boolean
  utterances: Utterance[]
  partialAi: string
  partialUser: string
  isRecording: boolean
  turnPending: boolean
  errorMessage: string | null
  /** 発話保存が再試行しても失敗したか。true なら「記録できていない」ことを画面に出す（#1476） */
  utteranceSaveFailed?: boolean
  /** 音声縮退・テキスト継続の案内（#1831） */
  voiceDegradeText?: string | null
  textContinueMode?: boolean
  /** video 要素がマウントした瞬間にストリームをアタッチするための callback ref（page.tsx 側で理由を解説） */
  sessionVideoCallbackRef: RefCallback<HTMLVideoElement>
  transcriptEndRef: RefObject<HTMLDivElement | null>
  aiAudioRef: RefObject<HTMLAudioElement | null>
  /** 親コンポーネントが保持する ConsentDialog インスタンス（LOBBY / SESSION 画面で共有） */
  consentDialog: ReactNode
  onToggleCamera: () => void
  onToggleCaptions: () => void
  onToggleHandsFree: () => void
  onStartRecording: () => void
  onStopRecording: () => void
  onSubmitTextAnswer?: (text: string) => void
  onJoin: () => void
  onStop: (forced: boolean) => void
}

/**
 * 面接中（通話画面）。部品を並べて状態を渡すだけの presentational component。
 * VAD・MediaRecorder・AudioContext・タイマー等の状態管理は親（page-content）が行い、
 * 状態から表示への変換は session/ 配下の純関数（sessionDerive / turnPhase）にある。
 */
export default function SessionScreen(props: SessionScreenProps) {
  const { status, companyName, isConnected, remainingSeconds, currentQuestionIndex } = props

  // 面接セッション中は学生下部ナビを隠す（#1569）。SelectionScreen/LobbyScreen は
  // 同じ /interview パス上の別 status なので、ここでは「このコンポーネントが
  // マウントされている間」を「セッション中」として扱う。
  useEffect(() => {
    setInterviewSessionActive(true)
    return () => setInterviewSessionActive(false)
  }, [])

  const [openPanel, setOpenPanel] = useState<SidePanelKind | null>(null)
  const [endConfirmOpen, setEndConfirmOpen] = useState(false)

  const { currentQuestionText } = deriveTranscriptView(props.utterances, props.partialAi, props.partialUser)
  const phase = turnPhaseOf(props)
  const chipLabel = props.isDeepeningQuestion ? '深掘り質問' : props.questionCategory

  return (
    <Box sx={{ height: '100vh', '@supports (height: 100dvh)': { height: '100dvh' }, width: '100%', display: 'flex', flexDirection: 'column', overflow: 'hidden', bgcolor: CALL.stage, color: CALL.ink }}>
      <NoticeBanners
        warningText={isConnected ? sessionWarningText(props.sessionWarningShown, remainingSeconds) : null}
        sessionEnded={remainingSeconds <= 0}
        utteranceSaveFailed={!!props.utteranceSaveFailed}
        voiceDegradeText={props.voiceDegradeText}
      />

      <Box sx={{ position: 'relative', flex: 1, minHeight: 0, display: 'flex', gap: 1.5, p: 1.5 }}>
        <InterviewerTile
          nameTag={`${companyName || 'AI面接練習'} 面接官`}
          avatarGender={props.avatarGender}
          aiLevel={props.aiLevel}
          aiSpeaking={props.aiSpeaking}
          isConnected={isConnected}
          turnPending={props.turnPending}
          studentPauseCount={props.studentPauseCount ?? 0}
          recording={props.isRecording}
          chipLabel={chipLabel}
          isDeepening={props.isDeepeningQuestion}
        >
          <SelfView userName={props.userName} cameraEnabled={props.cameraEnabled} videoRef={props.sessionVideoCallbackRef} />
          <QuestionCaption
            phase={phase}
            captionsVisible={props.captionsVisible}
            questionText={currentQuestionText}
            questionProgress={props.questionProgress}
            questionRemainingLabel={props.questionRemainingLabel}
            urgency={urgencyOf(props.questionRemainingSeconds)}
            anchorRef={props.transcriptEndRef}
          />
          <StatusOverlay status={status} errorMessage={props.errorMessage} onReconnect={props.onJoin} />
        </InterviewerTile>
        {openPanel && <SidePanel kind={openPanel} utterances={props.utterances} onClose={() => setOpenPanel(null)} />}
      </Box>

      <ControlBar
        currentQuestionIndex={currentQuestionIndex}
        totalQuestionCount={props.totalQuestionCount}
        remainingSeconds={remainingSeconds}
        phase={phase}
        talkDisabled={talkDisabled({ ...props, isConnected })}
        isConnected={isConnected}
        cameraEnabled={props.cameraEnabled}
        captionsVisible={props.captionsVisible}
        handsFreeMode={props.handsFreeMode}
        openPanel={openPanel}
        onToggleCamera={props.onToggleCamera}
        onToggleCaptions={props.onToggleCaptions}
        onToggleHandsFree={props.onToggleHandsFree}
        onTogglePanel={(kind) => setOpenPanel((cur) => (cur === kind ? null : kind))}
        onStartRecording={props.onStartRecording}
        onStopRecording={props.onStopRecording}
        onRequestEnd={() => setEndConfirmOpen(true)}
      />

      {props.textContinueMode && props.onSubmitTextAnswer && (
        <TextAnswerBar
          disabled={!isConnected || props.turnPending || props.isRecording}
          onSubmit={props.onSubmitTextAnswer}
        />
      )}

      <EndConfirmDialog
        open={endConfirmOpen}
        currentQuestionIndex={currentQuestionIndex}
        onContinue={() => setEndConfirmOpen(false)}
        onConfirmEnd={() => {
          setEndConfirmOpen(false)
          props.onStop(false)
        }}
      />
      <audio ref={props.aiAudioRef} />
      {props.consentDialog}
    </Box>
  )
}
