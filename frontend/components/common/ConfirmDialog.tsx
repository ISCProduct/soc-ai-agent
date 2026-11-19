'use client'

import { useId, useRef, useState, type ReactNode } from 'react'
import {
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
} from '@mui/material'

export interface ConfirmDialogProps {
  open: boolean
  title: string
  message: ReactNode
  confirmLabel: string
  cancelLabel?: string
  /** 取り消せない操作（削除・無効化など）のとき true。確定ボタンを color="error" にする */
  destructive?: boolean
  /** 呼び出し側で処理中を持っている場合に渡す。内部の処理中と同じく、閉じない・押せない状態にする */
  busy?: boolean
  /**
   * 確定したときの処理。Promise を返すと、終わるまでダイアログを閉じず、ボタンも押せなくする。
   * ダイアログを閉じる（open を false にする）のは呼び出し側の役目。
   */
  onConfirm: () => void | Promise<void>
  /** キャンセル・Esc・背景クリック。処理中は呼ばない */
  onCancel: () => void
}

/**
 * ブラウザ標準の window.confirm の代わりに使う確認ダイアログ（SOCAIAGENT-534）。
 * 初期フォーカスは安全側のキャンセルに置き、フォーカスの移動・復帰・Esc は MUI Dialog 標準に任せる。
 */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel = 'キャンセル',
  destructive = false,
  busy = false,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const titleId = useId()
  const descriptionId = useId()
  const [pending, setPending] = useState(false)
  // 同じイベントループ内の連打でも二重に実行しないよう、state とは別に同期的に持つ
  const pendingRef = useRef(false)
  const inProgress = pending || busy

  const handleConfirm = async () => {
    if (pendingRef.current || busy) return
    pendingRef.current = true
    setPending(true)
    try {
      await onConfirm()
    } finally {
      pendingRef.current = false
      setPending(false)
    }
  }

  const handleClose = () => {
    if (inProgress) return
    onCancel()
  }

  return (
    <Dialog
      open={open}
      onClose={handleClose}
      disableEscapeKeyDown={inProgress}
      aria-labelledby={titleId}
      aria-describedby={descriptionId}
      fullWidth
      maxWidth="xs"
    >
      <DialogTitle id={titleId} sx={{ fontWeight: 700 }}>
        {title}
      </DialogTitle>
      <DialogContent>
        <DialogContentText id={descriptionId}>{message}</DialogContentText>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={handleClose} disabled={inProgress} color="inherit" autoFocus>
          {cancelLabel}
        </Button>
        <Button
          variant="contained"
          color={destructive ? 'error' : 'primary'}
          onClick={handleConfirm}
          disabled={inProgress}
          startIcon={inProgress ? <CircularProgress size={14} color="inherit" /> : null}
          disableElevation
        >
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
