'use client'

import type { RefObject } from 'react'
import { Box } from '@mui/material'
import { UI } from '@/lib/design/tokens'

import type { InterviewCompany } from '../types'
import CameraPreview from './lobby/CameraPreview'
import DeviceSettings from './lobby/DeviceSettings'
import type { FocusGoal } from './lobby/FocusGoalCard'
import JoinPanel from './lobby/JoinPanel'
import LobbyFooter from './lobby/LobbyFooter'
import LobbyHeader from './lobby/LobbyHeader'
import { LOBBY_MAX_WIDTH } from './lobby/lobbyStyles'

export interface LobbyScreenProps {
  userName: string
  companyName: string
  /** 職種名。空なら企業名のみ表示する */
  positionTitle: string
  totalQuestionCount: number
  interviewCompany: InterviewCompany | null
  /** マッチング結果画面から遷移してきた場合に表示するバナー用フラグ */
  fromMatchingResults: boolean
  /** 前回レポートの課題。取得経路が未実装のため未指定なら「今回の目標」は出さない */
  focusGoal?: FocusGoal | null
  lobbyPermissionError: string | null
  onRetryPermissions: () => void
  micEnabled: boolean
  cameraEnabled: boolean
  onToggleMic: () => void
  onToggleCamera: () => void
  lobbyVideoRef: RefObject<HTMLVideoElement | null>
  /** プレビューに繋いだストリーム。機器一覧と音量メーターに使う */
  lobbyStream: MediaStream | null
  micId: string
  cameraId: string
  speakerId: string
  onSelectMic: (id: string) => void
  onSelectCamera: (id: string) => void
  onSelectSpeaker: (id: string) => void
  onBack: () => void
  /** 録画同意（画面内のチェックボックス）。false の間は開始できない */
  consentGiven: boolean
  onConsentChange: (checked: boolean) => void
  onJoin: () => void
}

/**
 * LOBBY SCREEN
 * カメラ・マイクの確認と面接参加操作を行う画面。
 * ストリームの取得・所有は親コンポーネント（page.tsx）が行い、
 * このコンポーネントは部品を並べるだけの presentational component。
 */
export default function LobbyScreen(props: LobbyScreenProps) {
  return (
    <Box sx={{ minHeight: '100vh', bgcolor: UI.paper, display: 'flex', flexDirection: 'column' }}>
      <LobbyHeader onBack={props.onBack} />
      <Box
        sx={{
          flex: 1,
          width: '100%',
          maxWidth: LOBBY_MAX_WIDTH,
          mx: 'auto',
          p: { xs: 2, md: 6 },
          display: 'flex',
          flexDirection: { xs: 'column', md: 'row' },
          alignItems: { md: 'center' },
          justifyContent: 'center',
          gap: { xs: 3, md: 6 },
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0, width: '100%' }}>
          <CameraPreview
            userName={props.userName}
            videoRef={props.lobbyVideoRef}
            permissionError={props.lobbyPermissionError}
            cameraEnabled={props.cameraEnabled}
            micEnabled={props.micEnabled}
            onRetryPermissions={props.onRetryPermissions}
            onToggleCamera={props.onToggleCamera}
            onToggleMic={props.onToggleMic}
          />
          <DeviceSettings
            stream={props.lobbyStream}
            micId={props.micId}
            cameraId={props.cameraId}
            speakerId={props.speakerId}
            onSelectMic={props.onSelectMic}
            onSelectCamera={props.onSelectCamera}
            onSelectSpeaker={props.onSelectSpeaker}
          />
        </Box>
        <JoinPanel
          companyName={props.companyName}
          positionTitle={props.positionTitle}
          totalQuestionCount={props.totalQuestionCount}
          interviewCompany={props.interviewCompany}
          fromMatchingResults={props.fromMatchingResults}
          focusGoal={props.focusGoal}
          consentGiven={props.consentGiven}
          onConsentChange={props.onConsentChange}
          onJoin={props.onJoin}
          onBack={props.onBack}
        />
      </Box>
      <LobbyFooter />
    </Box>
  )
}
