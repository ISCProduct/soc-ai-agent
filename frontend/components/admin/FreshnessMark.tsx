import { Box, Stack, Typography } from '@mui/material'
import { FRESHNESS, type Freshness } from '@/lib/design-tokens'

type FreshnessMarkProps = {
  freshness: Freshness
  /** 「あと3日」「92日経過」など、鮮度の内訳。省略時はラベルだけ出す。 */
  detail?: string
  /** 一覧の左ガターで使うとき true。字形だけを出し、ラベルは読み上げに回す。 */
  compact?: boolean
}

/**
 * データ鮮度の表示。管理画面の一覧で最も繰り返し現れる情報。
 *
 * 取得日時の TTL（info 90日 / relations 60日 / tech 30日 / jobs 7日）と欠損の有無が
 * この画面群の主要な判断材料なので、汎用のステータスチップではなく専用の表示にしている。
 *
 * 色だけで状態を伝えない（§19）。字形（● ◐ ○）と文字を必ず併記し、
 * compact のときも字形は残してラベルは読み上げへ回す。
 *
 * 一覧では行頭のガター列に置く想定。列を1つ使うが、鮮度は装飾ではなく
 * 「この行に対応が必要か」そのものなので、情報として列を与える。
 */
export function FreshnessMark({ freshness, detail, compact = false }: FreshnessMarkProps) {
  const { mark, label, color } = FRESHNESS[freshness]
  const fullLabel = detail ? `${label}（${detail}）` : label

  if (compact) {
    return (
      <Box
        component="span"
        sx={{ color, fontSize: 14, lineHeight: 1 }}
        // 字形だけでは読み上げられないため、意味はここで渡す。
        role="img"
        aria-label={fullLabel}
      >
        {/* 取得済みは字形を持たないので、列幅を保つために空白を置く */}
        {mark || '　'}
      </Box>
    )
  }

  return (
    <Stack direction="row" spacing={0.75} alignItems="baseline" sx={{ color }}>
      {mark ? (
        <Box component="span" aria-hidden sx={{ fontSize: 14, lineHeight: 1.6 }}>
          {mark}
        </Box>
      ) : null}
      <Typography variant="body2" component="span" sx={{ color: 'inherit', fontWeight: 600 }}>
        {label}
      </Typography>
      {detail ? (
        <Typography variant="body2" component="span" color="text.secondary">
          {detail}
        </Typography>
      ) : null}
    </Stack>
  )
}
