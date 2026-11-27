'use client'

import { forwardRef } from 'react'
import { Alert as MuiAlert, type AlertProps } from '@mui/material'

/**
 * 学生画面の Alert（#2217）。MUI の Alert は role を指定しないと、どの severity でも role="alert"（割り込み）になり、
 * 成功や情報まで強く読み上げられる。error だけを role="alert" にし、info・success・warning は role="status"（割り込まない）にする。
 * 画面ごとに role を明示しているものは、その指定を優先する。
 *
 * 注意（#2217 のレビュー）: role="status" の要素を、中身ごと後から差し込むと読まれない画面リーダーがある。
 * ボタンを押した結果として後から出す通知（成功・Snackbar など）と、失敗の通知（見た目が warning のものを含む）は、
 * 呼び出し側で role="alert" を明示する。画面を開いたときから出ている注意は status のままでよい。
 */
export const Alert = forwardRef<HTMLDivElement, AlertProps>(function Alert({ role, severity = 'success', ...props }, ref) {
  return <MuiAlert ref={ref} severity={severity} role={role ?? (severity === 'error' ? 'alert' : 'status')} {...props} />
})
