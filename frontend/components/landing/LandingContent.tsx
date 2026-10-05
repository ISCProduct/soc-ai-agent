import { Box, Button, Container, Typography } from '@mui/material'
import { LandingVisual } from './LandingVisual'
import { LP } from './tokens'

/**
 * 未ログインの訪問者に出す公開ランディングページ（#1653）。
 *
 * 以前は `/` が認証必須で、未ログインは即 `/login` へ飛ばされていた。
 * 製品の説明は `/login` の1行だけで、展示会のQRから来た来場者は
 * いきなりログインフォームを見ることになっていた。
 *
 * **このコンポーネントはAPIを呼ばない。** 本番は展示会運用で `desired=0` から
 * 起動するため、Backend の起動を待たずに表示できる必要がある
 * （受け入れ条件6）。Server Component のまま保つこと。
 *
 * リンクに `component={Link}` を使わないこと。MUI の Button は Client Component で、
 * Server Component から関数を渡すと RSC の境界を越えられず
 * 「Functions cannot be passed directly to Client Components」で描画ごと落ちる。
 * ビルドも型検査も通り、落ちるのは実行時。`href` だけ渡せばアンカーになる。
 *
 * ## 構成
 *
 * LPの定石に沿う。「できることの一覧」だけの索引ページにしない。
 *
 *   FV（誰向け・何・強み＋CTA＋事実） → 課題提起 → 解決策（課題と対応させる）
 *   → 3者の入口 → FAQ → クロージングCTA
 *
 * BtoB寄りの商材なので、感情訴求や限定訴求は入れない。
 * 「校内・社内で説明できる理屈」を優先する。
 *
 * ## 数字について
 *
 * **持っていない数字は書かない。** 導入校数・内定率・満足度は計測していないので
 * 載せない。載せるのは製品の事実（評価観点5つ、スコアの構成、掲載企業数）だけ。
 * 導入事例もまだ無いので、そのセクション自体を置いていない。
 * 数字を足すときは出どころを確認すること。
 */

/** 就活で実際に詰まるところ。機能から逆算せず、利用者の言葉で書く。 */
const PROBLEMS = [
  {
    who: '学生',
    quote: '何から始めればいいか分からない',
    body: '自己分析のやり方が分からないまま、とりあえず求人サイトを眺めて終わってしまう。',
  },
  {
    who: '学生',
    quote: '面接の練習相手がいない',
    body: '先生の時間は限られていて、本番までに数回しか練習できない。何が悪かったのかも分からない。',
  },
  {
    who: '先生',
    quote: '一人ひとりの状況を把握しきれない',
    body: '誰が止まっているのか、どこで止まっているのかが、面談するまで見えない。',
  },
] as const

/** 課題への対応。機能名の羅列にせず「どの課題にどう効くか」で並べる。 */
const SOLUTIONS = [
  {
    n: '01',
    title: 'AIと話すだけで、自己分析が進む',
    forWhom: '何から始めればいいか分からない',
    body: 'チャットで答えていくと、強みと志向が10カテゴリのスコアになります。就活の進み方に合わせて4フェーズで記録するので、自分がどう変わったかも残ります。',
  },
  {
    n: '02',
    title: 'スコアに合う企業が、根拠つきで出る',
    forWhom: '求人サイトを眺めて終わってしまう',
    body: '診断結果と、企業側が求める人物像を突き合わせて候補を出します。なぜ合うのかが見えるので、志望動機にそのまま使えます。',
  },
  {
    n: '03',
    title: '面接は何度でも、音声で練習できる',
    forWhom: '面接の練習相手がいない',
    body: '論理性・具体性・主体性・コミュニケーション力・積極性の5つの観点で講評します。根拠は実際の発言から引用するので、どこを直すかが分かります。',
  },
  {
    n: '04',
    title: '書類は添削まで、ひと続きで',
    forWhom: 'ESが書けない',
    body: '職務経歴書のレビューと、ESの添削・リライト。設問の文字数上限に合わせて整えます。',
  },
] as const

/** 製品の事実だけ。計測していない指標は載せない。 */
const FACTS = [
  { v: '4万社', k: 'から企業を選定' },
  { v: '10カテゴリ', k: '× 4フェーズでスコア化' },
  { v: '5観点', k: 'で面接を講評' },
] as const

/** 振り分け先。未ログインで到達する3系統に対応する。 */
const ENTRANCES = [
  {
    label: '学生',
    target: '専門学校で就職活動をしている方',
    action: '無料で診断を始める',
    href: '/login',
    primary: true,
  },
  {
    label: '企業',
    target: '学生の採用を検討している企業の採用担当者',
    action: '企業ポータルへ',
    href: '/company-portal/sign-in',
    primary: false,
  },
  {
    label: '学校・教員',
    target: '導入校の先生・就職課の方',
    action: '管理画面へ',
    href: '/admin',
    primary: false,
  },
] as const

/** 利用前に解消したい実務的な疑問を中心に置く。 */
const FAQS = [
  {
    q: '利用にお金はかかりますか',
    a: '在校生の利用は学校の導入に含まれます。学生個人への課金はありません。',
  },
  {
    q: 'AIとの会話や面接の内容は、先生に見られますか',
    a: '就職指導のため、先生は担当学生の就活状況とスコアを確認できます。企業に渡るのは、スカウト公開に同意した情報だけです。',
  },
  {
    q: '面接練習にマイクは必要ですか',
    a: '音声で行うため、マイクのある端末が必要です。ブラウザだけで動き、アプリの導入は要りません。',
  },
  {
    q: '企業として求人を載せたいのですが',
    a: '企業ポータルからアカウントを作成し、求人を登録してください。掲載には学校側の審査があります。',
  },
] as const

/** セクション見出し。小さいラベルと大きい見出しの2段で統一する。 */
function SectionHead({ label, title, light }: { label: string; title: string; light?: boolean }) {
  return (
    <Box sx={{ mb: { xs: 3, md: 5 } }}>
      <Typography
        component="p"
        sx={{
          fontSize: 12,
          fontWeight: 700,
          letterSpacing: '.18em',
          color: light ? LP.accent : LP.primary,
        }}
      >
        {label}
      </Typography>
      <Typography
        component="h2"
        sx={{
          mt: 1,
          fontSize: { xs: 24, md: 32 },
          fontWeight: 700,
          lineHeight: 1.45,
          color: light ? LP.paper : LP.ink,
        }}
      >
        {title}
      </Typography>
    </Box>
  )
}

export function LandingContent() {
  return (
    <Box component="main" sx={{ bgcolor: LP.paper, color: LP.ink }}>
      {/* ── ファーストビュー ───────────────────────── */}
      <Box
        sx={{
          background: `linear-gradient(180deg, ${LP.tint} 0%, ${LP.paper} 100%)`,
          borderBottom: `1px solid ${LP.rule}`,
        }}
      >
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 6, md: 10 } }}>
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: '1.15fr 0.85fr' },
              gap: { xs: 5, md: 8 },
              alignItems: 'center',
            }}
          >
            <Box>
              <Typography
                component="p"
                sx={{ fontSize: 13, fontWeight: 700, letterSpacing: '.16em', color: LP.primary }}
              >
                専門学校の就職活動を支援するAIエージェント
              </Typography>

              <Typography
                component="h1"
                sx={{
                  mt: 2,
                  fontSize: { xs: 32, sm: 44, md: 52 },
                  fontWeight: 700,
                  lineHeight: 1.3,
                  letterSpacing: '-0.02em',
                }}
              >
                何から始めればいいか
                <br />
                分からない就活を、
                <br />
                <Box
                  component="span"
                  sx={{
                    // 強調は面で塗らず下に引く。文字の可読性を落とさない。
                    // マーカーは行をまたぐと途切れて見えるので、必ず単独行に置く。
                    display: 'inline-block',
                    backgroundImage: `linear-gradient(${LP.accent}, ${LP.accent})`,
                    backgroundSize: '100% 0.34em',
                    backgroundPosition: '0 88%',
                    backgroundRepeat: 'no-repeat',
                  }}
                >
                  最初の一歩から。
                </Box>
              </Typography>

              <Typography
                sx={{ mt: 3, fontSize: { xs: 15, md: 17 }, lineHeight: 2, maxWidth: '32em' }}
              >
                AIとの対話で適性を診断し、企業とのマッチング、面接練習、履歴書・ES添削まで。
                就職活動をひと続きで進められます。
              </Typography>

              <Box sx={{ mt: 4, display: 'flex', flexWrap: 'wrap', gap: 2, alignItems: 'center' }}>
                <Button
                  href="/login"
                  disableElevation
                  variant="contained"
                  sx={{
                    bgcolor: LP.primary,
                    borderRadius: '6px',
                    fontSize: 16,
                    fontWeight: 700,
                    px: 4,
                    py: 1.5,
                    '&:hover': { bgcolor: '#005B8E' },
                  }}
                >
                  無料で診断を始める
                </Button>
                <Button
                  href="#entrances"
                  sx={{ fontSize: 15, fontWeight: 700, color: LP.ink, borderRadius: '6px' }}
                >
                  企業・学校の方はこちら
                </Button>
              </Box>

              {/* 権威づけの代わりに製品の事実を置く。実績値は持っていないので使わない。 */}
              <Box
                sx={{
                  mt: 5,
                  pt: 3,
                  borderTop: `1px solid ${LP.rule}`,
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: { xs: 3, sm: 5 },
                }}
              >
                {FACTS.map((f) => (
                  <Box key={f.v}>
                    <Typography
                      sx={{ fontSize: { xs: 22, md: 26 }, fontWeight: 700, lineHeight: 1.2 }}
                    >
                      {f.v}
                    </Typography>
                    <Typography sx={{ fontSize: 12, color: LP.muted, mt: 0.5 }}>{f.k}</Typography>
                  </Box>
                ))}
              </Box>
            </Box>

            <LandingVisual />
          </Box>
        </Container>
      </Box>

      {/* ── 課題提起 ─────────────────────────────── */}
      <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 12 } }}>
        <SectionHead label="ISSUE" title="就活は、つまずく場所が決まっています。" />
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
            gap: { xs: 3, md: 4 },
          }}
        >
          {PROBLEMS.map((p) => (
            <Box key={p.quote} sx={{ borderTop: `3px solid ${LP.ink}`, pt: 2.5 }}>
              <Typography sx={{ fontSize: 12, fontWeight: 700, color: LP.muted }}>
                {p.who}
              </Typography>
              <Typography
                sx={{ mt: 1, fontSize: { xs: 18, md: 19 }, fontWeight: 700, lineHeight: 1.6 }}
              >
                「{p.quote}」
              </Typography>
              <Typography sx={{ mt: 1.5, fontSize: 14, lineHeight: 1.95, color: LP.muted }}>
                {p.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </Container>

      {/* ── 解決策（濃い地で切り替える） ────────────────── */}
      <Box sx={{ bgcolor: LP.ink, color: LP.paper }}>
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 12 } }}>
          <SectionHead light label="SOLUTION" title="つまずく場所に、ひとつずつ手を当てます。" />
          <Box sx={{ display: 'grid', gap: { xs: 4, md: 5 } }}>
            {SOLUTIONS.map((s) => (
              <Box
                key={s.n}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', md: '4rem 1fr 1fr' },
                  gap: { xs: 1.5, md: 4 },
                  pt: { xs: 3, md: 4 },
                  borderTop: '1px solid rgba(255,255,255,.18)',
                  alignItems: 'start',
                }}
              >
                <Typography
                  sx={{
                    fontSize: { xs: 13, md: 15 },
                    fontWeight: 700,
                    color: LP.accent,
                    fontVariantNumeric: 'tabular-nums',
                    letterSpacing: '.06em',
                  }}
                >
                  {s.n}
                </Typography>
                <Box>
                  <Typography
                    component="h3"
                    sx={{ fontSize: { xs: 19, md: 22 }, fontWeight: 700, lineHeight: 1.55 }}
                  >
                    {s.title}
                  </Typography>
                  <Typography sx={{ mt: 1.5, fontSize: 12, color: LP.accent, fontWeight: 700 }}>
                    → 「{s.forWhom}」に効きます
                  </Typography>
                </Box>
                <Typography sx={{ fontSize: 14, lineHeight: 2, color: 'rgba(255,255,255,.78)' }}>
                  {s.body}
                </Typography>
              </Box>
            ))}
          </Box>
        </Container>
      </Box>

      {/* ── 3者の入口 ────────────────────────────── */}
      <Box id="entrances" sx={{ bgcolor: LP.tint }}>
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 12 } }}>
          <SectionHead label="ENTRANCE" title="ご利用の方を選んでください。" />
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 2.5,
            }}
          >
            {ENTRANCES.map((e) => (
              <Box
                key={e.href}
                sx={{
                  bgcolor: LP.paper,
                  border: `1px solid ${e.primary ? LP.primary : LP.rule}`,
                  borderTopWidth: e.primary ? 4 : 1,
                  borderRadius: '8px',
                  p: { xs: 2.5, md: 3 },
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 1,
                }}
              >
                <Typography component="h3" sx={{ fontSize: 20, fontWeight: 700 }}>
                  {e.label}
                </Typography>
                <Typography sx={{ fontSize: 13, color: LP.muted, lineHeight: 1.8, flexGrow: 1 }}>
                  {e.target}
                </Typography>
                <Button
                  href={e.href}
                  disableElevation
                  variant={e.primary ? 'contained' : 'outlined'}
                  sx={{
                    mt: 1.5,
                    borderRadius: '6px',
                    fontWeight: 700,
                    fontSize: 15,
                    py: 1.25,
                    ...(e.primary
                      ? { bgcolor: LP.primary, '&:hover': { bgcolor: '#005B8E' } }
                      : { borderColor: LP.rule, color: LP.ink }),
                  }}
                >
                  {e.action}
                </Button>
              </Box>
            ))}
          </Box>
        </Container>
      </Box>

      {/* ── FAQ ────────────────────────────────── */}
      <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 12 } }}>
        <SectionHead label="FAQ" title="よくあるご質問" />
        {/* 読み幅は絞るが、左端は他セクションと揃える。Container を細くすると
            中央寄せになって、ISSUE / SOLUTION と縦のラインがずれる。 */}
        <Box sx={{ maxWidth: 760, borderTop: `1px solid ${LP.rule}` }}>
          {FAQS.map((f) => (
            <Box key={f.q} sx={{ borderBottom: `1px solid ${LP.rule}`, py: 3 }}>
              <Typography
                component="h3"
                sx={{ fontSize: { xs: 16, md: 17 }, fontWeight: 700, lineHeight: 1.6 }}
              >
                {f.q}
              </Typography>
              <Typography sx={{ mt: 1.25, fontSize: 14, lineHeight: 2, color: LP.muted }}>
                {f.a}
              </Typography>
            </Box>
          ))}
        </Box>
      </Container>

      {/* ── クロージング ───────────────────────────── */}
      <Box sx={{ bgcolor: LP.ink, color: LP.paper }}>
        <Container
          maxWidth="md"
          sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 11 }, textAlign: 'center' }}
        >
          <Typography
            component="p"
            sx={{ fontSize: { xs: 22, md: 30 }, fontWeight: 700, lineHeight: 1.55 }}
          >
            登録して、最初の質問に答えるところから。
          </Typography>
          <Typography sx={{ mt: 2, fontSize: 14, color: 'rgba(255,255,255,.72)', lineHeight: 2 }}>
            在校生の利用は学校の導入に含まれます。学生個人への課金はありません。
          </Typography>
          <Button
            href="/login"
            disableElevation
            variant="contained"
            sx={{
              mt: 4,
              bgcolor: LP.paper,
              color: LP.ink,
              borderRadius: '6px',
              fontSize: 16,
              fontWeight: 700,
              px: 5,
              py: 1.5,
              '&:hover': { bgcolor: 'rgba(255,255,255,.88)' },
            }}
          >
            無料で診断を始める
          </Button>
        </Container>
      </Box>

      <Box component="footer" sx={{ borderTop: `1px solid ${LP.rule}` }}>
        <Container
          maxWidth="lg"
          sx={{
            px: { xs: 2.5, md: 4 },
            py: 3,
            display: 'flex',
            flexWrap: 'wrap',
            gap: 2.5,
            alignItems: 'baseline',
          }}
        >
          <Typography sx={{ fontSize: 13, color: LP.muted }}>© 就活AI</Typography>
          <Typography
            component="a"
            href="/privacy"
            sx={{ fontSize: 13, color: LP.ink, textUnderlineOffset: '3px' }}
          >
            プライバシーポリシー
          </Typography>
        </Container>
      </Box>
    </Box>
  )
}
