import { Box, Button, Container } from '@mui/material'
import { LP } from './tokens'

/**
 * LPの固定ヘッダー（#1653）。
 *
 * サービスLPにヘッダーが無いと、それだけで作りかけに見える。
 * ロゴ・ページ内ナビ・CTA の3点を常に出す。
 *
 * ページ内アンカーだけで動かし、JS は足さない（LandingContent を
 * Server Component のまま保つため）。スクロール連動の縮小なども入れない。
 */

const NAV = [
  { label: '課題', href: '#issue' },
  { label: 'できること', href: '#solution' },
  { label: 'ご利用の方', href: '#entrances' },
  { label: 'よくある質問', href: '#faq' },
] as const

export function LandingHeader() {
  return (
    <Box
      component="header"
      sx={{
        position: 'sticky',
        top: 0,
        zIndex: 10,
        bgcolor: 'rgba(12,22,32,.86)',
        backdropFilter: 'blur(10px)',
        borderBottom: '1px solid rgba(255,255,255,.10)',
        color: LP.paper,
      }}
    >
      <Container
        maxWidth="lg"
        sx={{
          px: { xs: 2.5, md: 4 },
          minHeight: { xs: 58, md: 68 },
          display: 'flex',
          alignItems: 'center',
          gap: 2,
        }}
      >
        <Box
          component="a"
          href="#top"
          sx={{
            display: 'flex',
            alignItems: 'baseline',
            gap: 0.75,
            textDecoration: 'none',
            color: 'inherit',
            flexShrink: 0,
          }}
        >
          <Box sx={{ fontSize: { xs: 17, md: 19 }, fontWeight: 700, letterSpacing: '-0.02em' }}>
            就活AI
          </Box>
          <Box
            sx={{
              fontSize: 10.5,
              fontWeight: 700,
              letterSpacing: '.14em',
              color: 'rgba(255,255,255,.55)',
              display: { xs: 'none', sm: 'block' },
            }}
          >
            IT企業エージェント
          </Box>
        </Box>

        <Box
          component="nav"
          sx={{
            ml: 'auto',
            display: { xs: 'none', md: 'flex' },
            alignItems: 'center',
            gap: 0.5,
          }}
        >
          {NAV.map((n) => (
            <Box
              key={n.href}
              component="a"
              href={n.href}
              sx={{
                fontSize: 13.5,
                fontWeight: 700,
                color: 'rgba(255,255,255,.80)',
                textDecoration: 'none',
                px: 1.5,
                py: 1,
                borderRadius: '6px',
                transition: 'color .2s, background-color .2s',
                '&:hover': { color: LP.paper, bgcolor: 'rgba(255,255,255,.08)' },
              }}
            >
              {n.label}
            </Box>
          ))}
        </Box>

        <Button
          href="/login"
          disableElevation
          variant="contained"
          sx={{
            ml: { xs: 'auto', md: 1.5 },
            flexShrink: 0,
            bgcolor: LP.paper,
            color: LP.ink,
            borderRadius: '7px',
            fontSize: { xs: 13, md: 14 },
            fontWeight: 700,
            px: { xs: 2, md: 2.75 },
            py: 1,
            '&:hover': { bgcolor: 'rgba(255,255,255,.88)' },
          }}
        >
          ログイン
        </Button>
      </Container>
    </Box>
  )
}
