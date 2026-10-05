import { Box, Button, Container, Typography } from '@mui/material'

/**
 * 未ログインの訪問者に出す公開ランディングページ（#1653）。
 *
 * 以前は `/` が認証必須で、未ログインは即 `/login` へ飛ばされていた。
 * 製品の説明は `/login` の1行だけで、展示会のQRから来た来場者は
 * いきなりログインフォームを見ることになっていた。
 *
 * 未ログインで到達する経路は3系統（学生 / 企業 / 教員・学校管理者）ある。
 * ここでその3つへ振り分ける。
 *
 * **このコンポーネントはAPIを呼ばない。** 本番は展示会運用で `desired=0` から
 * 起動するため、Backend の起動を待たずに表示できる必要がある
 * （docs: Issue #1653 の受け入れ条件6）。Server Component のまま保つこと。
 *
 * リンクに `component={Link}` を使わないこと。MUI の Button は Client Component で、
 * Server Component から関数（コンポーネント参照）を渡すと RSC の境界を越えられず
 * 「Functions cannot be passed directly to Client Components」で描画ごと落ちる。
 * ビルドも型検査も通り、落ちるのは実行時（エラー境界が出る）。
 * `href` だけ渡せば Button はアンカーを描画する。
 *
 * ## 体裁
 *
 * カード・影・大きな角丸を使わない。`lib/design-tokens.ts` が
 * 「罫線がカードと影の代わりに構造を担う」「カードを大量に使わない」と定めており、
 * プロジェクトルール §18 / §27 も同じ。見出し＋説明＋ボタンの角丸カードを積む形は
 * この製品の他の画面と揃わない。
 *
 * 構造は「ラベル列 ＋ 横罫」の1パターンだけで通す。学生側の書体 BIZ UDPGothic は
 * 学校配布物・自治体文書のUD書体なので、その字面に合う組み方にする。
 * 既存テーマの `shape.borderRadius: 12` が効くため、ボタンでは明示的に打ち消す。
 */

/** 振り分け先。未ログインで到達する3系統に対応する。 */
const ENTRANCES = [
  {
    label: '学生',
    target: '専門学校で就職活動をしている方',
    body: 'AIとの対話で適性を診断します。IT企業とのマッチング、面接練習、履歴書レビュー、ES添削まで、ひと続きで進められます。',
    action: 'ログイン・新規登録',
    href: '/login',
    primary: true,
  },
  {
    label: '企業',
    target: '学生の採用を検討している企業の採用担当者',
    body: '求人を掲載して応募を受け付けられます。スカウト公開に同意した学生を検索し、直接スカウトを送ることもできます。',
    action: '企業ポータルへ',
    href: '/company-portal/sign-in',
    primary: false,
  },
  {
    label: '学校・教員',
    target: '導入校の先生・就職課の方',
    body: '学生の就活状況、要フォローの学生、応募・面接・内定の状況を確認できます。',
    action: '管理画面へ',
    href: '/admin',
    primary: false,
  },
] as const

/**
 * 学生が辿る順序。番号は装飾ではなく就職活動の進み方そのもの。
 * 順序に意味が無ければ番号は振らない。
 */
const STEPS = [
  { n: '1', title: '適性を診断する', body: 'AIとの対話から強み・志向を整理し、スコアとして蓄積します。' },
  { n: '2', title: '企業と出会う', body: '診断結果と、企業側が求める人物像を突き合わせて候補を出します。' },
  { n: '3', title: '面接を練習する', body: '音声で面接を練習し、論理性・具体性・主体性などの観点で講評を受け取れます。' },
  { n: '4', title: '書類を仕上げる', body: '職務経歴書のレビューと、ESの添削・リライトができます。' },
] as const

/** 罫線。影とカードの代わりにこれで構造を作る。 */
const RULE = '#C9D2DC'
const RULE_STRONG = '#111111'

/** ラベル列の幅。入口と順序で同じ値を使い、縦のラインを揃える。 */
const LABEL_W = '7.5rem'

export function LandingContent() {
  return (
    <Box component="main" sx={{ bgcolor: 'background.paper', minHeight: '100vh', pb: 10 }}>
      <Container maxWidth="md" sx={{ px: { xs: 2.5, sm: 4 } }}>
        <Box component="header" sx={{ pt: { xs: 5, sm: 8 } }}>
          <Typography
            component="p"
            sx={{ fontSize: 13, letterSpacing: '0.24em', color: 'text.secondary' }}
          >
            就活AI
          </Typography>
          <Typography
            component="h1"
            sx={{
              mt: 0.5,
              fontSize: { xs: 30, sm: 42 },
              fontWeight: 700,
              letterSpacing: '-0.01em',
              lineHeight: 1.25,
            }}
          >
            IT企業エージェント
          </Typography>
          {/* 題字を二重罫で受ける。和文の見出し罫。 */}
          <Box sx={{ borderTop: `3px solid ${RULE_STRONG}`, mt: 2 }} />
          <Box sx={{ borderTop: `1px solid ${RULE_STRONG}`, mt: '3px' }} />
          <Typography
            component="p"
            sx={{ mt: 2.5, fontSize: { xs: 15, sm: 17 }, maxWidth: '34em', lineHeight: 1.9 }}
          >
            適性診断から企業マッチングまで。
            <br />
            専門学校の就職活動を支援するAIエージェントです。
          </Typography>
        </Box>

        <Box component="section" sx={{ mt: { xs: 6, sm: 9 } }}>
          <Typography
            component="h2"
            sx={{ fontSize: 13, letterSpacing: '0.2em', color: 'text.secondary', mb: 2 }}
          >
            ご利用の方
          </Typography>

          <Box sx={{ borderTop: `2px solid ${RULE_STRONG}` }}>
            {ENTRANCES.map((e) => (
              <Box
                key={e.href}
                sx={{
                  borderBottom: `1px solid ${RULE}`,
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', sm: `${LABEL_W} 1fr` },
                  columnGap: 3,
                  rowGap: 1,
                  py: { xs: 3, sm: 3.5 },
                }}
              >
                <Typography
                  component="h3"
                  sx={{ fontSize: { xs: 19, sm: 20 }, fontWeight: 700, lineHeight: 1.5 }}
                >
                  {e.label}
                </Typography>
                <Box sx={{ minWidth: 0 }}>
                  <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>{e.target}</Typography>
                  <Typography sx={{ mt: 1, fontSize: 15, lineHeight: 1.9, maxWidth: '36em' }}>
                    {e.body}
                  </Typography>
                  <Button
                    href={e.href}
                    disableElevation
                    variant={e.primary ? 'contained' : 'outlined'}
                    sx={{
                      mt: 2,
                      borderRadius: '2px',
                      fontSize: 15,
                      fontWeight: 700,
                      px: 2.5,
                      py: 1,
                      width: { xs: '100%', sm: 'auto' },
                      ...(e.primary ? {} : { borderColor: RULE_STRONG, color: 'text.primary' }),
                    }}
                  >
                    {e.action}
                  </Button>
                </Box>
              </Box>
            ))}
          </Box>
        </Box>

        <Box component="section" sx={{ mt: { xs: 6, sm: 9 } }}>
          <Typography
            component="h2"
            sx={{ fontSize: 13, letterSpacing: '0.2em', color: 'text.secondary', mb: 2 }}
          >
            学生が進める順序
          </Typography>

          <Box sx={{ borderTop: `2px solid ${RULE_STRONG}` }}>
            {STEPS.map((s) => (
              <Box
                key={s.n}
                sx={{
                  borderBottom: `1px solid ${RULE}`,
                  display: 'grid',
                  gridTemplateColumns: { xs: '2.5rem 1fr', sm: `${LABEL_W} 1fr` },
                  columnGap: { xs: 2, sm: 3 },
                  py: 2.5,
                  alignItems: 'baseline',
                }}
              >
                <Typography
                  aria-hidden
                  sx={{
                    fontSize: { xs: 15, sm: 17 },
                    fontWeight: 700,
                    color: 'text.secondary',
                    fontVariantNumeric: 'tabular-nums',
                  }}
                >
                  {s.n}
                </Typography>
                <Box sx={{ minWidth: 0 }}>
                  <Typography component="h3" sx={{ fontSize: 16, fontWeight: 700 }}>
                    {s.title}
                  </Typography>
                  <Typography
                    sx={{ mt: 0.5, fontSize: 14, color: 'text.secondary', lineHeight: 1.9 }}
                  >
                    {s.body}
                  </Typography>
                </Box>
              </Box>
            ))}
          </Box>
        </Box>

        <Box
          component="footer"
          sx={{
            mt: { xs: 6, sm: 9 },
            pt: 2.5,
            borderTop: `1px solid ${RULE}`,
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'baseline',
            gap: 2.5,
          }}
        >
          <Typography sx={{ fontSize: 13, color: 'text.secondary' }}>© 就活AI</Typography>
          <Typography
            component="a"
            href="/privacy"
            sx={{ fontSize: 13, color: 'text.primary', textUnderlineOffset: '3px' }}
          >
            プライバシーポリシー
          </Typography>
        </Box>
      </Container>
    </Box>
  )
}
