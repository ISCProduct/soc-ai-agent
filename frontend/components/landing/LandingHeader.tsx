import { Box, Button, Container } from '@mui/material'
import { LP } from './tokens'

/**
 * LPのヘッダー（#1653）。
 *
 * 半透明＋blur の固定ヘッダーは SaaS の LP で最も使い回されている形なので使わない。
 * 紙に押した版面のヘッダーとして、紙色のまま太い下罫で受ける。
 *
 * ページ内アンカーだけで動かし、JS は足さない
 * （LandingContent を Server Component のまま保つため）。
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
        bgcolor: LP.paper,
        borderBottom: `2px solid ${LP.ink}`,
      }}
    >
      <Container
        maxWidth="lg"
        sx={{
          px: { xs: 2.5, md: 5 },
          minHeight: { xs: 56, md: 64 },
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
            gap: 1,
            textDecoration: 'none',
            color: LP.ink,
            flexShrink: 0,
          }}
        >
          <Box sx={{ fontSize: { xs: 17, md: 18 }, fontWeight: 700, letterSpacing: '.02em' }}>
            就活AI
          </Box>
          <Box
            sx={{
              fontSize: 10.5,
              color: LP.muted,
              display: { xs: 'none', sm: 'block' },
              letterSpacing: '.08em',
            }}
          >
            IT企業エージェント
          </Box>
        </Box>

        <Box
          component="nav"
          sx={{ ml: 'auto', display: { xs: 'none', md: 'flex' }, alignItems: 'center', gap: 3 }}
        >
          {NAV.map((n) => (
            <Box
              key={n.href}
              component="a"
              href={n.href}
              sx={{
                fontSize: 13.5,
                color: LP.inkSoft,
                textDecoration: 'none',
                pb: 0.25,
                borderBottom: '1px solid transparent',
                transition: 'border-color .2s',
                '&:hover': { borderBottomColor: LP.ink },
                '@media (prefers-reduced-motion: reduce)': { transition: 'none' },
              }}
            >
              {n.label}
            </Box>
          ))}
        </Box>

        <Button
          href="/login"
          disableElevation
          variant="outlined"
          sx={{
            ml: { xs: 'auto', md: 3 },
            flexShrink: 0,
            borderRadius: 0,
            borderColor: LP.ink,
            color: LP.ink,
            fontSize: { xs: 12.5, md: 13.5 },
            fontWeight: 700,
            px: { xs: 1.75, md: 2.5 },
            py: 0.75,
            '&:hover': { bgcolor: LP.ink, color: LP.paper, borderColor: LP.ink },
          }}
        >
          ログイン
        </Button>
      </Container>
    </Box>
  )
}
