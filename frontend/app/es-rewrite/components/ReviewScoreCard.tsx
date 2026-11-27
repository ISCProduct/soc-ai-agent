import { Box, LinearProgress, Paper, Stack, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import { formatDate } from '@/lib/i18n/format'
import { UI } from '@/lib/design/tokens'
import type { ReviewResult } from '../types'

// ScoreDelta は同一設問の前回スコアとの差（#1531）。
// 前回が無い項目は何も出さない。0 を出すと「評価された結果の±0」と
// 「比べる相手がいない」が区別できなくなる。
function ScoreDelta({ current, previous }: { current: number; previous: number | null | undefined }) {
  if (previous == null) return null
  const diff = current - previous
  if (diff === 0) {
    return <Typography sx={{ fontSize: 12, fontWeight: 600, color: UI.inkFaint }}>±0</Typography>
  }
  const up = diff > 0
  return (
    <Typography sx={{ fontSize: 12, fontWeight: 700, color: up ? '#15803d' : '#b91c1c' }}>
      {up ? '▲' : '▼'}{Math.abs(diff)}
    </Typography>
  )
}

function formatPreviousDate(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '前回'
  return formatDate(d, { year: 'numeric', month: 'numeric', day: 'numeric' })
}

type ScoreKey = 'specificity_score' | 'star_score' | 'company_fit_score' | 'length_balance_score'

const SCORE_ITEMS: { key: ScoreKey; label: string; color: string }[] = [
  { key: 'specificity_score',    label: '具体性',       color: '#3b82f6' },
  { key: 'star_score',           label: 'STAR法準拠',   color: '#8b5cf6' },
  { key: 'company_fit_score',    label: '企業適合性',   color: UI.mark },
  { key: 'length_balance_score', label: '文字数バランス', color: '#10b981' },
]

// 添削スコア（前回との差分・企業適合性を評価できなかった理由の案内を含む）
export default function ReviewScoreCard({
  reviewResult,
  reviewedCompany,
}: {
  reviewResult: ReviewResult
  reviewedCompany: string
}) {
  return (
    <Paper elevation={0} sx={{ p: 3, borderRadius: 2, border: `2px solid ${UI.mark}`, bgcolor: `color-mix(in srgb, ${UI.mark} 2%, transparent)` }}>
      <Typography sx={{ fontWeight: 700, fontSize: 17, color: UI.mark, mb: reviewResult.previous ? 1 : 2.5 }}>添削スコア</Typography>
      {reviewResult.previous && (
        <Typography sx={{ fontSize: 12, color: UI.inkFaint, mb: 2 }}>
          同じ設問の前回（{formatPreviousDate(reviewResult.previous.created_at)}）と比べています
        </Typography>
      )}
      <Stack spacing={2}>
        {SCORE_ITEMS.map(({ key, label, color }) => {
          const score = reviewResult[key]
          // 未評価（企業情報なし）の項目は行ごと出さない。空のバーは 0/10 に見え、
          // 「低く評価された」と誤解させるため(#1524)。理由はスコアの下でまとめて案内する。
          // null だけでなく undefined も弾く（値が欠けたレスポンスで NaN のバーを出さない）
          if (score == null) return null
          const pct = (score / 10) * 100
          return (
            <Box key={key}>
              <Stack direction="row" justifyContent="space-between" sx={{ mb: 0.5 }}>
                <Typography sx={{ fontSize: 13, fontWeight: 600, color: UI.inkSoft }}>{label}</Typography>
                <Stack direction="row" spacing={1} alignItems="baseline">
                  <ScoreDelta current={score} previous={reviewResult.previous?.[key] ?? null} />
                  <Typography sx={{ fontSize: 13, fontWeight: 700, color }}>{score} / 10</Typography>
                </Stack>
              </Stack>
              <LinearProgress
                variant="determinate"
                value={pct}
                sx={{ height: 8, borderRadius: 4, bgcolor: UI.ruleSoft, '& .MuiLinearProgress-bar': { bgcolor: color, borderRadius: 4 } }}
              />
            </Box>
          )
        })}
      </Stack>
      {reviewResult.company_fit_score == null && (
        !reviewedCompany ? (
          <Typography sx={{ mt: 2, fontSize: 13, color: UI.inkFaint, lineHeight: 1.8 }}>
            企業名を入力して添削すると、企業適合性も評価します。
          </Typography>
        ) : (
          // 「企業情報を取得できたか」は company_fit_score ではなく company_context_source で判断する。
          // 取得できていてもスコアだけ算出できないことがあり（企業情報を根拠にした対策アドバイスは
          // 出ている）、そこで「公開情報が見つかりませんでした」と出すと画面が矛盾する(#1524)。
          <Alert
            severity="info"
            role="status"
            sx={{ mt: 2, borderRadius: 2, fontSize: 13 }}
          >
            <Typography component="h3" sx={{ fontSize: 13, fontWeight: 700, mb: 0.5 }}>
              {(reviewResult.company_context_source ?? 'none') === 'none'
                ? '企業情報を取得できなかったため、企業適合性は評価していません'
                : '今回は企業適合性の点数を算出できませんでした'}
            </Typography>
            <Typography sx={{ fontSize: 13, lineHeight: 1.8 }}>
              {(reviewResult.company_context_source ?? 'none') === 'none'
                ? `「${reviewedCompany}」の公開情報が見つかりませんでした。根拠のない点数は出さないようにしています。上の項目とフィードバックは通常どおり評価済みです。企業名を正式名称（例: 株式会社◯◯）で入力し直すと、企業適合性も評価できる場合があります。`
                : `「${reviewedCompany}」の情報は参照できましたが、点数としてまとめられませんでした。上の項目とフィードバック、企業ごとの対策アドバイスは通常どおり利用できます。点数も知りたい場合は、もう一度「添削する」を押してください。`}
            </Typography>
          </Alert>
        )
      )}
    </Paper>
  )
}
