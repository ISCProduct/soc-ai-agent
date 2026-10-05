import { Box } from '@mui/material'
import { MessageSquare, Sparkles, Building2 } from 'lucide-react'
import { LP, RISE, delay } from './tokens'

/**
 * ファーストビューのプロダクト図（#1653）。
 *
 * 「アプリの画面」として見せる。小さなパネルを並べるだけだと製品の実体が
 * 伝わらず、LPのFVに必要な「何の製品か3秒で分かる」を満たせない。
 * ウィンドウのクロームと左のナビを付け、実画面の語彙（チャット／スコア／
 * 企業の適合）をそのまま使う。
 *
 * 画像ファイルを置かずCSSだけで組む。LPは未ログインの初回訪問が主で、
 * Backend 停止中でも出す必要があるため、外部アセットに依存させない。
 * 文言とスコアの観点は実物と揃えること（interview_rubric.go の rubricCriteria）。
 */

const SCORES = [
  { label: '論理性', v: 82 },
  { label: '具体性', v: 64 },
  { label: '主体性', v: 91 },
] as const

const NAV_ITEMS = [
  { icon: MessageSquare, label: '診断', active: true },
  { icon: Sparkles, label: '結果', active: false },
  { icon: Building2, label: '企業', active: false },
] as const

export function LandingVisual() {
  return (
    <Box aria-hidden sx={{ position: 'relative', ...RISE, animationDelay: '.24s' }}>
      {/* 背後の光。濃色地に奥行きを作る。 */}
      <Box
        sx={{
          position: 'absolute',
          inset: '-14% -10%',
          background: `radial-gradient(55% 45% at 65% 25%, rgba(86,180,233,.26), transparent 70%),
                       radial-gradient(45% 40% at 20% 85%, rgba(230,159,0,.18), transparent 72%)`,
          filter: 'blur(4px)',
          pointerEvents: 'none',
        }}
      />

      {/* アプリのウィンドウ */}
      <Box
        sx={{
          position: 'relative',
          bgcolor: LP.paper,
          borderRadius: '16px',
          overflow: 'hidden',
          boxShadow: '0 32px 70px rgba(0,0,0,.45), 0 2px 0 rgba(255,255,255,.12) inset',
        }}
      >
        {/* クローム */}
        <Box
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 0.75,
            px: 2,
            py: 1.25,
            borderBottom: `1px solid ${LP.rule}`,
            bgcolor: '#F7FAFC',
          }}
        >
          {['#E5796F', '#E8C35C', '#7FC08A'].map((c) => (
            <Box key={c} sx={{ width: 9, height: 9, borderRadius: 99, bgcolor: c }} />
          ))}
          <Box
            sx={{
              ml: 1.5,
              flexGrow: 1,
              maxWidth: 230,
              height: 18,
              borderRadius: 99,
              bgcolor: LP.paper,
              border: `1px solid ${LP.rule}`,
              display: 'flex',
              alignItems: 'center',
              px: 1.25,
              fontSize: 9.5,
              color: LP.muted,
            }}
          >
            shukatsu-ai.jp
          </Box>
        </Box>

        <Box sx={{ display: 'grid', gridTemplateColumns: '68px 1fr' }}>
          {/* 左ナビ */}
          <Box
            sx={{
              bgcolor: '#F7FAFC',
              borderRight: `1px solid ${LP.rule}`,
              py: 2,
              display: 'grid',
              gap: 0.5,
              alignContent: 'start',
            }}
          >
            {NAV_ITEMS.map(({ icon: Icon, label, active }) => (
              <Box
                key={label}
                sx={{
                  display: 'grid',
                  justifyItems: 'center',
                  gap: 0.5,
                  py: 1,
                  mx: 1,
                  borderRadius: '8px',
                  bgcolor: active ? LP.primary : 'transparent',
                  color: active ? LP.paper : LP.muted,
                }}
              >
                <Icon size={16} strokeWidth={2.2} />
                <Box sx={{ fontSize: 9, fontWeight: 700 }}>{label}</Box>
              </Box>
            ))}
          </Box>

          {/* 本文 */}
          <Box sx={{ p: { xs: 2, md: 2.5 }, display: 'grid', gap: 2 }}>
            {/* 対話 */}
            <Box sx={{ display: 'grid', gap: 1 }}>
              <Box
                sx={{
                  fontSize: 12.5,
                  lineHeight: 1.7,
                  bgcolor: LP.tint,
                  color: LP.ink,
                  borderRadius: '12px 12px 12px 3px',
                  px: 1.5,
                  py: 1.1,
                  maxWidth: '86%',
                }}
              >
                チームで一番こだわったのは、どこですか？
              </Box>
              <Box
                sx={{
                  fontSize: 12.5,
                  lineHeight: 1.7,
                  color: LP.paper,
                  bgcolor: LP.primary,
                  borderRadius: '12px 12px 3px 12px',
                  px: 1.5,
                  py: 1.1,
                  ml: 'auto',
                  maxWidth: '86%',
                }}
              >
                APIの応答が遅くて、原因を切り分けて…
              </Box>
            </Box>

            {/* スコア */}
            <Box
              sx={{
                border: `1px solid ${LP.rule}`,
                borderRadius: '12px',
                p: 1.75,
                display: 'grid',
                gap: 1.1,
              }}
            >
              <Box
                sx={{
                  fontSize: 10,
                  fontWeight: 700,
                  letterSpacing: '.14em',
                  color: LP.muted,
                }}
              >
                面接スコア
              </Box>
              {SCORES.map((s) => (
                <Box
                  key={s.label}
                  sx={{
                    display: 'grid',
                    gridTemplateColumns: '3.8rem 1fr 1.8rem',
                    alignItems: 'center',
                    gap: 1,
                  }}
                >
                  <Box sx={{ fontSize: 11.5, color: LP.muted }}>{s.label}</Box>
                  <Box sx={{ height: 6, bgcolor: LP.tint, borderRadius: 99, overflow: 'hidden' }}>
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
                      fontSize: 11.5,
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

            {/* 企業 */}
            <Box sx={{ display: 'grid', gap: 0.9 }}>
              {[
                { n: '株式会社サンプルソフト', m: 92 },
                { n: 'サンプル・テクノロジーズ', m: 87 },
              ].map((c) => (
                <Box
                  key={c.n}
                  sx={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 1,
                    border: `1px solid ${LP.rule}`,
                    borderRadius: '10px',
                    px: 1.5,
                    py: 1.1,
                  }}
                >
                  <Box
                    sx={{
                      width: 24,
                      height: 24,
                      borderRadius: '6px',
                      bgcolor: LP.tint,
                      color: LP.primary,
                      display: 'grid',
                      placeItems: 'center',
                      flexShrink: 0,
                    }}
                  >
                    <Building2 size={13} strokeWidth={2.2} />
                  </Box>
                  <Box
                    sx={{
                      fontSize: 12,
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
                      fontSize: 10.5,
                      fontWeight: 700,
                      color: LP.primary,
                      bgcolor: LP.tint,
                      borderRadius: 99,
                      px: 1.1,
                      py: 0.3,
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

      {/* 手前に浮かせる小片。奥行きを強める。 */}
      <Box
        sx={{
          position: 'absolute',
          right: { xs: -8, md: -26 },
          bottom: { xs: -14, md: -24 },
          bgcolor: LP.paper,
          borderRadius: '12px',
          boxShadow: '0 20px 44px rgba(0,0,0,.42)',
          px: 2,
          py: 1.5,
          display: 'grid',
          gap: 0.4,
          ...RISE,
          ...delay(6),
        }}
      >
        <Box sx={{ fontSize: 9.5, fontWeight: 700, letterSpacing: '.14em', color: LP.muted }}>
          ES添削
        </Box>
        <Box sx={{ fontSize: 13, fontWeight: 700, color: LP.ink }}>
          400字に整えました
        </Box>
      </Box>
    </Box>
  )
}
