'use client'

import { Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography } from '@mui/material'

type DeleteAccountDialogProps = {
  open: boolean
  /** 退会の処理中。処理中は Esc・幕のタップ・キャンセルで閉じない。 */
  deleting: boolean
  onClose: () => void
  onConfirm: () => void
}

/** アカウントの退会の確認ダイアログ（#2212） */
export function DeleteAccountDialog({ open, deleting, onClose, onConfirm }: DeleteAccountDialogProps) {
  // 処理の最中に閉じると、結果（失敗の文言・ログイン画面への移動）を待たずに画面へ戻ってしまう。
  // 処理が終わる（成功・失敗）と、呼び出し側が閉じる。
  const handleClose = () => {
    if (deleting) return
    onClose()
  }
  return (
    <Dialog open={open} onClose={handleClose} maxWidth="xs" fullWidth>
      <DialogTitle>アカウントを退会しますか？</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary">
          退会後すぐにログインできなくなり、サービス上でのデータ閲覧もできなくなります。
          保管データ（チャット・面接録画・職務経歴書など）は約30日後に完全削除されます。この操作は取り消せません。
        </Typography>
        <Typography variant="body2" component="ul" sx={{ pl: 2, mt: 1, color: 'text.secondary' }}>
          <li>退会直後: ログイン不可・データ閲覧不可</li>
          <li>約30日後: チャット・スコア・マッチング結果を完全削除</li>
          <li>約30日後: 面接練習の録画・レポートを完全削除</li>
          <li>約30日後: 職務経歴書・アカウント情報を完全削除</li>
        </Typography>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={handleClose} disabled={deleting}>
          キャンセル
        </Button>
        <Button onClick={onConfirm} color="error" variant="contained" disabled={deleting}>
          {deleting ? '退会処理中...' : '退会する'}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
