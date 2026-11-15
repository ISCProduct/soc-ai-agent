import { Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle } from '@mui/material'

interface Props {
  open: boolean
  currentQuestionIndex: number
  /** 「面接を続ける」または背景クリック・Esc */
  onContinue: () => void
  onConfirmEnd: () => void
}

const buttonSx = { minHeight: 48, borderRadius: 999, fontWeight: 700 } as const

/**
 * 終了の確認。誤タップで面接が終わらないよう、主ボタンは安全な「面接を続ける」にする。
 * フォーカスの移動・復帰・Esc は MUI Dialog 標準に任せ、初期フォーカスは安全側に置く。
 */
export default function EndConfirmDialog({ open, currentQuestionIndex, onContinue, onConfirmEnd }: Props) {
  return (
    <Dialog open={open} onClose={onContinue} aria-labelledby="end-confirm-title" fullWidth maxWidth="xs" PaperProps={{ sx: { borderRadius: 4 } }}>
      <DialogTitle id="end-confirm-title" sx={{ fontWeight: 700 }}>面接を終了しますか？</DialogTitle>
      <DialogContent>
        <DialogContentText>
          ここまでの回答（質問{currentQuestionIndex}まで）でレポートを作ります。続けて答えると、より詳しいレポートになります。
        </DialogContentText>
      </DialogContent>
      <DialogActions sx={{ flexDirection: 'column', gap: 1, px: 3, pb: 3, '& > :not(style)': { m: 0, width: '100%' } }}>
        <Button variant="contained" autoFocus onClick={onContinue} sx={buttonSx}>面接を続ける</Button>
        <Button variant="outlined" color="inherit" onClick={onConfirmEnd} sx={buttonSx}>終了してレポートを見る</Button>
      </DialogActions>
    </Dialog>
  )
}
