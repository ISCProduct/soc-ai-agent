'use client'

import type { MouseEvent } from 'react'
import {
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  Stack,
  Avatar,
  Chip,
  Snackbar,
  Alert,
  IconButton,
  Tooltip,
} from '@mui/material'
import {
  ArrowBack,
  LocationOn,
  People,
  TrendingUp as TrendingUpIcon,
  Refresh,
  Email,
  Favorite,
  FavoriteBorder,
} from '@mui/icons-material'
import type { AnalysisScores, Company, SnackbarState, SuggestedRole } from '../types'
import { buildEsRewriteQuery, buildInterviewQuery, getTopCategoryScores } from '../utils'
import AnalysisScoreCard from './AnalysisScoreCard'
import {
  GUEST_REGISTER_CTA_LABEL,
  GUEST_REGISTER_PATH,
  getGuestApplicationsButtonProps,
  getGuestEmailButtonProps,
} from '@/lib/guest-limits'
import { SHORTLIST, leadSentences, scaleBand } from '../shortlistTokens'

export interface ResultsListViewProps {
  companies: Company[]
  isProvisional: boolean
  diagnosisSummary?: string | null
  evaluatedCategories?: number | null
  minMatchedAxisCount?: number | null
  diagnosisConfidence?: number | null
  analysisScores: AnalysisScores | null
  scoreComment: string
  analysisError: string | null
  onRetryAnalysis: () => void
  jobSuitabilityComment: string
  suggestedRoles: SuggestedRole[]
  emailSending: boolean
  favoritingId: number | null
  applyingId: number | null
  snackbar: SnackbarState
  isGuestUser: boolean
  onCloseSnackbar: () => void
  onBack: () => void
  onReset: () => void
  onSendEmail: () => void
  onSelectCompany: (company: Company) => void
  onToggleFavorite: (e: MouseEvent, company: Company) => void
  onApply: (e: MouseEvent, company: Company) => void
  onNavigate: (path: string) => void
}

/**
 * マッチング結果一覧（ヘッダー・スコア・職種適性・企業カード・フッター・Snackbar）。
 */
export default function ResultsListView({
  companies,
  isProvisional,
  diagnosisSummary = null,
  evaluatedCategories = null,
  minMatchedAxisCount = null,
  diagnosisConfidence = null,
  analysisScores,
  scoreComment,
  analysisError,
  onRetryAnalysis,
  jobSuitabilityComment,
  suggestedRoles,
  emailSending,
  favoritingId,
  applyingId,
  snackbar,
  isGuestUser,
  onCloseSnackbar,
  onBack,
  onReset,
  onSendEmail,
  onSelectCompany,
  onToggleFavorite,
  onApply,
  onNavigate,
}: ResultsListViewProps) {
  const guestEmailProps = getGuestEmailButtonProps(isGuestUser)
  const guestApplicationsProps = getGuestApplicationsButtonProps(isGuestUser)
  // 一覧全体の幅。各社の位置はこの中で示す
  const band = scaleBand(companies.map((c) => c.matchScore))
  // 一覧の中で最も多い根拠軸数。これを下回る行だけ印を出す
  const maxAxisCount = Math.max(
    0,
    ...companies.map((c) => (typeof c.matchedAxisCount === 'number' ? c.matchedAxisCount : 0)),
  )

  return (
    <Box sx={{
      height: '100vh',
      display: 'flex',
      flexDirection: 'column',
      overflow: 'hidden',
      backgroundColor: SHORTLIST.paper,
      pb: { xs: 7, md: 0 },
    }}>
      {/* ヘッダー部分 */}
      <Box sx={{
        p: { xs: 2, sm: 3 },
        borderBottom: `1px solid ${SHORTLIST.rule}`,
        backgroundColor: SHORTLIST.paper,
        flexShrink: 0,
      }}>
        <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 1, mb: 2 }}>
          <Button variant="outlined" startIcon={<ArrowBack />} onClick={onBack}>
            チャットに戻る
          </Button>
          <Tooltip title={guestEmailProps.title} disableHoverListener={!guestEmailProps.disabled}>
            <span>
              <Button
                variant="contained"
                startIcon={<Email />}
                onClick={onSendEmail}
                disabled={emailSending || guestEmailProps.disabled}
                sx={{ width: { xs: '100%', sm: 'auto' } }}
              >
                {emailSending ? '送信中...' : '結果をメールで受け取る'}
              </Button>
            </span>
          </Tooltip>
        </Box>
        {isGuestUser && (
          <Alert
            severity="info"
            sx={{ mb: 2 }}
            action={
              <Button color="inherit" size="small" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>
                {GUEST_REGISTER_CTA_LABEL}
              </Button>
            }
          >
            ゲスト利用中です。メール送信や選考管理はアカウント登録後に利用できます。
            今の診断結果はそのまま引き継がれます。
          </Alert>
        )}
        {/*
          「AI分析完了！」をやめた理由。
          学生が知りたいのは「どこを受けるか」であって、AIが動いたことではない。
          製品が自分の手柄を先に言うと、結果そのものが軽く見える。
          見出しは何のリストなのかだけを述べる。
        */}
        <Box>
          <Typography
            component="h1"
            sx={{
              fontSize: { xs: 22, sm: 28 },
              fontWeight: 700,
              letterSpacing: '0.01em',
              color: SHORTLIST.ink,
              mb: 0.5,
            }}
          >
            あなたに近い{companies.length}社
          </Typography>
          <Typography sx={{ fontSize: 15, color: SHORTLIST.inkSoft, maxWidth: '62ch', lineHeight: 1.85, mb: 1.5 }}>
            {isProvisional
              ? 'ここまでの回答で並べています。会話を続けると根拠が増え、順番も変わります。'
              : '回答の傾向と、企業が重視する人物像を突き合わせた順に並べています。'}
          </Typography>
          {/*
            適合度の幅を一度だけ述べる。実測では候補が狭い帯に密集しており、
            78 と 71 の差は見た目ほど大きくない。各行に目盛りを並べるより、
            ここで一度「差は小さい」と言うほうが正直で、読む手間も少ない。
          */}
          {companies.length > 1 && band.max - band.min <= 15 && (
            <Typography sx={{ fontSize: 14, color: SHORTLIST.inkSoft, mb: 1.5 }}>
              適合度は{band.min}〜{band.max}に収まっています。順位の差はわずかなので、
              上から順に決める必要はありません。
            </Typography>
          )}
          {/*
            「暫定評価」のチップを外した。同じことを下の注意書きが書いており、
            孤立したピルは色がついているだけで情報を足していなかった。
          */}
          {isProvisional && (
            <Box sx={{ mb: 1.5 }}>
              <Alert
                severity="warning"
                icon={false}
                sx={{
                  textAlign: 'left',
                  maxWidth: '68ch',
                  backgroundColor: 'transparent',
                  border: `1px solid ${SHORTLIST.flag}`,
                  borderRadius: 0.5,
                  color: SHORTLIST.ink,
                  py: 1.25,
                }}
              >
                {diagnosisSummary ||
                  '回答の根拠がまだ薄いため、適合度は参考値です。選択肢に理由を添えると精度が上がります。'}
                {(evaluatedCategories != null || minMatchedAxisCount != null || diagnosisConfidence != null) && (
                  <Typography variant="caption" display="block" sx={{ mt: 0.5 }}>
                    {[
                      evaluatedCategories != null ? `評価カテゴリ ${evaluatedCategories}` : null,
                      minMatchedAxisCount != null ? `最小根拠軸 ${minMatchedAxisCount}` : null,
                      diagnosisConfidence != null ? `診断信頼度 ${diagnosisConfidence}` : null,
                    ]
                      .filter(Boolean)
                      .join(' / ')}
                  </Typography>
                )}
              </Alert>
            </Box>
          )}
        </Box>
      </Box>

      {/* スクロール可能なコンテンツエリア */}
      <Box sx={{
        flexGrow: 1,
        overflowY: 'auto',
        p: { xs: 2, sm: 4 },
        backgroundColor: SHORTLIST.paper,
      }}>
        <Box sx={{ maxWidth: 1200, mx: 'auto' }}>
          <AnalysisScoreCard
            analysisScores={analysisScores}
            scoreComment={scoreComment}
            analysisError={analysisError}
            onRetryAnalysis={onRetryAnalysis}
          />

          {/*
            緑の枠とピルと「→」をやめた。職種名は見出しの重さで足り、
            矢印は役割の説明を箇条書きに見せていただけだった。
          */}
          {(jobSuitabilityComment || suggestedRoles.length > 0) && (
            <Box sx={{ mb: 4 }}>
              <Typography sx={{ fontSize: 17, fontWeight: 700, color: SHORTLIST.ink, mb: 1 }}>
                向いていそうな職種
              </Typography>
              {jobSuitabilityComment && (
                <Typography sx={{ fontSize: 14.5, lineHeight: 1.9, color: SHORTLIST.inkSoft, maxWidth: '64ch', mb: 1.5 }}>
                  {jobSuitabilityComment}
                </Typography>
              )}
              <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0, maxWidth: '72ch' }}>
                {suggestedRoles.map((role, i) => (
                  <Box
                    component="li"
                    key={i}
                    sx={{
                      py: 1.25,
                      borderTop: i === 0 ? `1px solid ${SHORTLIST.ruleSoft}` : 'none',
                      borderBottom: `1px solid ${SHORTLIST.ruleSoft}`,
                    }}
                  >
                    <Typography sx={{ fontSize: 15, fontWeight: 700, color: SHORTLIST.ink }}>
                      {role.title}
                    </Typography>
                    {role.reason && (
                      <Typography sx={{ fontSize: 14, lineHeight: 1.85, color: SHORTLIST.inkSoft, mt: 0.25 }}>
                        {role.reason}
                      </Typography>
                    )}
                  </Box>
                ))}
              </Box>
            </Box>
          )}

          {/*
            カードをやめて罫線区切りの一覧にしている。
            学生の用途は「見比べて絞る」ことなので、1社ずつ箱に入れると
            隣と比べにくく、どれも同じ重みに見える。
          */}
          <Box component="ol" sx={{ listStyle: 'none', m: 0, p: 0 }}>
            {companies.map((company, index) => (
              <Box
                component="li"
                key={`${company.id}-${index}`}
                sx={{
                  borderTop: index === 0 ? `1px solid ${SHORTLIST.rule}` : 'none',
                  borderBottom: `1px solid ${SHORTLIST.rule}`,
                  cursor: 'pointer',
                  px: { xs: 2, sm: 3 },
                  py: { xs: 2.5, sm: 3 },
                  transition: 'background-color 120ms',
                  '&:hover': { backgroundColor: 'rgba(255,255,255,0.55)' },
                  '&:focus-visible': { outline: `3px solid ${SHORTLIST.mark}`, outlineOffset: -3 },
                }}
                onClick={() => onSelectCompany(company)}
              >
                  <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 2 }}>
                    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: { xs: 1.5, sm: 2.5 }, minWidth: 0 }}>
                      {/* 順位。実際に序列があるので番号は情報として機能する */}
                      <Typography
                        aria-hidden
                        sx={{
                          fontSize: { xs: 20, sm: 26 },
                          fontWeight: 700,
                          color: SHORTLIST.rule,
                          fontVariantNumeric: 'tabular-nums',
                          lineHeight: 1,
                          minWidth: { xs: 24, sm: 34 },
                        }}
                      >
                        {index + 1}
                      </Typography>
                      <Box sx={{ minWidth: 0 }}>
                        <Typography sx={{ fontSize: { xs: 17, sm: 20 }, fontWeight: 700, color: SHORTLIST.ink, lineHeight: 1.4 }}>
                          {company.name}
                        </Typography>
                        <Typography sx={{ fontSize: 13, color: SHORTLIST.inkSoft, mt: 0.25 }}>
                          {[company.industry, company.location, company.employees].filter(Boolean).join('　')}
                        </Typography>
                      </Box>
                    </Box>
                    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0 }}>
                      <Tooltip title={company.isFavorited ? 'お気に入りから外す' : 'お気に入りに入れる'}>
                        <IconButton
                          size="small"
                          onClick={(e) => onToggleFavorite(e, company)}
                          disabled={favoritingId === company.matchId}
                          sx={{ color: company.isFavorited ? SHORTLIST.flag : SHORTLIST.rule }}
                        >
                          {company.isFavorited ? <Favorite fontSize="small" /> : <FavoriteBorder fontSize="small" />}
                        </IconButton>
                      </Tooltip>
                      <Typography sx={{
                        fontSize: { xs: 22, sm: 26 }, fontWeight: 700, color: SHORTLIST.ink,
                        fontVariantNumeric: 'tabular-nums', lineHeight: 1,
                      }}>
                        {company.matchScore}
                      </Typography>
                    </Box>
                  </Box>
                  {/*
                    根拠軸数は全行に出さない。ほとんどの行が同じ値になるため、
                    毎行に書くと読み飛ばされ、本当に薄い行も埋もれる。
                    一覧の中で相対的に薄い行だけ印を出す。
                  */}
                  {typeof company.matchedAxisCount === 'number' &&
                    company.matchedAxisCount < maxAxisCount && (
                      <Typography sx={{ fontSize: 13, color: SHORTLIST.flag, mt: 0.5 }}>
                        根拠 {company.matchedAxisCount}軸（他の候補より少なめ）
                      </Typography>
                    )}

                  <Typography sx={{ fontSize: 15, lineHeight: 1.9, color: SHORTLIST.ink, maxWidth: '68ch', mb: 2 }}>
                    {leadSentences(company.description)}
                  </Typography>


                  {/*
                    「技術スタック:」のラベルを外した。Go や AWS が並んでいれば
                    それが何かは見れば分かる。ラベルは行数を増やすだけだった。
                  */}
                  {company.techStack && company.techStack.length > 0 && (
                    <Stack direction="row" spacing={0.75} flexWrap="wrap" useFlexGap sx={{ mb: 1.5 }}>
                      {company.techStack.map((tech, i) => (
                        <Box
                          key={i}
                          sx={{
                            fontSize: 12.5,
                            color: SHORTLIST.inkSoft,
                            border: `1px solid ${SHORTLIST.ruleSoft}`,
                            borderRadius: 0.5,
                            px: 0.9,
                            py: 0.2,
                          }}
                        >
                          {tech}
                        </Box>
                      ))}
                    </Stack>
                  )}

                  {company.tags && company.tags.length > 0 && (
                    <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                      {company.tags.map((tag, i) => (
                        <Chip key={i} label={tag} size="small" />
                      ))}
                    </Stack>
                  )}

                  {/*
                    3本の棒グラフをやめた。値が 97/95/77 のように軒並み高い側へ寄るため、
                    棒はどれも満杯に見えて差を伝えない。どの軸が噛み合ったかだけを
                    1行で書くほうが速く読める。
                  */}
                  {company.categoryScores && (
                    <Typography sx={{ fontSize: 13.5, color: SHORTLIST.inkSoft, mb: 2 }}>
                      噛み合った軸：
                      {getTopCategoryScores(company.categoryScores, 3)
                        .map(({ label, score }) => `${label} ${Math.round(score)}`)
                        .join('　')}
                    </Typography>
                  )}

                  {/*
                    4つのボタンが同じ重さで並んでいた。学生がこの行で次に取る行動は
                    ほぼ「応募する」か「面接を練習する」で、残り2つは寄り道。
                    主・副・副次の3段にして、迷う時間を減らす。
                  */}
                  <Stack direction="row" spacing={1} sx={{ mt: 2, alignItems: 'center', flexWrap: 'wrap', gap: 1 }} useFlexGap>
                    <Button
                      variant="contained"
                      size="small"
                      color="secondary"
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/interview?${buildInterviewQuery(company)}`)
                      }}
                    >
                      この企業の面接を練習する
                    </Button>
                    <Button
                      variant="text"
                      size="small"
                      sx={{ color: SHORTLIST.inkSoft, fontWeight: 500 }}
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/correlation-diagram?company_id=${company.id}`)
                      }}
                    >
                      関連企業を見る
                    </Button>
                    <Button
                      variant="text"
                      size="small"
                      sx={{ color: SHORTLIST.inkSoft, fontWeight: 500 }}
                      onClick={(e) => {
                        e.stopPropagation()
                        onNavigate(`/es-rewrite?${buildEsRewriteQuery(company)}`)
                      }}
                    >
                      ES添削・リライト
                    </Button>
                    <Tooltip title={guestApplicationsProps.title} disableHoverListener={!guestApplicationsProps.disabled}>
                      <span>
                        <Button
                          variant={company.isApplied ? 'contained' : 'outlined'}
                          size="small"
                          color="primary"
                          disabled={company.isApplied || applyingId === company.matchId || guestApplicationsProps.disabled}
                          onClick={(e) => onApply(e, company)}
                        >
                          {company.isApplied ? '応募済み' : applyingId === company.matchId ? '応募中...' : '応募する'}
                        </Button>
                      </span>
                    </Tooltip>
                  </Stack>
              </Box>
            ))}
          </Box>

          <Box sx={{ textAlign: 'center', mt: 4, mb: 4 }}>
            <Stack direction="row" spacing={2} justifyContent="center" flexWrap="wrap" useFlexGap>
              <Tooltip title={guestEmailProps.title} disableHoverListener={!guestEmailProps.disabled}>
                <span>
                  <Button
                    variant="contained"
                    startIcon={<Email />}
                    onClick={onSendEmail}
                    disabled={emailSending || guestEmailProps.disabled}
                  >
                    {emailSending ? '送信中...' : '結果をメールで受け取る'}
                  </Button>
                </span>
              </Tooltip>
              {isGuestUser && (
                <Button variant="contained" color="secondary" onClick={() => onNavigate(GUEST_REGISTER_PATH)}>
                  {GUEST_REGISTER_CTA_LABEL}
                </Button>
              )}
              <Tooltip title={guestApplicationsProps.title} disableHoverListener={!guestApplicationsProps.disabled}>
                <span>
                  <Button
                    variant="outlined"
                    size="large"
                    disabled={guestApplicationsProps.disabled}
                    onClick={() => onNavigate('/applications')}
                  >
                    選考管理を見る
                  </Button>
                </span>
              </Tooltip>
              <Button variant="outlined" size="large" startIcon={<Refresh />} onClick={onReset}>
                最初からやり直す
              </Button>
            </Stack>
          </Box>
        </Box>
      </Box>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={5000}
        onClose={onCloseSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
      >
        <Alert
          severity={snackbar.severity}
          onClose={onCloseSnackbar}
          action={
            snackbar.actionHref ? (
              <Button color="inherit" size="small" onClick={() => onNavigate(snackbar.actionHref!)}>
                {snackbar.actionLabel}
              </Button>
            ) : undefined
          }
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  )
}
