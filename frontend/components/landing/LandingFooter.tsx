import { Box, Container } from '@mui/material'
import { LP } from './tokens'

/**
 * LPのフッター（#1653）。
 *
 * **実在しないリンクは置かない。** 会社概要・利用規約・お問い合わせは
 * ページがまだ無いので載せていない。ダミーの # を置くと、押して何も
 * 起きない導線になる。ページができた時点で足すこと。
 */

const COLUMNS = [
  {
    title: 'ご利用の方',
    links: [
      { label: '学生ログイン・新規登録', href: '/login' },
      { label: '企業ポータル', href: '/company-portal/sign-in' },
      { label: '学校・教員向け管理画面', href: '/admin' },
    ],
  },
  {
    title: 'サポート',
    links: [
      { label: 'パスワードをお忘れの方', href: '/forgot-password' },
      { label: 'プライバシーポリシー', href: '/privacy' },
    ],
  },
] as const

export function LandingFooter() {
  return (
    <Box component="footer" sx={{ bgcolor: LP.paper, borderTop: `2px solid ${LP.ink}` }}>
      <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 5 }, py: { xs: 6, md: 8 } }}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '1.5fr repeat(2, 1fr)' },
            gap: { xs: 4.5, md: 6 },
          }}
        >
          <Box>
            <Box sx={{ fontSize: 19, fontWeight: 700, color: LP.ink, letterSpacing: '.02em' }}>
              就活AI
            </Box>
            <Box
              sx={{ mt: 1.5, fontSize: 12.5, lineHeight: 2.1, color: LP.muted, maxWidth: '24em' }}
            >
              適性診断から企業マッチングまで。
              <br />
              専門学校の就職活動を支援するAIエージェントです。
            </Box>
          </Box>

          {COLUMNS.map((c) => (
            <Box key={c.title}>
              <Box
                sx={{
                  fontSize: 11,
                  fontWeight: 700,
                  color: LP.ink,
                  pb: 1,
                  mb: 2,
                  borderBottom: `1px solid ${LP.rule}`,
                  letterSpacing: '.1em',
                }}
              >
                {c.title}
              </Box>
              <Box sx={{ display: 'grid', gap: 1.5 }}>
                {c.links.map((l) => (
                  <Box
                    key={l.href}
                    component="a"
                    href={l.href}
                    sx={{
                      fontSize: 13,
                      color: LP.inkSoft,
                      textDecoration: 'none',
                      '&:hover': { textDecoration: 'underline' },
                    }}
                  >
                    {l.label}
                  </Box>
                ))}
              </Box>
            </Box>
          ))}
        </Box>

        <Box sx={{ mt: { xs: 5, md: 7 }, fontSize: 12, color: LP.muted }}>© 就活AI</Box>
      </Container>
    </Box>
  )
}
