import { Box, Button, Container, Typography } from '@mui/material'
import {
  ArrowRight,
  Building2,
  ClipboardCheck,
  GraduationCap,
  Mic,
  Search,
  Sparkles,
  UserRound,
} from 'lucide-react'
import { LandingHeader } from './LandingHeader'
import { LandingVisual } from './LandingVisual'
import { LandingFooter } from './LandingFooter'
import { LP, RISE, delay } from './tokens'

/**
 * 未ログインの訪問者に出す公開ランディングページ（#1653）。
 *
 * 以前は `/` が認証必須で、未ログインは即 `/login` へ飛ばされていた。
 * 製品の説明は `/login` の1行だけで、展示会のQRから来た来場者は
 * いきなりログインフォームを見ることになっていた。
 *
 * **このコンポーネントはAPIを呼ばない。** 本番は展示会運用で `desired=0` から
 * 起動するため、Backend の起動を待たずに表示できる必要がある（受け入れ条件6）。
 * Server Component のまま保つこと。モーションもCSSだけで出している。
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
 *
 * ## 見た目の設計
 *
 * 地の色で段を作る: 濃色(FV) → 白(課題) → 薄青(解決策) → 白(入口) → 白(FAQ) → 濃色(締め)。
 * 全セクションを同じ白地に並べると、構成が正しくても平坦に見える。
 *
 * 大胆さは1箇所に集める。濃色のFVがそれで、以降は落ち着かせる。
 * 強調の橙は番号とラベルに点で置き、面では塗らない。
 *
 * ## 数字について
 *
 * **持っていない数字は書かない。** 導入校数・内定率・満足度は計測していないので
 * 載せない。載せるのは製品の事実（評価観点5つ、スコアの構成、掲載企業数）だけ。
 * 導入事例もまだ無いのでセクション自体を置いていない。
 */

/** 就活で実際に詰まるところ。機能から逆算せず、利用者の言葉で書く。 */
const PROBLEMS = [
  {
    who: '学生',
    quote: '何から始めればいいか\n分からない',
    body: '自己分析のやり方が分からないまま、とりあえず求人サイトを眺めて終わってしまう。',
  },
  {
    who: '学生',
    quote: '面接の練習相手が\nいない',
    body: '先生の時間は限られていて、本番までに数回しか練習できない。何が悪かったのかも分からない。',
  },
  {
    who: '先生',
    quote: '一人ひとりの状況を\n把握しきれない',
    body: '誰が止まっているのか、どこで止まっているのかが、面談するまで見えない。',
  },
] as const

/** 課題への対応。機能名の羅列にせず「どの課題にどう効くか」で並べる。 */
const SOLUTIONS = [
  {
    n: '01',
    icon: Sparkles,
    title: 'AIと話すだけで、自己分析が進む',
    forWhom: '何から始めればいいか分からない',
    body: 'チャットで答えていくと、強みと志向が10カテゴリのスコアになります。就活の進み方に合わせて4フェーズで記録するので、自分がどう変わったかも残ります。',
  },
  {
    n: '02',
    icon: Search,
    title: 'スコアに合う企業が、根拠つきで出る',
    forWhom: '求人サイトを眺めて終わってしまう',
    body: '診断結果と、企業側が求める人物像を突き合わせて候補を出します。なぜ合うのかが見えるので、志望動機にそのまま使えます。',
  },
  {
    n: '03',
    icon: Mic,
    title: '面接は何度でも、音声で練習できる',
    forWhom: '面接の練習相手がいない',
    body: '論理性・具体性・主体性・コミュニケーション力・積極性の5つの観点で講評します。根拠は実際の発言から引用するので、どこを直すかが分かります。',
  },
  {
    n: '04',
    icon: ClipboardCheck,
    title: '書類は添削まで、ひと続きで',
    forWhom: 'ESが書けない',
    body: '職務経歴書のレビューと、ESの添削・リライト。設問の文字数上限に合わせて整えます。',
  },
] as const

/** 製品の事実だけ。計測していない指標は載せない。 */
const FACTS = [
  { v: '4万社', k: 'から企業を選定' },
  { v: '10', k: 'カテゴリ × 4フェーズでスコア化' },
  { v: '5', k: '観点で面接を講評' },
] as const

/** 振り分け先。未ログインで到達する3系統に対応する。 */
const ENTRANCES = [
  {
    label: '学生',
    icon: UserRound,
    target: '専門学校で就職活動をしている方',
    action: '無料で診断を始める',
    href: '/login',
    primary: true,
  },
  {
    label: '企業',
    icon: Building2,
    target: '学生の採用を検討している企業の採用担当者',
    action: '企業ポータルへ',
    href: '/company-portal/sign-in',
    primary: false,
  },
  {
    label: '学校・教員',
    icon: GraduationCap,
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

/** セクションのラベル。短い横線を伴わせて、見出しの立ち上がりを作る。 */
function Eyebrow({ children, light }: { children: string; light?: boolean }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
      <Box sx={{ width: 28, height: 2, bgcolor: LP.accent }} />
      <Typography
        component="p"
        sx={{
          fontSize: 11.5,
          fontWeight: 700,
          letterSpacing: '.22em',
          color: light ? LP.accent : LP.primary,
        }}
      >
        {children}
      </Typography>
    </Box>
  )
}

function SectionHead({ label, title, light }: { label: string; title: string; light?: boolean }) {
  return (
    <Box sx={{ mb: { xs: 4, md: 6 }, ...RISE }}>
      <Eyebrow light={light}>{label}</Eyebrow>
      <Typography
        component="h2"
        sx={{
          mt: 2,
          fontSize: { xs: 26, md: 38 },
          fontWeight: 700,
          lineHeight: 1.4,
          letterSpacing: '-0.015em',
          color: light ? LP.paper : LP.ink,
        }}
      >
        {title}
      </Typography>
    </Box>
  )
}

/** 主CTA。FV・入口・締めで見た目を揃える。 */
const ctaSx = (variant: 'onDark' | 'onLight') => {
  const skin =
    variant === 'onDark'
      ? { bgcolor: LP.paper, color: LP.ink, shadow: '0 10px 26px rgba(0,0,0,.30)', hoverBg: 'rgba(255,255,255,.9)' }
      : { bgcolor: LP.primary, color: LP.paper, shadow: '0 8px 20px rgba(0,114,178,.28)', hoverBg: LP.primaryHover }
  return {
    borderRadius: '8px',
    fontSize: 16,
    fontWeight: 700,
    px: 4,
    py: 1.75,
    bgcolor: skin.bgcolor,
    color: skin.color,
    boxShadow: skin.shadow,
    transition: 'transform .2s, box-shadow .2s, background-color .2s',
    // hover と reduced-motion はここで1回だけ定義する。
    // 共通側と variant 側の両方に書くと後勝ちで静かに上書きされる（TS2783）。
    '&:hover': { bgcolor: skin.hoverBg, transform: 'translateY(-2px)' },
    '@media (prefers-reduced-motion: reduce)': {
      transition: 'none',
      '&:hover': { transform: 'none' },
    },
  }
}

export function LandingContent() {
  return (
    <Box sx={{ bgcolor: LP.paper, color: LP.ink }}>
      <LandingHeader />
      <Box component="main" id="top">
      {/* ── ファーストビュー（濃色。大胆さはここに集める） ───────── */}
      <Box sx={{ position: 'relative', bgcolor: LP.ink, color: LP.paper, overflow: 'hidden' }}>
        {/* 奥行きのための光。面で塗らず、滲みで出す。 */}
        <Box
          aria-hidden
          sx={{
            position: 'absolute',
            inset: 0,
            background: `radial-gradient(70% 55% at 78% 8%, rgba(86,180,233,.20), transparent 68%),
                         radial-gradient(50% 45% at 8% 92%, rgba(230,159,0,.12), transparent 70%)`,
          }}
        />
        <Container
          maxWidth="lg"
          sx={{ position: 'relative', px: { xs: 2.5, md: 4 }, py: { xs: 7, md: 13 } }}
        >
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: '1.1fr 0.9fr' },
              gap: { xs: 6, md: 9 },
              alignItems: 'center',
            }}
          >
            <Box>
              <Box sx={{ ...RISE }}>
                <Eyebrow light>FOR VOCATIONAL SCHOOLS</Eyebrow>
              </Box>

              <Typography
                component="h1"
                sx={{
                  mt: 2.5,
                  fontSize: { xs: 34, sm: 48, md: 60 },
                  fontWeight: 700,
                  lineHeight: 1.26,
                  letterSpacing: '-0.025em',
                  ...RISE,
                  ...delay(1),
                }}
              >
                何から始めればいいか
                <br />
                分からない就活を、
                <br />
                <Box
                  component="span"
                  sx={{
                    // 強調は面で塗らず下に引く。マーカーは行をまたぐと途切れるので
                    // 必ず単独行に置く。
                    display: 'inline-block',
                    backgroundImage: `linear-gradient(${LP.accent}, ${LP.accent})`,
                    backgroundSize: '100% 0.3em',
                    backgroundPosition: '0 86%',
                    backgroundRepeat: 'no-repeat',
                  }}
                >
                  最初の一歩から。
                </Box>
              </Typography>

              <Typography
                sx={{
                  mt: 3.5,
                  fontSize: { xs: 15, md: 17 },
                  lineHeight: 2.1,
                  color: 'rgba(255,255,255,.80)',
                  maxWidth: '30em',
                  ...RISE,
                  ...delay(2),
                }}
              >
                AIとの対話で適性を診断し、企業とのマッチング、面接練習、履歴書・ES添削まで。
                就職活動をひと続きで進められます。
              </Typography>

              <Box
                sx={{
                  mt: 4.5,
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: 2,
                  alignItems: 'center',
                  ...RISE,
                  ...delay(3),
                }}
              >
                <Button
                  href="/login"
                  disableElevation
                  variant="contained"
                  endIcon={<ArrowRight size={18} strokeWidth={2.4} />}
                  sx={ctaSx('onDark')}
                >
                  無料で診断を始める
                </Button>
                <Button
                  href="#entrances"
                  endIcon={<ArrowRight size={16} strokeWidth={2.4} />}
                  sx={{
                    fontSize: 15,
                    fontWeight: 700,
                    color: 'rgba(255,255,255,.88)',
                    borderRadius: '8px',
                    px: 2,
                    py: 1.5,
                  }}
                >
                  企業・学校の方はこちら
                </Button>
              </Box>

              {/* 権威づけの代わりに製品の事実を置く。実績値は持っていないので使わない。 */}
              <Box
                sx={{
                  mt: 6,
                  pt: 3.5,
                  borderTop: '1px solid rgba(255,255,255,.14)',
                  display: 'grid',
                  gridTemplateColumns: { xs: 'repeat(3, auto)', sm: 'repeat(3, auto)' },
                  justifyContent: 'start',
                  gap: { xs: 3, sm: 6 },
                  ...RISE,
                  ...delay(4),
                }}
              >
                {FACTS.map((f) => (
                  <Box key={f.k}>
                    <Typography
                      sx={{
                        fontSize: { xs: 24, md: 30 },
                        fontWeight: 700,
                        lineHeight: 1.1,
                        letterSpacing: '-0.02em',
                        fontVariantNumeric: 'tabular-nums',
                      }}
                    >
                      {f.v}
                    </Typography>
                    <Typography
                      sx={{
                        fontSize: 11.5,
                        color: 'rgba(255,255,255,.62)',
                        mt: 1,
                        lineHeight: 1.6,
                        maxWidth: '11em',
                      }}
                    >
                      {f.k}
                    </Typography>
                  </Box>
                ))}
              </Box>
            </Box>

            <LandingVisual />
          </Box>
        </Container>
      </Box>

      {/* ── 課題提起 ─────────────────────────────── */}
      <Container id="issue" maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 8, md: 14 } }}>
        <SectionHead label="ISSUE" title="就活は、つまずく場所が決まっています。" />
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
            gap: { xs: 4, md: 5 },
          }}
        >
          {PROBLEMS.map((p, i) => (
            <Box key={p.body} sx={{ ...RISE, ...delay(i) }}>
              <Typography
                aria-hidden
                sx={{
                  fontSize: 56,
                  fontWeight: 700,
                  lineHeight: 0.8,
                  color: LP.rule,
                  fontFamily: 'Georgia, serif',
                }}
              >
                “
              </Typography>
              <Typography
                sx={{
                  mt: 1,
                  fontSize: { xs: 19, md: 21 },
                  fontWeight: 700,
                  lineHeight: 1.65,
                  whiteSpace: 'pre-line',
                }}
              >
                {p.quote}
              </Typography>
              <Typography
                sx={{ mt: 2, fontSize: 11.5, fontWeight: 700, color: LP.primary, letterSpacing: '.1em' }}
              >
                {p.who}
              </Typography>
              <Typography
                sx={{
                  mt: 2,
                  pt: 2.5,
                  borderTop: `1px solid ${LP.rule}`,
                  fontSize: 14,
                  lineHeight: 2,
                  color: LP.muted,
                }}
              >
                {p.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </Container>

      {/* ── 解決策（薄青。番号を大きく立てる） ─────────────── */}
      <Box id="solution" sx={{ bgcolor: LP.tint, borderTop: `1px solid ${LP.rule}` }}>
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 8, md: 14 } }}>
          <SectionHead label="SOLUTION" title="つまずく場所に、ひとつずつ手を当てます。" />
          <Box sx={{ display: 'grid', gap: { xs: 2.5, md: 3 } }}>
            {SOLUTIONS.map((s, i) => (
              <Box
                key={s.n}
                sx={{
                  bgcolor: LP.paper,
                  borderRadius: '14px',
                  border: `1px solid ${LP.rule}`,
                  p: { xs: 3, md: 4 },
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', md: '5rem 1.05fr 1fr' },
                  gap: { xs: 2, md: 4 },
                  alignItems: 'start',
                  transition: 'box-shadow .25s, transform .25s',
                  '&:hover': {
                    boxShadow: '0 14px 34px rgba(12,22,32,.09)',
                    transform: 'translateY(-2px)',
                  },
                  ...RISE,
                  ...delay(i),
                  '@media (prefers-reduced-motion: reduce)': {
                    animation: 'none',
                    transition: 'none',
                    '&:hover': { transform: 'none' },
                  },
                }}
              >
                <Box aria-hidden sx={{ display: 'grid', gap: 1.5, justifyItems: 'start' }}>
                  <Box
                    sx={{
                      width: 44,
                      height: 44,
                      borderRadius: '11px',
                      display: 'grid',
                      placeItems: 'center',
                      color: LP.primary,
                      bgcolor: LP.tint,
                    }}
                  >
                    <s.icon size={21} strokeWidth={2} />
                  </Box>
                  <Typography
                    sx={{
                      fontSize: { xs: 26, md: 32 },
                      fontWeight: 700,
                      lineHeight: 0.95,
                      color: LP.accent,
                      letterSpacing: '-0.03em',
                      fontVariantNumeric: 'tabular-nums',
                    }}
                  >
                    {s.n}
                  </Typography>
                </Box>
                <Box>
                  <Typography
                    component="h3"
                    sx={{ fontSize: { xs: 19, md: 22 }, fontWeight: 700, lineHeight: 1.55 }}
                  >
                    {s.title}
                  </Typography>
                  <Typography
                    sx={{
                      mt: 1.5,
                      display: 'inline-block',
                      fontSize: 11.5,
                      fontWeight: 700,
                      color: LP.primary,
                      bgcolor: LP.tint,
                      borderRadius: 99,
                      px: 1.5,
                      py: 0.5,
                    }}
                  >
                    「{s.forWhom}」に効きます
                  </Typography>
                </Box>
                <Typography sx={{ fontSize: 14, lineHeight: 2.05, color: LP.muted }}>
                  {s.body}
                </Typography>
              </Box>
            ))}
          </Box>
        </Container>
      </Box>

      {/* ── 3者の入口 ────────────────────────────── */}
      <Box id="entrances">
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 8, md: 14 } }}>
          <SectionHead label="ENTRANCE" title="ご利用の方を選んでください。" />
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 3,
            }}
          >
            {ENTRANCES.map((e, i) => (
              <Box
                key={e.href}
                sx={{
                  position: 'relative',
                  borderRadius: '14px',
                  p: { xs: 3, md: 3.5 },
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 1.25,
                  transition: 'box-shadow .25s, transform .25s',
                  ...(e.primary
                    ? {
                        bgcolor: LP.ink,
                        color: LP.paper,
                        boxShadow: '0 16px 36px rgba(12,22,32,.22)',
                      }
                    : { bgcolor: LP.paper, border: `1px solid ${LP.rule}` }),
                  ...RISE,
                  ...delay(i),
                  // hover と reduced-motion は最後に1回だけ。variant 側にも書くと
                  // 後勝ちで静かに上書きされる（TS2783）。
                  '&:hover': {
                    transform: 'translateY(-3px)',
                    boxShadow: e.primary
                      ? '0 22px 46px rgba(12,22,32,.28)'
                      : '0 14px 30px rgba(12,22,32,.10)',
                  },
                  '@media (prefers-reduced-motion: reduce)': {
                    animation: 'none',
                    transition: 'none',
                    '&:hover': { transform: 'none' },
                  },
                }}
              >
                {e.primary && (
                  <Typography
                    sx={{
                      fontSize: 10.5,
                      fontWeight: 700,
                      letterSpacing: '.16em',
                      color: LP.accent,
                    }}
                  >
                    おすすめ
                  </Typography>
                )}
                <Box
                  aria-hidden
                  sx={{
                    width: 46,
                    height: 46,
                    borderRadius: '12px',
                    display: 'grid',
                    placeItems: 'center',
                    mb: 0.5,
                    ...(e.primary
                      ? { bgcolor: 'rgba(255,255,255,.12)', color: LP.accent }
                      : { bgcolor: LP.tint, color: LP.primary }),
                  }}
                >
                  <e.icon size={22} strokeWidth={2} />
                </Box>
                <Typography component="h3" sx={{ fontSize: 22, fontWeight: 700 }}>
                  {e.label}
                </Typography>
                <Typography
                  sx={{
                    fontSize: 13,
                    lineHeight: 1.9,
                    flexGrow: 1,
                    color: e.primary ? 'rgba(255,255,255,.74)' : LP.muted,
                  }}
                >
                  {e.target}
                </Typography>
                <Button
                  href={e.href}
                  disableElevation
                  variant={e.primary ? 'contained' : 'outlined'}
                  sx={{
                    mt: 1.5,
                    borderRadius: '8px',
                    fontWeight: 700,
                    fontSize: 15,
                    py: 1.4,
                    ...(e.primary
                      ? { bgcolor: LP.paper, color: LP.ink, '&:hover': { bgcolor: 'rgba(255,255,255,.9)' } }
                      : { borderColor: LP.rule, color: LP.ink, '&:hover': { borderColor: LP.ink } }),
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
      <Box id="faq" sx={{ bgcolor: LP.tint, borderTop: `1px solid ${LP.rule}` }}>
        <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 4 }, py: { xs: 8, md: 14 } }}>
          <SectionHead label="FAQ" title="よくあるご質問" />
          {/* 読み幅は絞るが、左端は他セクションと揃える。Container を細くすると
              中央寄せになって縦のラインがずれる。 */}
          <Box sx={{ maxWidth: 820, display: 'grid', gap: 2 }}>
            {FAQS.map((f, i) => (
              <Box
                key={f.q}
                sx={{
                  bgcolor: LP.paper,
                  border: `1px solid ${LP.rule}`,
                  borderRadius: '12px',
                  px: { xs: 2.5, md: 3.5 },
                  py: { xs: 2.5, md: 3 },
                  ...RISE,
                  ...delay(i),
                }}
              >
                <Typography
                  component="h3"
                  sx={{ fontSize: { xs: 16, md: 17 }, fontWeight: 700, lineHeight: 1.65 }}
                >
                  {f.q}
                </Typography>
                <Typography sx={{ mt: 1.25, fontSize: 14, lineHeight: 2.05, color: LP.muted }}>
                  {f.a}
                </Typography>
              </Box>
            ))}
          </Box>
        </Container>
      </Box>

      {/* ── クロージング ───────────────────────────── */}
      <Box sx={{ position: 'relative', bgcolor: LP.ink, color: LP.paper, overflow: 'hidden' }}>
        <Box
          aria-hidden
          sx={{
            position: 'absolute',
            inset: 0,
            background: `radial-gradient(60% 70% at 50% 0%, rgba(86,180,233,.18), transparent 70%)`,
          }}
        />
        <Container
          maxWidth="md"
          sx={{
            position: 'relative',
            px: { xs: 2.5, md: 4 },
            py: { xs: 9, md: 14 },
            textAlign: 'center',
          }}
        >
          <Typography
            component="p"
            sx={{
              fontSize: { xs: 24, md: 36 },
              fontWeight: 700,
              lineHeight: 1.5,
              letterSpacing: '-0.02em',
            }}
          >
            登録して、最初の質問に
            <Box component="br" sx={{ display: { md: 'none' } }} />
            答えるところから。
          </Typography>
          <Typography
            sx={{ mt: 2.5, fontSize: 14, color: 'rgba(255,255,255,.70)', lineHeight: 2 }}
          >
            在校生の利用は学校の導入に含まれます。学生個人への課金はありません。
          </Typography>
          <Button
            href="/login"
            disableElevation
            variant="contained"
            sx={{ mt: 4.5, ...ctaSx('onDark'), px: 5.5 }}
          >
            無料で診断を始める
          </Button>
        </Container>
      </Box>

      </Box>
      <LandingFooter />
    </Box>
  )
}
