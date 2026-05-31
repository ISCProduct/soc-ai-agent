'use client'

import { Box, Chip, Collapse, IconButton, Paper, Stack, Typography } from '@mui/material'
import TrendingUpIcon from '@mui/icons-material/TrendingUp'
import TrendingDownIcon from '@mui/icons-material/TrendingDown'
import RemoveIcon from '@mui/icons-material/Remove'
import CloseIcon from '@mui/icons-material/Close'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import { useState } from 'react'

export type WeightScore = {
  weight_category: string
  score: number
}

export type ScoreDelta = {
  category: string
  before: number
  after: number
  delta: number
}

export function computeScoreDeltas(before: WeightScore[], after: WeightScore[]): ScoreDelta[] {
  const beforeMap = new Map(before.map(s => [s.weight_category, s.score]))
  return after
    .map(s => {
      const prev = beforeMap.get(s.weight_category) ?? 0
      return {
        category: s.weight_category,
        before: prev,
        after: s.score,
        delta: s.score - prev,
      }
    })
    .filter(d => d.delta !== 0)
    .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta))
}

type Props = {
  beforeScores: WeightScore[] | null
  afterScores: WeightScore[] | null
  title?: string
}

export default function ScoreUpdateBanner({ beforeScores, afterScores, title = 'プロフィールスコアが更新されました' }: Props) {
  const [open, setOpen] = useState(true)

  if (!afterScores || afterScores.length === 0) return null

  const deltas = beforeScores ? computeScoreDeltas(beforeScores, afterScores) : []
  const hasChanges = deltas.length > 0

  return (
    <Collapse in={open}>
      <Paper
        elevation={0}
        sx={{
          border: '1px solid',
          borderColor: 'primary.light',
          borderRadius: 2,
          bgcolor: 'primary.50',
          p: 2,
          mb: 2,
        }}
      >
        <Stack direction="row" alignItems="flex-start" spacing={1}>
          <AutoAwesomeIcon color="primary" sx={{ mt: 0.25 }} />
          <Box flex={1}>
            <Stack direction="row" alignItems="center" justifyContent="space-between">
              <Typography fontWeight={700} color="primary.main" fontSize="0.95rem">
                {title}
              </Typography>
              <IconButton size="small" onClick={() => setOpen(false)}>
                <CloseIcon fontSize="small" />
              </IconButton>
            </Stack>

            {hasChanges ? (
              <Stack direction="row" flexWrap="wrap" gap={1} mt={1}>
                {deltas.map(d => (
                  <DeltaChip key={d.category} delta={d} />
                ))}
              </Stack>
            ) : (
              <Typography variant="body2" color="text.secondary" mt={0.5}>
                今回のセッション結果がプロフィールに反映されました。
              </Typography>
            )}
          </Box>
        </Stack>
      </Paper>
    </Collapse>
  )
}

function DeltaChip({ delta }: { delta: ScoreDelta }) {
  if (delta.delta > 0) {
    return (
      <Chip
        icon={<TrendingUpIcon />}
        label={`${delta.category} +${delta.delta}`}
        size="small"
        color="success"
        variant="outlined"
      />
    )
  }
  if (delta.delta < 0) {
    return (
      <Chip
        icon={<TrendingDownIcon />}
        label={`${delta.category} ${delta.delta}`}
        size="small"
        color="error"
        variant="outlined"
      />
    )
  }
  return (
    <Chip
      icon={<RemoveIcon />}
      label={delta.category}
      size="small"
      variant="outlined"
    />
  )
}
