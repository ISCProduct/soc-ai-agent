import type { ReactNode } from 'react'
import { Box, IconButton, Typography } from '@mui/material'

import { CALL } from '../../callTokens'
import { ROUND_BUTTON_SIZE } from './sessionLayout'

interface Props {
  /** 下に出す短い文言。アクセシブルネームも同じにする（見える名前と読み上げを一致させる） */
  label: string
  icon: ReactNode
  onClick: () => void
  disabled?: boolean
  /** トグルの状態。指定したときだけ aria-pressed を付ける */
  pressed?: boolean
  /** danger は「終了」用の横長の赤いボタン */
  tone?: 'default' | 'danger'
}

/** Meet 風の「丸ボタン＋下に小さなラベル」。 */
export default function RoundButton({ label, icon, onClick, disabled, pressed, tone = 'default' }: Props) {
  const danger = tone === 'danger'
  const buttonWidth = danger ? { xs: 56, md: 64 } : ROUND_BUTTON_SIZE
  return (
    // 項目の幅はボタンの幅に固定する。ラベル（「ハンズフリー」など）が幅を広げると、
    // スマホで丸ボタンが1行に収まらず折り返すため。ラベルは中央のまま左右にはみ出す
    <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 0.5, width: buttonWidth }}>
      <IconButton
        onClick={onClick}
        disabled={disabled}
        aria-label={label}
        aria-pressed={pressed}
        sx={{
          // #1481: タップ領域は44px以上にする
          height: ROUND_BUTTON_SIZE,
          width: buttonWidth,
          borderRadius: 999,
          color: CALL.ink,
          bgcolor: danger ? CALL.danger : pressed ? CALL.line : CALL.control,
          '&:hover': { bgcolor: danger ? CALL.danger : CALL.line },
          '&.Mui-disabled': { color: CALL.muted, bgcolor: CALL.panel },
        }}
      >
        {icon}
      </IconButton>
      <Typography component="span" sx={{ fontSize: 11, color: CALL.muted, whiteSpace: 'nowrap' }} aria-hidden>
        {label}
      </Typography>
    </Box>
  )
}
