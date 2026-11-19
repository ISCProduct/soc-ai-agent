'use client'

import { forwardRef } from 'react'
import { Alert as MuiAlert, type AlertProps } from '@mui/material'

/**
 * 学生画面の Alert（#2217）。MUI の Alert は role を指定しないと、どの severity でも role="alert"（割り込み）になり、
 * 成功や情報まで強く読み上げられる。error だけを role="alert" にし、info・success・warning は role="status"（割り込まない）にする。
 * 画面ごとに role を明示しているものは、その指定を優先する。
 */
export const Alert = forwardRef<HTMLDivElement, AlertProps>(function Alert({ role, severity = 'success', ...props }, ref) {
  return <MuiAlert ref={ref} severity={severity} role={role ?? (severity === 'error' ? 'alert' : 'status')} {...props} />
})
