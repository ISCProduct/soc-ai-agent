import { Box } from '@mui/material'
import { LP } from './tokens'

/**
 * ファーストビューのプロダクト図（#1653）。
 *
 * LPのFVは「誰向けの・何の・どんな強みか」を3秒で伝える必要があり、
 * キャッチコピーだけでは足りない。ここでは実画面の語彙
 * （チャット／スコア／企業カード）を簡略化して重ね、
 * 「対話すると、スコアになって、企業に当たる」という製品の筋を1枚で見せる。
 *
 * 画像ファイルを置かずCSSだけで組む。LPは未ログインの初回訪問が主で、
 * Backend 停止中でも出す必要があるため、外部アセットに依存させない。
 * 装飾ではなく製品の説明なので、文言は実物と揃えること。
 */

/** AI面接の評価観点。interview_rubric.go の rubricCriteria と揃える。 */
const SCORES = [
  { label: '論理性', v: 82 },
  { label: '具体性', v: 64 },
  { label: '主体性', v: 91 },
] as const

export function LandingVisual() {
  return (
    <Box
      aria-hidden
      sx={{
        position: 'relative',
        display: 'grid',
        gap: 1.5,
        // 画面を少しだけ傾けて重ねる。情報は左右に流さず縦に積む。
        '& > *': { bgcolor: LP.paper, border: `1px solid ${LP.rule}`, borderRadius: '10px' },
      }}
    >
      {/* 1. 対話 */}
      <Box sx={{ p: 2, boxShadow: '0 1px 2px rgba(14,26,36,.06)' }}>
        <Box sx={{ fontSize: 11, color: LP.muted, letterSpacing: '.08em', mb: 1.25 }}>
          AIとの対話
        </Box>
        <Box
          sx={{
            fontSize: 13,
            lineHeight: 1.7,
            bgcolor: LP.tint,
            borderRadius: '8px 8px 8px 2px',
            p: 1.25,
            maxWidth: '88%',
          }}
        >
          チームで一番こだわったのは、どこですか？
        </Box>
        <Box
          sx={{
            fontSize: 13,
            lineHeight: 1.7,
            color: LP.paper,
            bgcolor: LP.ink,
            borderRadius: '8px 8px 2px 8px',
            p: 1.25,
            mt: 1,
            ml: 'auto',
            maxWidth: '88%',
          }}
        >
          APIの応答が遅くて、原因を切り分けて…
        </Box>
      </Box>

      {/* 2. スコア化 */}
      <Box sx={{ p: 2, boxShadow: '0 1px 2px rgba(14,26,36,.06)' }}>
        <Box sx={{ fontSize: 11, color: LP.muted, letterSpacing: '.08em', mb: 1.25 }}>
          スコアになる
        </Box>
        <Box sx={{ display: 'grid', gap: 1 }}>
          {SCORES.map((s) => (
            <Box
              key={s.label}
              sx={{ display: 'grid', gridTemplateColumns: '4.5rem 1fr 2.2rem', alignItems: 'center', gap: 1 }}
            >
              <Box sx={{ fontSize: 12, color: LP.muted }}>{s.label}</Box>
              <Box sx={{ height: 6, bgcolor: LP.tint, borderRadius: 99 }}>
                <Box sx={{ width: `${s.v}%`, height: '100%', bgcolor: LP.primary, borderRadius: 99 }} />
              </Box>
              <Box
                sx={{
                  fontSize: 12,
                  fontWeight: 700,
                  textAlign: 'right',
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
      <Box sx={{ p: 2, boxShadow: '0 1px 2px rgba(14,26,36,.06)' }}>
        <Box sx={{ fontSize: 11, color: LP.muted, letterSpacing: '.08em', mb: 1.25 }}>
          企業に当たる
        </Box>
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
                borderRadius: '8px',
                px: 1.5,
                py: 1.25,
              }}
            >
              <Box sx={{ fontSize: 13, fontWeight: 700, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {c.n}
              </Box>
              <Box
                sx={{
                  fontSize: 12,
                  fontWeight: 700,
                  color: LP.primary,
                  fontVariantNumeric: 'tabular-nums',
                  flexShrink: 0,
                }}
              >
                適合 {c.m}%
              </Box>
            </Box>
          ))}
        </Box>
      </Box>
    </Box>
  )
}
