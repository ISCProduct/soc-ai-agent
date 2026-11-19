'use client'

import {
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControlLabel,
  Typography,
} from '@mui/material'

export interface ConsentDialogProps {
  open: boolean
  consentGiven: boolean
  onConsentChange: (checked: boolean) => void
  onClose: () => void
  onConfirm: () => void
}

/**
 * 録画同意ダイアログ（LOBBY / SESSION 画面で共有）
 */
export default function ConsentDialog({ open, consentGiven, onConsentChange, onClose, onConfirm }: ConsentDialogProps) {
  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>面接練習を開始する前に</DialogTitle>
      <DialogContent>
        <Typography variant="body2" sx={{ mb: 2 }}>
          面接練習では、より良いフィードバックのために以下のデータを収集します。
        </Typography>
        <Typography variant="body2" component="ul" sx={{ pl: 2, mb: 2, color: 'text.secondary' }}>
          <li>音声・動画の録画（フィードバック生成に使用）</li>
          <li>会話のテキスト（評価スコア算出に使用）</li>
          <li>音声は、文字起こしのため外部のAI事業者へ送信します。面接中にお名前や連絡先は話さないでください</li>
        </Typography>
        <Typography variant="body2" color="error.main" sx={{ mb: 2 }}>
          ※ これらのデータは企業には提供されません。サービス内でのみ使用します。
        </Typography>
        <Typography variant="body2" sx={{ mb: 2 }}>
          録画データは、アカウントが有効な間保管し、退会後約30日で削除します。
          AI の学習に使うかどうかは、プロフィールの設定で、いつでも選べます（同意しない場合は学習に使いません）。
          詳細は<Button size="small" sx={{ p: 0, minWidth: 0, textDecoration: 'underline', fontSize: 'inherit' }}
            onClick={() => window.open('/privacy', '_blank')}>プライバシーポリシー</Button>をご確認ください。
        </Typography>
        <FormControlLabel
          control={
            <Checkbox
              checked={consentGiven}
              onChange={(e) => onConsentChange(e.target.checked)}
            />
          }
          label="上記の内容に同意して面接練習を始める"
        />
      </DialogContent>
      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose}>キャンセル</Button>
        <Button
          variant="contained"
          disabled={!consentGiven}
          onClick={onConfirm}
        >
          面接に参加
        </Button>
      </DialogActions>
    </Dialog>
  )
}
