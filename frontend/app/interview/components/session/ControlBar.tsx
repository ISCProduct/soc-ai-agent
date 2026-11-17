import { Box, Typography } from '@mui/material'
import ChatBubbleOutlineIcon from '@mui/icons-material/ChatBubbleOutline'
import ScheduleIcon from '@mui/icons-material/Schedule'
import LightbulbOutlinedIcon from '@mui/icons-material/LightbulbOutlined'
import LogoutIcon from '@mui/icons-material/Logout'
import MicIcon from '@mui/icons-material/Mic'
import MicOffIcon from '@mui/icons-material/MicOff'
import SubtitlesIcon from '@mui/icons-material/Subtitles'
import SubtitlesOffIcon from '@mui/icons-material/SubtitlesOff'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'

import { formatSeconds } from '@/lib/interview/utils'
import { CALL } from '../../callTokens'
import ProgressDots from './ProgressDots'
import RoundButton from './RoundButton'
import TalkButton from './TalkButton'
import { WIDE_BREAKPOINT } from './sessionLayout'
import type { TurnPhase } from './turnPhase'

export type SidePanelKind = 'hint' | 'log'

interface Props {
  currentQuestionIndex: number
  totalQuestionCount: number
  remainingSeconds: number
  phase: TurnPhase
  talkDisabled: boolean
  isConnected: boolean
  cameraEnabled: boolean
  captionsVisible: boolean
  handsFreeMode: boolean
  openPanel: SidePanelKind | null
  onToggleCamera: () => void
  onToggleCaptions: () => void
  onToggleHandsFree: () => void
  onTogglePanel: (kind: SidePanelKind) => void
  onStartRecording: () => void
  onStopRecording: () => void
  onRequestEnd: () => void
}

/**
 * 広い画面ではグループごとに左・中央・右へ並べる。
 * スマホでは両グループを `contents` にして子を外側の行へ流し込み、
 * 主ボタンを最上段（全幅）、丸ボタンを残りの1行にそろえる（Figma のスマホ版と同じ並び）。
 */
const groupSx = {
  display: { xs: 'contents', [WIDE_BREAKPOINT]: 'flex' },
  alignItems: 'flex-start',
  justifyContent: 'center',
  gap: 1.5,
} as const

/** 下部コントロールバー: 左=進行、中央=カメラ等と主ボタン、右=ヒント・記録・終了。 */
export default function ControlBar(p: Props) {
  return (
    <Box
      component="footer"
      sx={{
        flexShrink: 0,
        display: { xs: 'flex', [WIDE_BREAKPOINT]: 'grid' },
        flexWrap: 'wrap',
        justifyContent: 'space-between',
        gridTemplateColumns: { [WIDE_BREAKPOINT]: '1fr auto 1fr' },
        gap: { xs: 1, [WIDE_BREAKPOINT]: 1.5 },
        alignItems: 'center',
        px: 2,
        py: 1.5,
        minHeight: { [WIDE_BREAKPOINT]: 96 },
      }}
    >
      {/* スマホでは左の進行表示を省き、主ボタンと操作を優先する */}
      <Box sx={{ display: { xs: 'none', [WIDE_BREAKPOINT]: 'flex' }, flexDirection: 'column', gap: 0.75 }}>
        <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1.5, color: CALL.ink }}>
          <Typography component="span" sx={{ fontSize: 15, fontWeight: 700 }}>
            質問 {p.currentQuestionIndex} / {p.totalQuestionCount}
          </Typography>
          <Typography component="span" sx={{ display: 'inline-flex', alignItems: 'center', gap: 0.5, fontSize: 12, color: CALL.muted }}>
            <ScheduleIcon sx={{ fontSize: 14 }} aria-hidden />
            残り {formatSeconds(p.remainingSeconds)}
          </Typography>
        </Box>
        <ProgressDots current={p.currentQuestionIndex} total={p.totalQuestionCount} />
      </Box>

      <Box sx={groupSx}>
        <RoundButton
          label="カメラ"
          pressed={p.cameraEnabled}
          disabled={!p.isConnected}
          onClick={p.onToggleCamera}
          icon={p.cameraEnabled ? <VideocamIcon /> : <VideocamOffIcon />}
        />
        <RoundButton
          label="字幕"
          pressed={p.captionsVisible}
          onClick={p.onToggleCaptions}
          icon={p.captionsVisible ? <SubtitlesIcon /> : <SubtitlesOffIcon />}
        />
        <RoundButton
          label="ハンズフリー"
          pressed={p.handsFreeMode}
          disabled={!p.isConnected}
          onClick={p.onToggleHandsFree}
          icon={p.handsFreeMode ? <MicIcon /> : <MicOffIcon />}
        />
        <TalkButton phase={p.phase} disabled={p.talkDisabled} onStart={p.onStartRecording} onStop={p.onStopRecording} />
      </Box>

      <Box sx={{ ...groupSx, justifyContent: 'flex-end' }}>
        <RoundButton label="ヒント" pressed={p.openPanel === 'hint'} onClick={() => p.onTogglePanel('hint')} icon={<LightbulbOutlinedIcon />} />
        <RoundButton label="記録" pressed={p.openPanel === 'log'} onClick={() => p.onTogglePanel('log')} icon={<ChatBubbleOutlineIcon />} />
        <Box aria-hidden sx={{ width: '1px', alignSelf: 'stretch', bgcolor: CALL.line, mx: 0.5, display: { xs: 'none', [WIDE_BREAKPOINT]: 'block' } }} />
        <RoundButton label="終了" tone="danger" onClick={p.onRequestEnd} icon={<LogoutIcon />} />
      </Box>
    </Box>
  )
}
