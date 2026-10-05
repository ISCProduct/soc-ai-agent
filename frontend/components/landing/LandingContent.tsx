import Link from 'next/link'
import { Box, Button, Card, CardContent, Container, Divider, Stack, Typography } from '@mui/material'

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
 */

/** 振り分け先。未ログインで到達する3系統に対応する。 */
const ENTRANCES = [
  {
    role: '学生の方',
    summary: '専門学校で就職活動をしている方',
    body: 'AIとの対話で適性を診断し、IT企業とのマッチング、面接練習、履歴書レビュー、ES添削まで進められます。',
    action: 'ログイン・新規登録',
    href: '/login',
    primary: true,
  },
  {
    role: '企業の方',
    summary: '学生の採用を検討している企業の採用担当者',
    body: '求人を掲載して応募を受け付けられます。スカウト公開に同意した学生を検索して、直接スカウトを送ることもできます。',
    action: '企業ポータルへ',
    href: '/company-portal/sign-in',
    primary: false,
  },
  {
    role: '学校・教員の方',
    summary: '導入校の先生・就職課の方',
    body: '学生の就活状況、要フォローの学生、応募・面接・内定の状況を確認できます。',
    action: '管理画面へ',
    href: '/admin',
    primary: false,
  },
] as const

/** 製品が何をするか。学生向けの機能を、就活の順序に沿って並べる。 */
const FEATURES = [
  { title: 'AI適性診断', body: 'AIとの対話から強み・志向を整理し、スコアとして蓄積します。' },
  { title: '企業マッチング', body: '診断結果と企業側の求める人物像を突き合わせて候補を出します。' },
  { title: 'AI面接練習', body: '音声で面接を練習し、論理性・具体性・主体性などの観点で講評を受け取れます。' },
  { title: '履歴書・ES支援', body: '職務経歴書のレビューと、ESの添削・リライトができます。' },
] as const

export function LandingContent() {
  return (
    <Box component="main" sx={{ pb: 8 }}>
      <Container maxWidth="md" sx={{ pt: { xs: 5, md: 8 }, pb: { xs: 4, md: 6 } }}>
        <Typography variant="overline" color="text.secondary" component="p">
          就活AI
        </Typography>
        <Typography variant="h3" component="h1" fontWeight="bold" sx={{ mt: 1, mb: 2 }}>
          IT企業エージェント
        </Typography>
        <Typography variant="h6" component="p" color="text.secondary" sx={{ maxWidth: '36em' }}>
          適性診断から企業マッチングまで。専門学校の就職活動を支援するAIエージェントです。
        </Typography>
      </Container>

      <Container maxWidth="md" sx={{ pb: { xs: 5, md: 7 } }}>
        <Typography variant="h5" component="h2" fontWeight="bold" sx={{ mb: 1 }}>
          ご利用の方はこちらから
        </Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
          学生・企業・学校でアカウントが分かれています。お使いの立場を選んでください。
        </Typography>

        <Stack spacing={2}>
          {ENTRANCES.map((e) => (
            <Card
              key={e.href}
              elevation={0}
              sx={{ border: '1px solid', borderColor: 'divider', borderRadius: 2 }}
            >
              <CardContent sx={{ p: { xs: 2.5, md: 3 } }}>
                <Typography variant="h6" component="h3" fontWeight="bold">
                  {e.role}
                </Typography>
                <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                  {e.summary}
                </Typography>
                <Typography variant="body1" sx={{ mt: 1.5 }}>
                  {e.body}
                </Typography>
                <Button
                  component={Link}
                  href={e.href}
                  variant={e.primary ? 'contained' : 'outlined'}
                  size="large"
                  sx={{ mt: 2, width: { xs: '100%', sm: 'auto' } }}
                >
                  {/* ボタンは立場込みの具体的な操作名にする。「ログイン」だけだと
                      3枚のカードで同じ文言が並び、どれを押すのか読み取れない。 */}
                  {`${e.role.replace('の方', '')}　${e.action}`}
                </Button>
              </CardContent>
            </Card>
          ))}
        </Stack>
      </Container>

      <Divider />

      <Container maxWidth="md" sx={{ py: { xs: 5, md: 7 } }}>
        <Typography variant="h5" component="h2" fontWeight="bold" sx={{ mb: 3 }}>
          学生ができること
        </Typography>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr' },
            gap: 3,
          }}
        >
          {FEATURES.map((f) => (
            <Box key={f.title}>
              <Typography variant="subtitle1" component="h3" fontWeight="bold">
                {f.title}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                {f.body}
              </Typography>
            </Box>
          ))}
        </Box>
      </Container>

      <Divider />

      <Container maxWidth="md" sx={{ pt: 4 }}>
        <Stack
          direction={{ xs: 'column', sm: 'row' }}
          spacing={{ xs: 1, sm: 3 }}
          alignItems={{ xs: 'flex-start', sm: 'center' }}
        >
          <Typography variant="body2" color="text.secondary">
            © 就活AI
          </Typography>
          <Button component={Link} href="/privacy" size="small" color="inherit">
            プライバシーポリシー
          </Button>
        </Stack>
      </Container>
    </Box>
  )
}
