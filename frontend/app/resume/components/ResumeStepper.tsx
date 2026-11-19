'use client'

import { Box, Stack, Typography } from '@mui/material'
import CheckIcon from '@mui/icons-material/Check'
import { UI } from '@/lib/design/tokens'

export const RESUME_STEPS = ['書類を選ぶ', '応募先を決める', '結果を見る'] as const

type ResumeStepperProps = {
  /** 今のステップ（1〜3）。これより前は完了として表示する */
  current: 1 | 2 | 3
}

function Badge({ n, state }: { n: number; state: 'done' | 'current' | 'todo' }) {
  const filled = state !== 'todo'
  return (
    <Box
      aria-hidden
      sx={{
        width: 28,
        height: 28,
        flexShrink: 0,
        borderRadius: '14px',
        display: 'grid',
        placeItems: 'center',
        fontSize: 14,
        fontWeight: 700,
        bgcolor: state === 'done' ? 'success.main' : state === 'current' ? UI.mark : UI.field,
        color: filled ? 'primary.contrastText' : UI.inkSoft,
        border: filled ? 'none' : `1.5px solid ${UI.inkFaint}`,
      }}
    >
      {state === 'done' ? <CheckIcon sx={{ fontSize: 18 }} /> : n}
    </Box>
  )
}

/** 書類 → 応募先 → 結果 の流れ。PC は丸と線、スマホは3本の帯と「ステップ n/3」。 */
export function ResumeStepper({ current }: ResumeStepperProps) {
  const stateOf = (n: number) => (n < current ? 'done' : n === current ? 'current' : 'todo')
  return (
    <>
      <Stack
        component="ol"
        aria-label="進み具合"
        direction="row"
        alignItems="center"
        spacing={1.75}
        sx={{ display: { xs: 'none', md: 'flex' }, listStyle: 'none', m: 0, p: 0, pt: 1 }}
      >
        {RESUME_STEPS.map((label, i) => {
          const n = i + 1
          const state = stateOf(n)
          return (
            <Stack
              component="li"
              key={label}
              direction="row"
              alignItems="center"
              spacing={1.75}
              aria-current={state === 'current' ? 'step' : undefined}
            >
              <Stack direction="row" alignItems="center" spacing={1}>
                <Badge n={n} state={state} />
                <Typography sx={{ fontSize: 15, fontWeight: state === 'todo' ? 400 : 700, color: state === 'todo' ? UI.inkSoft : UI.ink }}>
                  {label}
                </Typography>
              </Stack>
              {n < 3 && <Box aria-hidden sx={{ width: 40, height: 2, bgcolor: UI.rule }} />}
            </Stack>
          )
        })}
      </Stack>
      <Box sx={{ display: { xs: 'block', md: 'none' }, width: '100%' }}>
        <Stack direction="row" spacing={0.75} aria-hidden>
          {RESUME_STEPS.map((label, i) => (
            <Box
              key={label}
              sx={{ flex: 1, height: 4, borderRadius: 2, bgcolor: stateOf(i + 1) === 'done' ? 'success.main' : stateOf(i + 1) === 'current' ? UI.mark : UI.rule }}
            />
          ))}
        </Stack>
        <Typography sx={{ mt: 1, fontSize: 14, color: UI.inkSoft }}>
          ステップ {current}/3 <Box component="span" sx={{ ml: 1, fontWeight: 700, color: UI.ink }}>{RESUME_STEPS[current - 1]}</Box>
        </Typography>
      </Box>
    </>
  )
}
