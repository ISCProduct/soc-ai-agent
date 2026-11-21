'use client'

import type { ReactNode } from 'react'
import { Alert, Box, Button, Typography } from '@mui/material'
import { ArrowBack } from '@mui/icons-material'
import { GUEST_REGISTER_CTA_LABEL, GUEST_REGISTER_PATH } from '@/lib/guest-limits'
import { CONTENT_MAX_WIDTH, UI } from '@/lib/design/tokens'

interface Props {
  companyCount: number
  /** 「お気に入りだけ」を表示中か（#2301） */
  favoritesOnly: boolean
  isProvisional: boolean
  diagnosisSummary: string | null
  band: { min: number; max: number }
  isGuestUser: boolean
  /** PC のみ右上に出すメール送信ボタン */
  emailAction: ReactNode
  onBack: () => void
  onNavigate: (path: string) => void
}

/** 適合度の幅がこの値以内なら「差はわずか」と一度だけ述べる */
const NARROW_BAND_SPAN = 15

export function ResultsHeader({ companyCount, favoritesOnly, isProvisional, diagnosisSummary, band, isGuestUser, emailAction, onBack, onNavigate }: Props) {
  return (
    <Box component="header" sx={{ px: { xs: 2, md: 4 }, pt: { xs: 1.5, md: 3.5 }, pb: { xs: 2.5, md: 3.25 }, backgroundColor: UI.field, borderBottom: `1px solid ${UI.rule}`, flexShrink: 0 }}>
      <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto' }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: { xs: 1, md: 2 } }}>
          <Button startIcon={<ArrowBack />} onClick={onBack} sx={{ color: UI.inkSoft, ml: -1, minHeight: 44 }}>
            チャットに戻る
          </Button>
          {emailAction}
        </Box>
        {isGuestUser && (
          <Alert severity="info" sx={{ mb: 2 }}
            action={<Button color="inherit" size="small" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>{GUEST_REGISTER_CTA_LABEL}</Button>}>
            ゲスト利用中です。メール送信や選考管理はアカウント登録後に利用できます。
            今の診断結果はそのまま引き継がれます。
          </Alert>
        )}
        <Typography component="h1" sx={{ fontSize: { xs: 24, md: 28 }, fontWeight: 700, color: UI.ink, lineHeight: 1.35, mb: 1.5 }}>
          {favoritesOnly ? `お気に入りにした${companyCount}社` : `あなたに近い${companyCount}社`}
        </Typography>
        <Box sx={{ display: 'grid', gap: 0.5, maxWidth: { md: 780 } }}>
          <Typography sx={{ fontSize: { xs: 15, md: 16 }, color: UI.inkSoft, lineHeight: { xs: 1.85, md: '28px' } }}>
            {favoritesOnly
              ? '今までの診断でお気に入りにした企業です。同じ企業は1社にまとめています。適合度は、いちばん新しい診断のものです。'
              : '回答の傾向と、企業が重視する点を突き合わせた順に並べています。順位はAIが回答から推定したもので、企業の良し悪しを決めるものではありません。'}
          </Typography>
          {companyCount > 1 && band.max - band.min <= NARROW_BAND_SPAN && (
            <Typography sx={{ fontSize: { xs: 15, md: 16 }, color: UI.inkSoft, lineHeight: { xs: 1.85, md: '28px' } }}>
              {band.min === band.max
                ? `適合度はどれも${band.min}で、差はありません。上から順に決める必要はありません。`
                : `適合度は${band.min}〜${band.max}に収まっていて差はわずかです。上から順に決める必要はありません。`}
            </Typography>
          )}
        </Box>
        {isProvisional && (
          <Alert severity="warning" icon={false}
            sx={{ mt: 1.5, maxWidth: '68ch', backgroundColor: 'transparent', border: `1px solid ${UI.flag}`, borderRadius: 0.5, color: UI.ink, py: 1.25 }}>
            {diagnosisSummary || '回答の根拠がまだ薄いため、適合度は参考値です。選択肢に理由を添えると精度が上がります。'}
          </Alert>
        )}
      </Box>
    </Box>
  )
}
