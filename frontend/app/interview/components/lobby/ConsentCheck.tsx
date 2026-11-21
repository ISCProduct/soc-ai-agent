import { useState } from 'react'
import Link from 'next/link'
import { Box, Button, Checkbox, Collapse, FormControlLabel, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { MIN_TAP_SIZE, textLinkSx } from './lobbyStyles'

interface ConsentCheckProps {
  checked: boolean
  onChange: (checked: boolean) => void
}

const DETAILS_ID = 'lobby-consent-details'

/**
 * 録画同意のチェックボックス。同意しないと「面接を始める」は押せない（親が制御）。
 *
 * 「詳しく見る」は ConsentDialog を開かず、その場で折りたたみを開く。
 * ダイアログ側にも同意チェックと「面接に参加」ボタンがあり、ロビーに置くと
 * 同意の入口と開始ボタンが二重になって迷うため。説明文は ConsentDialog と同じ内容にしている。
 */
export default function ConsentCheck({ checked, onChange }: ConsentCheckProps) {
  const [open, setOpen] = useState(false)
  return (
    <Box>
      <FormControlLabel
        sx={{ alignItems: 'flex-start', m: 0 }}
        control={
          <Checkbox
            checked={checked}
            onChange={(e) => onChange(e.target.checked)}
            sx={{ p: 1.25, color: UI.inkSoft, '&.Mui-checked': { color: UI.mark } }}
          />
        }
        label={
          <Box sx={{ pt: 1.25 }}>
            <Typography sx={{ fontSize: 14, fontWeight: 600, color: UI.ink }}>
              録画と会話の記録を、フィードバックのためだけに使うことに同意します
            </Typography>
            <Typography sx={{ fontSize: 12, color: UI.inkSoft, mt: 0.5 }}>
              企業には渡りません。録画は有効な間は保管し、退会後12ヶ月で削除します。AI の学習への利用は別の設定で、同意した場合のみです。
            </Typography>
          </Box>
        }
      />
      <Button
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls={DETAILS_ID}
        sx={{ ...textLinkSx, ml: 5, minHeight: MIN_TAP_SIZE }}
      >
        {open ? '閉じる' : '詳しく見る'}
      </Button>
      <Collapse in={open} id={DETAILS_ID}>
        <Typography component="ul" sx={{ pl: 7, m: 0, fontSize: 12, color: UI.inkSoft, lineHeight: 1.8 }}>
          <li>音声・動画の録画（フィードバック生成に使用）</li>
          <li>会話のテキスト（評価スコア算出に使用）</li>
          <li>音声は、文字起こしのため外部のAI事業者へ送信します。面接中にお名前や連絡先は話さないでください</li>
          <li>
            詳細は<Link href="/privacy" target="_blank" style={{ color: UI.mark }}>プライバシーポリシー</Link>をご確認ください
          </li>
        </Typography>
      </Collapse>
    </Box>
  )
}
