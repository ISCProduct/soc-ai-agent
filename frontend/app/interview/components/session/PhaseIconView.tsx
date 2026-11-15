import { CircularProgress } from '@mui/material'
import MicIcon from '@mui/icons-material/Mic'
import StopIcon from '@mui/icons-material/Stop'
import VolumeUpIcon from '@mui/icons-material/VolumeUp'

import type { PhaseIcon } from './turnPhase'

/** 状態は色だけでなくアイコンでも示す。TalkButton と QuestionCaption で共有。 */
export default function PhaseIconView({ icon, size = 20 }: { icon: PhaseIcon; size?: number }) {
  switch (icon) {
    case 'speaker': return <VolumeUpIcon sx={{ fontSize: size }} />
    case 'mic': return <MicIcon sx={{ fontSize: size }} />
    case 'stop': return <StopIcon sx={{ fontSize: size }} />
    case 'spinner': return <CircularProgress size={size - 4} color="inherit" aria-hidden />
  }
}
