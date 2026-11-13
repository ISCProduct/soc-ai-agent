'use client'

import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from '@mui/material'
import { LOW_MATCH_THRESHOLD } from '@/lib/low-match'

type LowMatchConfirmDialogProps = {
  open: boolean
  companyName: string
  matchScore: number
  onCancel: () => void
  onConfirm: () => void
}

// マッチ度が低い企業への応募前に一度だけ確認する（#1028）。
//
// 応募を止めるものではないので、既定のフォーカスは「このまま応募する」に置かない。
// 学生が考え直す余地を作るのが目的で、誤操作を責める文面にはしない。
export function LowMatchConfirmDialog({
  open,
  companyName,
  matchScore,
  onCancel,
  onConfirm,
}: LowMatchConfirmDialogProps) {
  return (
    <Dialog open={open} onClose={onCancel} maxWidth="xs" fullWidth>
      <DialogTitle>この企業に応募しますか？</DialogTitle>
      <DialogContent>
        <Stack spacing={1.5}>
          <Typography variant="body2">
            <strong>{companyName}</strong> とのマッチ度は{' '}
            <strong>{Math.round(matchScore)}点</strong>で、
            目安の{LOW_MATCH_THRESHOLD}点を下回っています。
          </Typography>
          <Typography variant="body2" color="text.secondary">
            マッチ度はこれまでの診断結果から算出した参考値です。低いからといって
            応募できないわけではありません。志望理由がはっきりしているなら、
            そのまま進んで問題ありません。
          </Typography>
          <Typography variant="body2" color="text.secondary">
            迷う場合は、他の企業も見てから決めるか、先生に相談してみてください。
          </Typography>
        </Stack>
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onCancel}>ほかの企業も見る</Button>
        <Button onClick={onConfirm} variant="contained">
          このまま応募する
        </Button>
      </DialogActions>
    </Dialog>
  )
}
