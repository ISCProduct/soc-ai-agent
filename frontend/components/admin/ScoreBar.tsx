'use client'

import { Box, Stack, Tooltip, Typography } from '@mui/material'

type ScoreBarProps = {
  label: string
  /** 0〜100 のスコア */
  score: number
  /** バーの色。省略時はテーマの primary */
  color?: string
  /** 補足（ツールチップに出す） */
  hint?: string
}

// スコアの目盛り。数値だけを並べても大小が読み取れないため、
// 生徒間の比較ができるようバーの長さで示す（#1225）。
//
// 幅を割合で持たせているので、行が増えても同じ基準で並ぶ。
export function ScoreBar({ label, score, color, hint }: ScoreBarProps) {
  // スコアは 0〜100 想定。外れ値が来てもバーが枠を突き抜けないよう丸める
  const pct = Math.max(0, Math.min(100, score))
  const shown = Number.isInteger(score) ? String(score) : score.toFixed(1)

  const bar = (
    <Stack spacing={0.25} sx={{ minWidth: 150 }}>
      <Stack direction="row" justifyContent="space-between" alignItems="baseline" spacing={1}>
        <Typography variant="caption" sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>
          {label}
        </Typography>
        <Typography variant="caption" sx={{ fontVariantNumeric: 'tabular-nums', fontWeight: 600 }}>
          {shown}
        </Typography>
      </Stack>
      <Box
        role="img"
        aria-label={`${label} ${shown}点`}
        sx={{ height: 6, borderRadius: 3, bgcolor: 'action.hover', overflow: 'hidden' }}
      >
        <Box
          sx={{
            width: `${pct}%`,
            height: '100%',
            borderRadius: 3,
            bgcolor: color ?? 'primary.main',
          }}
        />
      </Box>
    </Stack>
  )

  return hint ? <Tooltip title={hint}>{bar}</Tooltip> : bar
}
