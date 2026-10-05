import { Box, Button, Container, Typography } from '@mui/material'
import { LandingHeader } from './LandingHeader'
import { LandingVisual } from './LandingVisual'
import { LandingWalkthrough } from './LandingWalkthrough'
import { LandingFooter } from './LandingFooter'
import { LP, GENKO_GRID, RISE, NO_MOTION, delay } from './tokens'

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
 *   FV → 課題提起 → 解決策（課題と対応させる） → 3者の入口 → FAQ → クロージング
 *
 * ## 見た目
 *
 * SaaS の LP で最も使い回されている形を避ける。具体的には使わないもの:
 * 濃紺の地＋青い放射グラデの発光／同じ角丸・同じ影の白カードの反復／
 * ALL-CAPS の英字ラベル（ISSUE / SOLUTION）／角丸四角に入れたアイコン／
 * ブラウザのクローム付きモックアップ。どれも題材を説明しない。
 *
 * 代わりに題材の語彙を使う。ES の原稿用紙のマス目を地紋にし、節は漢数字で送り、
 * 見出しのラベルは縦組みで置く（md 以上）。日本の就職活動にしか無い形で、
 * 汎用テンプレートからは出てこない。
 *
 * 角丸は使わない。罫と余白で構造を作る。色は墨・紙・朱＋操作の青だけ。
 *
 * ## 数字について
 *
 * **持っていない数字は書かない。** 導入校数・内定率・満足度は計測していないので
 * 載せない。導入事例もまだ無いのでセクション自体を置いていない。
 */

const PROBLEMS = [
  {
    who: '学生',
    quote: '自己PRに何を書けばいいか分からない',
    body: '授業の課題もアルバイトもやってきたのに、いざ書こうとすると「普通のことしかしていない」と思えてくる。',
  },
  {
    who: '学生',
    quote: '面接で答えがまとまらない',
    body: '練習相手がいない。先生に見てもらえるのは数回で、何が悪かったのかも分からないまま本番が来る。',
  },
  {
    who: '先生',
    quote: '誰が止まっているか、面談するまで分からない',
    body: '受け持ちの学生全員の進み具合を追いきれない。気づいたときには応募が止まっていることがある。',
  },
] as const

/** 課題への対応。機能名の羅列にせず「どの課題にどう効くか」で並べる。 */
const SOLUTIONS = [
  {
    n: '一',
    title: '授業やアルバイトの経験から、自分の強みを整理する',
    forWhom: '自己PRに何を書けばいいか分からない',
    body: 'AIとの対話で、やってきたことを掘り下げます。出てくるのは「協調性がある」のような言葉ではなく、どの経験がどの強みの根拠になるかという並びです。自己PRにそのまま使えます。',
  },
  {
    n: '二',
    title: '制作課題のコードから、技術の実績を出す',
    forWhom: '実務経験がないから書くことがない',
    body: 'GitHub をつなぐと、使ってきた言語とリポジトリから Frontend / Backend / Infrastructure / Database のスキルが出ます。授業や個人制作が、そのまま応募の材料になります。',
  },
  {
    n: '三',
    title: '合う企業が、合う理由つきで出る',
    forWhom: '求人サイトを眺めて終わってしまう',
    body: '整理した強みと、企業が求める人物像を突き合わせます。「なぜ合うのか」が文章で付くので、志望動機を書くときの材料になります。',
  },
  {
    n: '四',
    title: 'ESは、設問の文字数に収まる形まで直す',
    forWhom: '書いたものが長すぎる・薄い',
    body: '職務経歴書のレビューと、ESの添削・リライト。設問ごとの文字数上限に合わせて整えます。',
  },
] as const

/** 製品の事実だけ。計測していない指標は載せない。 */
/**
 * 製品の事実だけ。**計測していない指標は載せない。**
 * 以前は「4万社から選定」と書いていたが、その件数の根拠がコード上に無かったので外した。
 * ここに書いてよいのは、実装を読めば確かめられることだけ。
 */
const FACTS = [
  {
    v: '5',
    unit: '観点',
    k: '論理性・具体性・主体性・コミュニケーション力・積極性で面接を講評',
  },
  {
    v: '4',
    unit: '分野',
    k: 'GitHub から Frontend / Backend / Infrastructure / Database のスキルを算出',
  },
  {
    v: '0',
    unit: '件',
    k: '実際の発言と照合できない指摘は表示しない',
  },
] as const

/**
 * 専門学校・大学での使われ方。他の就活サービスに置き換えられない部分を書く。
 * いずれも実装にある挙動（GitHubスキルスコア / 教員の就活状況確認 /
 * 企業の掲載審査）に対応させること。無い機能を書かない。
 */
const SCHOOL_TIES = [
  {
    title: '制作課題が実績になる',
    body: 'GitHub をつなぐと、授業や個人制作で使ってきた言語とリポジトリから技術スキルが出ます。アルバイト以外に書くことがない、という状態になりません。',
  },
  {
    title: '先生が、止まっている人に気づける',
    body: '先生は受け持ちの学生の就活状況を一覧で見られます。応募が止まっている人、面接が近い人が分かるので、面談の前に声をかけられます。',
  },
  {
    title: '載っている企業は、学校が通したものだけ',
    body: '企業が求人を出すには学校側の審査を通る必要があります。学校が把握していない求人は学生に表示されません。',
  },
] as const

const ENTRANCES = [
  {
    label: '学生',
    target: '専門学校・大学で就職活動をしている方',
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

/**
 * 節の見出し。ラベルは md 以上で縦組みにして左の柱に立てる。
 * 英字の ALL-CAPS ラベルは使わない（題材を説明しないうえ、生成物に見える）。
 */
function Section({
  id,
  label,
  title,
  children,
  tinted,
}: {
  id: string
  label: string
  title: string
  children: React.ReactNode
  tinted?: boolean
}) {
  return (
    <Box
      component="section"
      id={id}
      sx={{
        bgcolor: tinted ? LP.card : 'transparent',
        borderTop: `1px solid ${LP.rule}`,
      }}
    >
      <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 5 }, py: { xs: 7, md: 12 } }}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', md: '7.5rem 1fr' },
            gap: { xs: 3, md: 5 },
          }}
        >
          <Box sx={{ ...RISE, ...NO_MOTION }}>
            <Typography
              component="p"
              sx={{
                fontSize: 13,
                fontWeight: 700,
                color: LP.ink,
                letterSpacing: '.3em',
                // 縦組み。md 未満では横に倒す（狭い画面で縦組みは読みにくい）。
                writingMode: { xs: 'horizontal-tb', md: 'vertical-rl' },
                borderLeft: { md: `2px solid ${LP.seal}` },
                borderBottom: { xs: `2px solid ${LP.seal}`, md: 'none' },
                pl: { md: 1.5 },
                pb: { xs: 1, md: 0 },
                display: 'inline-block',
              }}
            >
              {label}
            </Typography>
          </Box>

          <Box sx={{ minWidth: 0 }}>
            <Typography
              component="h2"
              sx={{
                fontSize: { xs: 24, md: 34 },
                fontWeight: 700,
                lineHeight: 1.5,
                letterSpacing: '.01em',
                mb: { xs: 4, md: 6 },
                ...RISE,
                ...NO_MOTION,
              }}
            >
              {title}
            </Typography>
            {children}
          </Box>
        </Box>
      </Container>
    </Box>
  )
}

/** 主操作。角丸を使わない。 */
const ctaSx = {
  borderRadius: 0,
  bgcolor: LP.primary,
  color: LP.card,
  fontSize: 16,
  fontWeight: 700,
  px: 4.5,
  py: 1.75,
  letterSpacing: '.04em',
  boxShadow: `4px 4px 0 ${LP.ink}`,
  transition: 'transform .18s, box-shadow .18s, background-color .18s',
  '&:hover': {
    bgcolor: LP.primaryHover,
    transform: 'translate(2px, 2px)',
    boxShadow: `2px 2px 0 ${LP.ink}`,
  },
  '@media (prefers-reduced-motion: reduce)': {
    transition: 'none',
    '&:hover': { transform: 'none' },
  },
} as const

export function LandingContent() {
  return (
    <Box sx={{ bgcolor: LP.paper, color: LP.ink }}>
      <LandingHeader />

      <Box component="main" id="top">
        {/* ── ファーストビュー。原稿用紙の地紋を敷く ───────────── */}
        <Box sx={{ position: 'relative', ...GENKO_GRID(28, LP.ruleSoft) }}>
          <Container maxWidth="lg" sx={{ px: { xs: 2.5, md: 5 }, py: { xs: 7, md: 12 } }}>
            <Box
              sx={{
                display: 'grid',
                gridTemplateColumns: { xs: '1fr', md: '1.08fr 0.92fr' },
                gap: { xs: 6, md: 8 },
                alignItems: 'center',
              }}
            >
              <Box>
                <Typography
                  component="p"
                  sx={{
                    display: 'inline-block',
                    fontSize: 12,
                    fontWeight: 700,
                    letterSpacing: '.14em',
                    color: LP.card,
                    bgcolor: LP.seal,
                    px: 1.5,
                    py: 0.5,
                    ...RISE,
                    ...NO_MOTION,
                  }}
                >
                  専門学校・大学の学生のための就活AI
                </Typography>

                <Typography
                  component="h1"
                  sx={{
                    mt: 3,
                    fontSize: { xs: 33, sm: 44, md: 56 },
                    fontWeight: 700,
                    lineHeight: 1.42,
                    letterSpacing: '.01em',
                    ...RISE,
                    ...delay(1),
                    ...NO_MOTION,
                  }}
                >
                  何から始めればいいか
                  <br />
                  分からない就活を、
                  <br />
                  <Box
                    component="span"
                    sx={{
                      // 朱の傍線。日本語の強調は面で塗らず、脇に線を引く。
                      display: 'inline-block',
                      borderBottom: `4px solid ${LP.seal}`,
                      pb: 0.5,
                    }}
                  >
                    最初の一歩から。
                  </Box>
                </Typography>

                <Typography
                  sx={{
                    mt: 4,
                    fontSize: { xs: 15, md: 16.5 },
                    lineHeight: 2.2,
                    color: LP.inkSoft,
                    maxWidth: '30em',
                    ...RISE,
                    ...delay(2),
                    ...NO_MOTION,
                  }}
                >
                  授業やアルバイト、制作課題の経験から、自己PRに書ける強みを整理します。
                  面接は音声で何度でも練習でき、直すところが自分の言葉で返ってきます。
                </Typography>

                <Box
                  sx={{
                    mt: 4.5,
                    display: 'flex',
                    flexWrap: 'wrap',
                    gap: 3,
                    alignItems: 'center',
                    ...RISE,
                    ...delay(3),
                    ...NO_MOTION,
                  }}
                >
                  <Button href="/login" disableElevation variant="contained" sx={ctaSx}>
                    無料で診断を始める
                  </Button>
                  <Box
                    component="a"
                    href="#entrances"
                    sx={{
                      fontSize: 14,
                      fontWeight: 700,
                      color: LP.ink,
                      textDecoration: 'none',
                      borderBottom: `1px solid ${LP.ink}`,
                      pb: 0.25,
                    }}
                  >
                    企業・学校の方はこちら
                  </Box>
                </Box>

                {/* 権威づけの代わりに製品の事実を置く。実績値は持っていないので使わない。 */}
                <Box
                  sx={{
                    mt: 6,
                    display: 'grid',
                    gridTemplateColumns: 'repeat(3, auto)',
                    justifyContent: 'start',
                    gap: { xs: 3, sm: 5 },
                    ...RISE,
                    ...delay(4),
                    ...NO_MOTION,
                  }}
                >
                  {FACTS.map((f) => (
                    <Box key={f.k} sx={{ borderTop: `2px solid ${LP.ink}`, pt: 1.5 }}>
                      <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 0.25 }}>
                        <Typography
                          sx={{
                            fontSize: { xs: 26, md: 32 },
                            fontWeight: 700,
                            lineHeight: 1,
                            fontVariantNumeric: 'tabular-nums',
                          }}
                        >
                          {f.v}
                        </Typography>
                        <Typography sx={{ fontSize: 12, fontWeight: 700 }}>{f.unit}</Typography>
                      </Box>
                      <Typography
                        sx={{ fontSize: 11, color: LP.muted, mt: 1, maxWidth: '10em', lineHeight: 1.7 }}
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

        {/* ── 課題提起 ──────────────────────────── */}
        <Section id="issue" label="課題" title="就活でつまずくのは、だいたい同じところです。">
          <Box sx={{ borderTop: `2px solid ${LP.ink}` }}>
            {PROBLEMS.map((p, i) => (
              <Box
                key={p.body}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', md: '4rem 1fr 1fr' },
                  gap: { xs: 1.5, md: 4 },
                  py: { xs: 3, md: 3.5 },
                  borderBottom: `1px solid ${LP.rule}`,
                  alignItems: 'baseline',
                  ...RISE,
                  ...delay(i),
                  ...NO_MOTION,
                }}
              >
                <Typography
                  sx={{
                    fontSize: 12,
                    fontWeight: 700,
                    color: LP.seal,
                    letterSpacing: '.1em',
                  }}
                >
                  {p.who}
                </Typography>
                <Typography
                  sx={{ fontSize: { xs: 18, md: 20 }, fontWeight: 700, lineHeight: 1.7 }}
                >
                  「{p.quote}」
                </Typography>
                <Typography sx={{ fontSize: 13.5, lineHeight: 2.1, color: LP.muted }}>
                  {p.body}
                </Typography>
              </Box>
            ))}
          </Box>
        </Section>

        {/* ── 使ってみる。ここが一番大きい（情報の強弱） ──────── */}
        <Section
          id="walkthrough"
          tinted
          label="使ってみる"
          title="面接の前日に、答えが出てこないとき。"
        >
          <LandingWalkthrough />
        </Section>

        {/* ── 専門学校・大学ならでは ───────────────── */}
        <Section
          id="school"
          label="学校との関わり"
          title="授業も、制作課題も、就職の材料になります。"
        >
          <Box sx={{ borderTop: `2px solid ${LP.ink}` }}>
            {SCHOOL_TIES.map((t, i) => (
              <Box
                key={t.title}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '1fr', md: '12rem 1fr' },
                  gap: { xs: 1, md: 4 },
                  py: { xs: 3, md: 3.5 },
                  borderBottom: `1px solid ${LP.rule}`,
                  ...RISE,
                  ...delay(i),
                  ...NO_MOTION,
                }}
              >
                <Typography sx={{ fontSize: 15, fontWeight: 700, lineHeight: 1.7 }}>
                  {t.title}
                </Typography>
                <Typography sx={{ fontSize: 13.5, lineHeight: 2.15, color: LP.muted }}>
                  {t.body}
                </Typography>
              </Box>
            ))}
          </Box>
        </Section>

        {/* ── 解決策。節は漢数字で送る ────────────────── */}
        <Section id="solution" tinted label="できること" title="自己PRが書けないときも、面接で答えがまとまらないときも。">
          <Box sx={{ borderTop: `2px solid ${LP.ink}` }}>
            {SOLUTIONS.map((s, i) => (
              <Box
                key={s.n}
                sx={{
                  display: 'grid',
                  gridTemplateColumns: { xs: '2.5rem 1fr', md: '3.5rem 1.05fr 1fr' },
                  columnGap: { xs: 2, md: 4 },
                  rowGap: 1.5,
                  py: { xs: 3.5, md: 4.5 },
                  borderBottom: `1px solid ${LP.rule}`,
                  ...RISE,
                  ...delay(i),
                  ...NO_MOTION,
                }}
              >
                <Typography
                  aria-hidden
                  sx={{
                    fontSize: { xs: 26, md: 34 },
                    fontWeight: 700,
                    lineHeight: 1,
                    color: LP.seal,
                    gridRow: { xs: 'span 2', md: 'auto' },
                  }}
                >
                  {s.n}
                </Typography>
                <Box>
                  <Typography
                    component="h3"
                    sx={{ fontSize: { xs: 18, md: 21 }, fontWeight: 700, lineHeight: 1.65 }}
                  >
                    {s.title}
                  </Typography>
                  <Typography
                    sx={{
                      mt: 1.5,
                      fontSize: 12,
                      color: LP.muted,
                      borderLeft: `2px solid ${LP.rule}`,
                      pl: 1.25,
                      lineHeight: 1.8,
                    }}
                  >
                    「{s.forWhom}」に効きます
                  </Typography>
                </Box>
                <Typography
                  sx={{
                    fontSize: 13.5,
                    lineHeight: 2.15,
                    color: LP.inkSoft,
                    gridColumn: { xs: '2', md: 'auto' },
                  }}
                >
                  {s.body}
                </Typography>
              </Box>
            ))}
          </Box>
        </Section>

        {/* ── 3者の入口 ───────────────────────── */}
        <Section id="entrances" label="ご利用の方" title="ご利用の方を選んでください。">
          <Box
            sx={{
              display: 'grid',
              gridTemplateColumns: { xs: '1fr', md: 'repeat(3, 1fr)' },
              gap: 0,
              borderTop: `2px solid ${LP.ink}`,
              borderLeft: { md: `1px solid ${LP.rule}` },
            }}
          >
            {ENTRANCES.map((e, i) => (
              <Box
                key={e.href}
                sx={{
                  p: { xs: 3, md: 3.5 },
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 1.25,
                  borderRight: { md: `1px solid ${LP.rule}` },
                  borderBottom: `1px solid ${LP.rule}`,
                  bgcolor: e.primary ? LP.card : 'transparent',
                  ...RISE,
                  ...delay(i),
                  ...NO_MOTION,
                }}
              >
                {e.primary && (
                  <Typography
                    sx={{
                      alignSelf: 'start',
                      fontSize: 10.5,
                      fontWeight: 700,
                      letterSpacing: '.14em',
                      color: LP.card,
                      bgcolor: LP.seal,
                      px: 1,
                      py: 0.25,
                    }}
                  >
                    在校生の方
                  </Typography>
                )}
                <Typography component="h3" sx={{ fontSize: 21, fontWeight: 700 }}>
                  {e.label}
                </Typography>
                <Typography
                  sx={{ fontSize: 13, lineHeight: 2, color: LP.muted, flexGrow: 1 }}
                >
                  {e.target}
                </Typography>
                <Button
                  href={e.href}
                  disableElevation
                  variant={e.primary ? 'contained' : 'outlined'}
                  sx={{
                    mt: 1.5,
                    borderRadius: 0,
                    fontWeight: 700,
                    fontSize: 14.5,
                    py: 1.3,
                    ...(e.primary
                      ? {
                          bgcolor: LP.primary,
                          color: LP.card,
                          boxShadow: `3px 3px 0 ${LP.ink}`,
                          '&:hover': { bgcolor: LP.primaryHover },
                        }
                      : {
                          borderColor: LP.ink,
                          color: LP.ink,
                          '&:hover': { bgcolor: LP.ink, color: LP.paper, borderColor: LP.ink },
                        }),
                  }}
                >
                  {e.action}
                </Button>
              </Box>
            ))}
          </Box>
        </Section>

        {/* ── FAQ ──────────────────────────── */}
        <Section id="faq" tinted label="よくある質問" title="よくあるご質問">
          <Box sx={{ maxWidth: 840, borderTop: `2px solid ${LP.ink}` }}>
            {FAQS.map((f, i) => (
              <Box
                key={f.q}
                sx={{
                  py: { xs: 3, md: 3.5 },
                  borderBottom: `1px solid ${LP.rule}`,
                  display: 'grid',
                  gridTemplateColumns: { xs: '1.5rem 1fr', md: '2rem 1fr' },
                  columnGap: 2,
                  ...RISE,
                  ...delay(i),
                  ...NO_MOTION,
                }}
              >
                <Typography
                  aria-hidden
                  sx={{ fontSize: 15, fontWeight: 700, color: LP.seal, lineHeight: 1.7 }}
                >
                  問
                </Typography>
                <Box>
                  <Typography
                    component="h3"
                    sx={{ fontSize: { xs: 16, md: 17 }, fontWeight: 700, lineHeight: 1.7 }}
                  >
                    {f.q}
                  </Typography>
                  <Typography sx={{ mt: 1.25, fontSize: 13.5, lineHeight: 2.15, color: LP.muted }}>
                    {f.a}
                  </Typography>
                </Box>
              </Box>
            ))}
          </Box>
        </Section>

        {/* ── クロージング ─────────────────────── */}
        <Box
          sx={{
            borderTop: `2px solid ${LP.ink}`,
            bgcolor: LP.ink,
            color: LP.paper,
            ...GENKO_GRID(28, 'rgba(255,255,255,.06)'),
          }}
        >
          <Container
            maxWidth="md"
            sx={{ px: { xs: 2.5, md: 5 }, py: { xs: 8, md: 12 }, textAlign: 'center' }}
          >
            <Typography
              component="p"
              sx={{
                fontSize: { xs: 23, md: 34 },
                fontWeight: 700,
                lineHeight: 1.6,
                letterSpacing: '.01em',
              }}
            >
              登録して、最初の質問に
              <Box component="br" sx={{ display: { md: 'none' } }} />
              答えるところから。
            </Typography>
            <Typography
              sx={{ mt: 3, fontSize: 13.5, color: 'rgba(250,248,244,.68)', lineHeight: 2.1 }}
            >
              在校生の利用は学校の導入に含まれます。学生個人への課金はありません。
            </Typography>
            <Button
              href="/login"
              disableElevation
              variant="contained"
              sx={{
                ...ctaSx,
                mt: 5,
                bgcolor: LP.paper,
                color: LP.ink,
                boxShadow: `4px 4px 0 ${LP.seal}`,
                '&:hover': {
                  bgcolor: LP.card,
                  transform: 'translate(2px, 2px)',
                  boxShadow: `2px 2px 0 ${LP.seal}`,
                },
              }}
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
