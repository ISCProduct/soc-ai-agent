import { Box, CircularProgress, Stack, Typography } from '@mui/material'
import CheckCircleOutlineIcon from '@mui/icons-material/CheckCircleOutline'
import RadioButtonUncheckedIcon from '@mui/icons-material/RadioButtonUnchecked'
import { REPORT_STAGE_STEPS, type ReportStage } from '../../reportProgress'
import { UI } from '@/lib/design/tokens'

type StepState = 'done' | 'active' | 'waiting'

const STATE_LABEL: Record<StepState, string> = { done: '完了', active: '進行中', waiting: '待機中' }
/** 段階アイコンの枠（px）。行ごとにアイコン幅が違っても文言の左端が揃うようにする */
const ICON_BOX = 20

function stepState(index: number, current: number, stage: ReportStage): StepState {
  if (current < 0) return stage === 'done' ? 'done' : 'waiting'
  if (index < current) return 'done'
  return index === current ? 'active' : 'waiting'
}

function StepIcon({ state }: { state: StepState }) {
  if (state === 'done') return <CheckCircleOutlineIcon fontSize="small" sx={{ color: 'success.main' }} />
  if (state === 'active') return <CircularProgress size={18} sx={{ color: UI.mark }} />
  return <RadioButtonUncheckedIcon fontSize="small" sx={{ color: UI.inkFaint }} />
}

/** 段階リスト。状態は色・アイコン・右端の文言の3つで示す */
export default function StageList({ stage }: { stage: ReportStage }) {
  const current = REPORT_STAGE_STEPS.findIndex((s) => s.stage === stage)
  return (
    <Stack component="ul" spacing={1} sx={{ listStyle: 'none', m: 0, p: 0 }}>
      {REPORT_STAGE_STEPS.map((step, i) => {
        const state = stepState(i, current, stage)
        return (
          <Box component="li" key={step.stage} sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
            <Box sx={{ width: ICON_BOX, height: ICON_BOX, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
              <StepIcon state={state} />
            </Box>
            <Typography variant="body2" sx={{ flex: 1, fontWeight: state === 'active' ? 700 : 400, color: state === 'waiting' ? UI.inkFaint : UI.ink }}>
              {step.label}
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: state === 'active' ? 700 : 400, color: state === 'active' ? UI.mark : UI.inkSoft }}>
              {STATE_LABEL[state]}
            </Typography>
          </Box>
        )
      })}
    </Stack>
  )
}
