'use client'

import type { CSSProperties } from 'react'
import { Box, Typography } from '@mui/material'
import { marketColors, type MarketType } from '@/lib/company-data'

type DiagramCompanyNodeLabelProps = {
  name: string
  marketType: MarketType
  isFocus?: boolean
  truncateAt?: number
}

/** 関連図ノードの共通ラベル（色ドット + 社名） */
export function DiagramCompanyNodeLabel({
  name,
  marketType,
  isFocus = false,
  truncateAt = 22,
}: DiagramCompanyNodeLabelProps) {
  const display =
    truncateAt > 0 && name.length > truncateAt ? `${name.slice(0, truncateAt)}…` : name

  return (
    <Box
      sx={{
        display: 'flex',
        alignItems: 'center',
        gap: 1,
        px: 0.5,
        py: 0.25,
        minWidth: 0,
        maxWidth: 180,
      }}
    >
      <Box
        sx={{
          width: 8,
          height: 8,
          borderRadius: '50%',
          bgcolor: marketColors[marketType],
          flexShrink: 0,
        }}
      />
      <Typography
        component="span"
        sx={{
          fontSize: isFocus ? 13 : 12,
          fontWeight: isFocus ? 700 : 500,
          lineHeight: 1.35,
          color: '#1f2937',
          wordBreak: 'break-word',
        }}
      >
        {display}
      </Typography>
    </Box>
  )
}

export function diagramNodeStyle(opts: {
  marketType: MarketType
  isFocus?: boolean
  isDetail?: boolean
}): CSSProperties {
  const { marketType, isFocus = false, isDetail = false } = opts
  const borderColor = isFocus ? '#0f766e' : isDetail ? '#2563eb' : '#e5e7eb'
  return {
    background: isFocus ? '#ecfdf5' : isDetail ? '#eff6ff' : '#ffffff',
    border: `${isFocus || isDetail ? 2 : 1}px solid ${borderColor}`,
    borderRadius: 12,
    padding: '10px 12px',
    minWidth: isFocus ? 160 : 140,
    boxShadow: isFocus
      ? '0 8px 24px rgba(15, 118, 110, 0.12)'
      : '0 1px 3px rgba(15, 23, 42, 0.06)',
    cursor: 'pointer',
    borderLeft: `3px solid ${marketColors[marketType]}`,
  }
}
