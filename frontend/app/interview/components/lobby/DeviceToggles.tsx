import { Button, Stack } from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import MicOffIcon from '@mui/icons-material/MicOff'
import VideocamIcon from '@mui/icons-material/Videocam'
import VideocamOffIcon from '@mui/icons-material/VideocamOff'
import type { ReactNode } from 'react'
import { UI } from '@/lib/design/tokens'
import { MIN_TAP_SIZE } from './lobbyStyles'

interface PillProps {
  label: string
  enabled: boolean
  onIcon: ReactNode
  offIcon: ReactNode
  onClick: () => void
}

/**
 * ピル形のオン／オフ切替。
 * 状態は色だけで伝えない。文言（オン/オフ）とアイコンの形も切り替え、aria-pressed でも示す。
 * オフは配色を反転させ、暗い枠の上でも一目で区別できるようにする。
 */
function TogglePill({ label, enabled, onIcon, offIcon, onClick }: PillProps) {
  return (
    <Button
      onClick={onClick}
      aria-pressed={enabled}
      aria-label={`${label}をオン/オフする（現在${enabled ? 'オン' : 'オフ'}）`}
      startIcon={enabled ? onIcon : offIcon}
      sx={{
        minHeight: MIN_TAP_SIZE,
        px: 2.5,
        borderRadius: 9999,
        textTransform: 'none',
        fontWeight: 600,
        bgcolor: enabled ? UI.nightPanel : UI.nightInk,
        color: enabled ? UI.nightInk : UI.nightBg,
        '&:hover': { bgcolor: enabled ? UI.nightBg : UI.nightSoft },
      }}
    >
      {label} {enabled ? 'オン' : 'オフ'}
    </Button>
  )
}

interface DeviceTogglesProps {
  cameraEnabled: boolean
  micEnabled: boolean
  onToggleCamera: () => void
  onToggleMic: () => void
}

/** プレビュー下中央のカメラ／マイク切替。 */
export default function DeviceToggles({ cameraEnabled, micEnabled, onToggleCamera, onToggleMic }: DeviceTogglesProps) {
  return (
    <Stack direction="row" spacing={1.5} sx={{ position: 'absolute', bottom: 16, left: 0, right: 0, justifyContent: 'center' }}>
      <TogglePill label="カメラ" enabled={cameraEnabled} onIcon={<VideocamIcon />} offIcon={<VideocamOffIcon />} onClick={onToggleCamera} />
      <TogglePill label="マイク" enabled={micEnabled} onIcon={<MicIcon />} offIcon={<MicOffIcon />} onClick={onToggleMic} />
    </Stack>
  )
}
