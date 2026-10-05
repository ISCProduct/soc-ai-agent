import { Box } from '@mui/material'
import { LP, RISE, NO_MOTION, delay } from './tokens'

/**
 * ファーストビューのプロダクト図（#1653）。
 *
 * ブラウザのクローム（信号機の3点＋URLバー）は付けない。
 * SaaS の LP で最も使い回されている形で、製品の説明を足さない。
 *
 * 代わりに実画面の断片をそのまま置く。対話 → スコア → 企業の適合、の順に
 * 紙片を重ね、右へ送るほど手前に出す。紙を並べた机の上の見立てで、
 * ES の原稿用紙を地紋にしたページの作りと揃える。
 *
 * 画像ファイルを置かずCSSだけで組む。Backend 停止中でも出す必要があるため、
 * 外部アセットに依存させない。
 * スコアの観点は interview_rubric.go の rubricCriteria と揃えること。
 */

const SCORES = [
  { label: '論理性', v: 82 },
  { label: '具体性', v: 64 },
  { label: '主体性', v: 91 },
] as const

/** 紙片。白をわずかに起こして、紙の上に紙を重ねた段差を出す。 */
const SHEET = {
  bgcolor: LP.card,
  border: `1px solid ${LP.rule}`,
  p: { xs: 2, md: 2.5 },
  ...NO_MOTION,
} as const

const TAG = {
  fontSize: 10,
  fontWeight: 700,
  letterSpacing: '.18em',
  color: LP.muted,
  mb: 1.5,
} as const

export function LandingVisual() {
  return (
    <Box
      aria-hidden
      sx={{
        position: 'relative',
        display: 'grid',
        gap: { xs: 2, md: 2.5 },
        pl: { md: 3 },
      }}
    >
      {/* 1. 対話 */}
      <Box
        sx={{
          ...SHEET,
          mr: { md: 6 },
          boxShadow: '2px 3px 0 rgba(28,26,23,.06)',
          ...RISE,
          ...delay(3),
        }}
      >
        <Box sx={TAG}>対話</Box>
        <Box
          sx={{
            fontSize: 13,
            lineHeight: 1.85,
            color: LP.ink,
            borderLeft: `2px solid ${LP.rule}`,
            pl: 1.5,
          }}
        >
          チームで一番こだわったのは、どこですか？
        </Box>
        <Box
          sx={{
            mt: 1.5,
            fontSize: 13,
            lineHeight: 1.85,
            color: LP.ink,
            borderLeft: `2px solid ${LP.primary}`,
            pl: 1.5,
          }}
        >
          APIの応答が遅くて、原因を切り分けて…
        </Box>
      </Box>

      {/* 2. スコア */}
      <Box
        sx={{
          ...SHEET,
          ml: { md: 5 },
          boxShadow: '2px 3px 0 rgba(28,26,23,.08)',
          ...RISE,
          ...delay(4),
        }}
      >
        <Box sx={TAG}>面接スコア</Box>
        <Box sx={{ display: 'grid', gap: 1.25 }}>
          {SCORES.map((s) => (
            <Box
              key={s.label}
              sx={{
                display: 'grid',
                gridTemplateColumns: '4rem 1fr 2rem',
                alignItems: 'center',
                gap: 1.25,
              }}
            >
              <Box sx={{ fontSize: 12, color: LP.muted }}>{s.label}</Box>
              {/* 棒は角丸にしない。方眼の目盛りに合わせる。 */}
              <Box sx={{ height: 8, bgcolor: LP.paper, border: `1px solid ${LP.ruleSoft}` }}>
                <Box sx={{ width: `${s.v}%`, height: '100%', bgcolor: LP.primary }} />
              </Box>
              <Box
                sx={{
                  fontSize: 12.5,
                  fontWeight: 700,
                  textAlign: 'right',
                  color: LP.ink,
                  fontVariantNumeric: 'tabular-nums',
                }}
              >
                {s.v}
              </Box>
            </Box>
          ))}
        </Box>
      </Box>

      {/* 3. 企業の適合 */}
      <Box
        sx={{
          ...SHEET,
          mr: { md: 3 },
          boxShadow: '3px 4px 0 rgba(28,26,23,.10)',
          ...RISE,
          ...delay(5),
        }}
      >
        <Box sx={TAG}>企業の適合</Box>
        <Box sx={{ display: 'grid' }}>
          {[
            { n: '株式会社サンプルソフト', m: 92 },
            { n: 'サンプル・テクノロジーズ', m: 87 },
          ].map((c, i) => (
            <Box
              key={c.n}
              sx={{
                display: 'flex',
                alignItems: 'baseline',
                gap: 1.5,
                py: 1.25,
                borderTop: i === 0 ? 'none' : `1px solid ${LP.ruleSoft}`,
              }}
            >
              <Box
                sx={{
                  fontSize: 13,
                  fontWeight: 700,
                  color: LP.ink,
                  minWidth: 0,
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
              >
                {c.n}
              </Box>
              <Box
                sx={{
                  ml: 'auto',
                  flexShrink: 0,
                  fontSize: 15,
                  fontWeight: 700,
                  color: LP.primary,
                  fontVariantNumeric: 'tabular-nums',
                  letterSpacing: '-0.01em',
                }}
              >
                {c.m}
                <Box component="span" sx={{ fontSize: 10, ml: 0.25, color: LP.muted }}>
                  ％
                </Box>
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  )
}
