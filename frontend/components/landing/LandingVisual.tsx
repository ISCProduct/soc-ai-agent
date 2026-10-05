import { Box } from '@mui/material'
import { LP, RISE, delay } from './tokens'

/**
 * ファーストビューのプロダクト図（#1653）。
 *
 * LPのFVは「誰向けの・何の・どんな強みか」を3秒で伝える必要があり、
 * キャッチコピーだけでは足りない。実画面の語彙（対話／スコア／企業の適合）を
 * 簡略化して重ね、「話すと、スコアになって、企業に当たる」という製品の筋を
 * 1枚で見せる。
 *
 * 濃色の地に白い面を浮かせることで奥行きを出す。パネルは少しずつ横へずらし、
 * 下へ行くほど手前に来るように影を強くする。装飾のための傾きは入れない
 * （読みにくくなるだけで、製品の説明にならない）。
 *
 * 画像ファイルを置かずCSSだけで組む。LPは未ログインの初回訪問が主で、
 * Backend 停止中でも出す必要があるため、外部アセットに依存させない。
 * 文言は実物と揃えること。スコアの観点は interview_rubric.go の
 * rubricCriteria と同じ。
 */

/** AI面接の評価観点。interview_rubric.go の rubricCriteria と揃える。 */
const SCORES = [
  { label: '論理性', v: 82 },
  { label: '具体性', v: 64 },
  { label: '主体性', v: 91 },
] as const

const PANEL = {
  bgcolor: LP.paper,
  borderRadius: '14px',
  border: '1px solid rgba(255,255,255,.10)',
  p: { xs: 2, md: 2.5 },
} as const

const CAPTION = {
  fontSize: 10.5,
  fontWeight: 700,
  letterSpacing: '.16em',
  color: LP.muted,
  mb: 1.5,
} as const

export function LandingVisual() {
  return (
    <Box aria-hidden sx={{ position: 'relative', ...RISE, animationDelay: '.22s' }}>
      {/* 背後の光。濃色地に奥行きを作る。 */}
      <Box
        sx={{
          position: 'absolute',
          inset: '-18% -12% -8%',
          background: `radial-gradient(60% 50% at 60% 30%, rgba(86,180,233,.22), transparent 70%),
                       radial-gradient(45% 40% at 25% 85%, rgba(230,159,0,.16), transparent 70%)`,
          filter: 'blur(6px)',
          pointerEvents: 'none',
        }}
      />

      <Box sx={{ position: 'relative', display: 'grid', gap: { xs: 1.5, md: 2 } }}>
        {/* 1. 対話 */}
        <Box
          sx={{
            ...PANEL,
            ml: { md: 0 },
            mr: { md: 5 },
            boxShadow: '0 10px 24px rgba(0,0,0,.28)',
            ...RISE,
            ...delay(3),
          }}
        >
          <Box sx={CAPTION}>AIとの対話</Box>
          <Box
            sx={{
              fontSize: 13,
              lineHeight: 1.75,
              bgcolor: LP.tint,
              color: LP.ink,
              borderRadius: '12px 12px 12px 3px',
              px: 1.5,
              py: 1.25,
              maxWidth: '90%',
            }}
          >
            チームで一番こだわったのは、どこですか？
          </Box>
          <Box
            sx={{
              fontSize: 13,
              lineHeight: 1.75,
              color: LP.paper,
              bgcolor: LP.primary,
              borderRadius: '12px 12px 3px 12px',
              px: 1.5,
              py: 1.25,
              mt: 1.25,
              ml: 'auto',
              maxWidth: '90%',
            }}
          >
            APIの応答が遅くて、原因を切り分けて…
          </Box>
        </Box>

        {/* 2. スコア化 */}
        <Box
          sx={{
            ...PANEL,
            ml: { md: 4 },
            boxShadow: '0 16px 34px rgba(0,0,0,.34)',
            ...RISE,
            ...delay(4),
          }}
        >
          <Box sx={CAPTION}>スコアになる</Box>
          <Box sx={{ display: 'grid', gap: 1.25 }}>
            {SCORES.map((s) => (
              <Box
                key={s.label}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: '4.2rem 1fr 2rem',
                  alignItems: 'center',
                  gap: 1.25,
                }}
              >
                <Box sx={{ fontSize: 12, color: LP.muted }}>{s.label}</Box>
                <Box sx={{ height: 7, bgcolor: LP.tint, borderRadius: 99, overflow: 'hidden' }}>
                  <Box
                    sx={{
                      width: `${s.v}%`,
                      height: '100%',
                      borderRadius: 99,
                      background: `linear-gradient(90deg, ${LP.primary}, ${LP.primaryOnDark})`,
                    }}
                  />
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

        {/* 3. 企業に当たる */}
        <Box
          sx={{
            ...PANEL,
            ml: { md: 1 },
            mr: { md: 3 },
            boxShadow: '0 22px 44px rgba(0,0,0,.40)',
            ...RISE,
            ...delay(5),
          }}
        >
          <Box sx={CAPTION}>企業に当たる</Box>
          <Box sx={{ display: 'grid', gap: 1 }}>
            {[
              { n: '株式会社サンプルソフト', m: 92 },
              { n: 'サンプル・テクノロジーズ', m: 87 },
            ].map((c) => (
              <Box
                key={c.n}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1,
                  border: `1px solid ${LP.rule}`,
                  borderRadius: '10px',
                  px: 1.5,
                  py: 1.25,
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
                    flexShrink: 0,
                    fontSize: 11.5,
                    fontWeight: 700,
                    color: LP.primary,
                    bgcolor: LP.tint,
                    borderRadius: 99,
                    px: 1.25,
                    py: 0.25,
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  適合 {c.m}%
                </Box>
              </Box>
            ))}
          </Box>
        </Box>
      </Box>
    </Box>
  )
}
