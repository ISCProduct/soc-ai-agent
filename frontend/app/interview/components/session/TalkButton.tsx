import { Button } from '@mui/material'

import { CALL } from '../../callTokens'
import { ROUND_BUTTON_SIZE, WIDE_BREAKPOINT } from './sessionLayout'
import PhaseIconView from './PhaseIconView'
import { PHASE_VIEW, type PhaseView, type TurnPhase } from './turnPhase'

const TONE_SX: Record<PhaseView['talkTone'], object> = {
  quiet: { bgcolor: CALL.control, color: CALL.muted },
  primary: { bgcolor: CALL.accent, color: '#fff', '&:hover': { bgcolor: CALL.accent, filter: 'brightness(1.1)' } },
  recording: { bgcolor: CALL.rec, color: '#fff', '&:hover': { bgcolor: CALL.rec, filter: 'brightness(1.1)' } },
}

interface Props {
  phase: TurnPhase
  disabled: boolean
  onStart: () => void
  onStop: () => void
}

/** 練習用の主ボタン。文言・アイコン・色は PHASE_VIEW の表で決まる。 */
export default function TalkButton({ phase, disabled, onStart, onStop }: Props) {
  const view = PHASE_VIEW[phase]
  return (
    <Button
      onClick={phase === 'recording' ? onStop : onStart}
      disabled={disabled}
      startIcon={<PhaseIconView icon={view.talkIcon} />}
      sx={{
        minHeight: ROUND_BUTTON_SIZE,
        minWidth: 160,
        px: 3,
        borderRadius: 999,
        fontSize: 15,
        fontWeight: 700,
        flexBasis: { xs: '100%', [WIDE_BREAKPOINT]: 'auto' },
        order: { xs: -1, [WIDE_BREAKPOINT]: 0 },
        ...TONE_SX[view.talkTone],
        // 無効でも「待機中」として読めるよう、薄くしすぎない
        '&.Mui-disabled': { ...TONE_SX[view.talkTone], opacity: view.talkTone === 'quiet' ? 0.9 : 0.6 },
      }}
    >
      {view.talkLabel}
    </Button>
  )
}
