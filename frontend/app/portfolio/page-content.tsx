'use client'

import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useRouter } from 'next/navigation'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  Container,
  FormControlLabel,
  FormGroup,
  Skeleton,
  Stack,
  Typography,
} from '@mui/material'
import PrintIcon from '@mui/icons-material/Print'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { authService } from '@/lib/auth'
import {
  PORTFOLIO_SECTIONS,
  type PortfolioData,
  type PortfolioSection,
  fetchPortfolio,
  formatDate,
} from '@/lib/portfolio'

/**
 * 自己分析ポートフォリオ（#2030）。診断・面接・ES・適性検査を1枚にまとめ、ブラウザの印刷で PDF にする。
 * 学生の個人情報なので本人だけが見る（共有リンクは作らない）。操作部分は印刷に出さない。
 */
export default function PortfolioPage() {
  const router = useRouter()
  const [selected, setSelected] = useState<PortfolioSection[]>(PORTFOLIO_SECTIONS.map((s) => s.key))
  const [data, setData] = useState<PortfolioData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [name, setName] = useState('')

  useEffect(() => {
    setName(authService.getStoredUser()?.name ?? '')
  }, [])

  const load = useCallback(async (sections: PortfolioSection[]) => {
    if (sections.length === 0) {
      setData(null)
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    try {
      setData(await fetchPortfolio(sections))
    } catch {
      setData(null)
      setError('ポートフォリオを読み込めませんでした。通信状況を確認して、もう一度お試しください')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(selected)
  }, [load, selected])

  // 選んだ順ではなく、決まった並び（PORTFOLIO_SECTIONS の順）で出す
  const toggle = (key: PortfolioSection) => {
    setSelected((prev) =>
      PORTFOLIO_SECTIONS.map((s) => s.key).filter((k) => (k === key ? !prev.includes(k) : prev.includes(k))),
    )
  }

  return (
    <Box sx={{ minHeight: '100vh', bgcolor: 'background.default', '@media print': { bgcolor: '#fff' } }}>
      <Box className="no-print" sx={{ bgcolor: 'background.paper', borderBottom: 1, borderColor: 'divider', py: 1.5, '@media print': { display: 'none' } }}>
        <Container maxWidth="md">
          <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', gap: 1 }}>
            <Typography variant="h6" component="h1" fontWeight="bold" color="primary">自己分析ポートフォリオ</Typography>
            <Button startIcon={<ArrowBackIcon />} size="small" onClick={() => router.push('/profile')}>プロフィールへ戻る</Button>
          </Stack>
        </Container>
      </Box>

      <Container maxWidth="md" sx={{ py: 3, pb: { xs: 12, md: 4 }, '@media print': { py: 0 } }}>
        <Card variant="outlined" sx={{ mb: 3, '@media print': { display: 'none' } }}>
          <CardContent>
            <Typography variant="subtitle1" component="h2" fontWeight="bold" gutterBottom>出力する内容</Typography>
            <Typography variant="body2" color="text.secondary" mb={1}>
              就職活動の提出や振り返りに使えるよう、これまでの結果を1つにまとめます。「印刷・PDFで保存」から PDF にできます。
              この画面はあなただけが見られます。
            </Typography>
            <FormGroup row>
              {PORTFOLIO_SECTIONS.map((s) => (
                <FormControlLabel
                  key={s.key}
                  control={<Checkbox checked={selected.includes(s.key)} onChange={() => toggle(s.key)} />}
                  label={s.label}
                />
              ))}
            </FormGroup>
            <Button
              variant="contained"
              startIcon={<PrintIcon />}
              sx={{ mt: 1 }}
              disabled={loading || !data || selected.length === 0}
              onClick={() => window.print()}
            >
              印刷・PDFで保存
            </Button>
          </CardContent>
        </Card>

        {selected.length === 0 && (
          <Alert severity="info">出力する内容を1つ以上選んでください。</Alert>
        )}
        {error && (
          <Alert severity="error" action={<Button color="inherit" size="small" onClick={() => void load(selected)}>再読み込み</Button>}>
            {error}
          </Alert>
        )}
        {loading && selected.length > 0 && (
          <Stack spacing={2} aria-label="ポートフォリオを読み込み中">
            <Skeleton variant="rounded" height={120} />
            <Skeleton variant="rounded" height={160} />
          </Stack>
        )}

        {!loading && data && (
          <Box component="article" aria-label="自己分析ポートフォリオ">
            <Box sx={{ mb: 3 }}>
              <Typography variant="h5" component="h2" fontWeight="bold">自己分析ポートフォリオ</Typography>
              <Typography variant="body2" color="text.secondary">
                {name && `${name}　`}作成日 {formatDate(data.generated_at)}
              </Typography>
            </Box>

            {data.diagnosis && (
              <Section title="適性の診断">
                {data.diagnosis.scores.length === 0 ? (
                  <Empty text="まだ診断の結果がありません。AI との対話で診断を進めると表示されます。" />
                ) : (
                  <>
                    {data.diagnosis.updated_at && <Caption text={`最終更新 ${formatDate(data.diagnosis.updated_at)}`} />}
                    <ScoreList scores={data.diagnosis.scores} />
                  </>
                )}
              </Section>
            )}

            {data.interviews && (
              <Section title="AI 面接のレポート（最新3回）">
                {data.interviews.length === 0 ? (
                  <Empty text="まだ面接のレポートがありません。" />
                ) : data.interviews.map((iv) => (
                  <Box key={iv.session_id} sx={{ mb: 2, breakInside: 'avoid' }}>
                    <Caption text={formatDate(iv.created_at)} />
                    {iv.summary_text && <Typography variant="body2" sx={{ mb: 1 }}>{iv.summary_text}</Typography>}
                    {iv.strengths.length > 0 && <Bullets label="強み" items={iv.strengths} />}
                    {iv.improvements.length > 0 && <Bullets label="改善点" items={iv.improvements} />}
                  </Box>
                ))}
              </Section>
            )}

            {data.es_reviews && (
              <Section title="ES 添削（最新5件）">
                {data.es_reviews.length === 0 ? (
                  <Empty text="まだ ES の添削がありません。" />
                ) : data.es_reviews.map((es) => (
                  <Box key={es.id} sx={{ mb: 2, breakInside: 'avoid' }}>
                    <Caption text={`${formatDate(es.created_at)}${es.company_name ? `・${es.company_name}` : ''}`} />
                    <Typography variant="body2" fontWeight="bold">{es.question}</Typography>
                    <Stack direction="row" spacing={1} sx={{ my: 0.5, flexWrap: 'wrap', gap: 0.5 }}>
                      <Chip size="small" label={`具体性 ${es.specificity_score}`} />
                      <Chip size="small" label={`STAR ${es.star_score}`} />
                      <Chip size="small" label={`文字数の配分 ${es.length_balance_score}`} />
                      {es.company_fit_score != null && <Chip size="small" label={`企業との適合 ${es.company_fit_score}`} />}
                    </Stack>
                    {es.feedback && <Typography variant="body2" color="text.secondary">{es.feedback}</Typography>}
                  </Box>
                ))}
              </Section>
            )}

            {data.assessments && (
              <Section title="適性検査（最新3回）">
                {data.assessments.length === 0 ? (
                  <Empty text="まだ適性検査の結果がありません。" />
                ) : data.assessments.map((a, i) => (
                  <Box key={`${a.title}-${i}`} sx={{ mb: 2, breakInside: 'avoid' }}>
                    <Caption text={`${formatDate(a.created_at)}${a.title ? `・${a.title}` : ''}`} />
                    <ScoreList scores={a.scores} />
                  </Box>
                ))}
              </Section>
            )}
          </Box>
        )}
      </Container>
    </Box>
  )
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <Box component="section" sx={{ mb: 3, pb: 2, borderBottom: 1, borderColor: 'divider' }}>
      <Typography variant="h6" component="h3" fontWeight="bold" sx={{ mb: 1 }}>{title}</Typography>
      {children}
    </Box>
  )
}

function Caption({ text }: { text: string }) {
  return <Typography variant="caption" color="text.secondary" display="block" sx={{ mb: 0.5 }}>{text}</Typography>
}

function Empty({ text }: { text: string }) {
  return <Typography variant="body2" color="text.secondary">{text}</Typography>
}

function Bullets({ label, items }: { label: string; items: string[] }) {
  return (
    <Box sx={{ mb: 0.5 }}>
      <Typography variant="body2" fontWeight="bold">{label}</Typography>
      <Box component="ul" sx={{ m: 0, pl: 3 }}>
        {items.map((t, i) => <li key={i}><Typography variant="body2" component="span">{t}</Typography></li>)}
      </Box>
    </Box>
  )
}

/** スコアは棒だけに頼らず、数値も文字で出す（印刷でも読めるように）。 */
function ScoreList({ scores }: { scores: { category: string; score: number }[] }) {
  return (
    <Stack spacing={0.75}>
      {scores.map((s) => (
        <Box key={s.category} sx={{ display: 'grid', gridTemplateColumns: { xs: '7em 1fr 3em', sm: '9em 1fr 3em' }, alignItems: 'center', gap: 1 }}>
          <Typography variant="body2">{s.category}</Typography>
          <Box role="img" aria-label={`${s.category} ${s.score}点`} sx={{ height: 8, borderRadius: 4, bgcolor: 'action.hover', overflow: 'hidden', '@media print': { border: '1px solid #999' } }}>
            <Box sx={{ height: '100%', width: `${Math.max(0, Math.min(100, s.score))}%`, bgcolor: 'primary.main', '@media print': { bgcolor: '#555', printColorAdjust: 'exact' } }} />
          </Box>
          <Typography variant="body2" sx={{ textAlign: 'right' }}>{s.score}</Typography>
        </Box>
      ))}
    </Stack>
  )
}
