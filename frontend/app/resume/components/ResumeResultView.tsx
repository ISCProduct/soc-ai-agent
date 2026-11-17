'use client'

import { useMemo, useState } from 'react'
import { Box, Button, ButtonBase, Collapse, Stack, Typography, useMediaQuery, useTheme } from '@mui/material'
import DownloadIcon from '@mui/icons-material/Download'
import ScoreUpdateBanner, { type WeightScore } from '@/components/feedback/ScoreUpdateBanner'
import { UI } from '@/lib/design/tokens'
import type { ReviewResult } from '../types'
import { buildFindings, countBySeverity, pageCountOf } from '../utils'
import { NoBadge, SeverityMark } from './ResumeFindingParts'
import { ResumeDocumentViewer } from './ResumeDocumentViewer'
import { ResumeFindingsPane, type FindingFilter } from './ResumeFindingsPane'
import { ResumeResultSummary } from './ResumeResultSummary'
import { ResumeSegmented } from './ResumeSegmented'

type ResumeResultViewProps = {
  review: ReviewResult
  scoresBefore: WeightScore[] | null
  scoresAfter: WeightScore[] | null
  annotateError: string
  onDownload: () => void
  /** 企業別レビューレポート（長文）。あれば末尾で開いて読める */
  ragReport: string
}

/**
 * 結果。PC は書類（左）と指摘（右）の分割、スマホは「指摘」「書類」のタブ。
 * 書類の強調と指摘は同じ番号で結ぶ。
 */
export function ResumeResultView({ review, scoresBefore, scoresAfter, annotateError, onDownload, ragReport }: ResumeResultViewProps) {
  const theme = useTheme()
  const compact = useMediaQuery(theme.breakpoints.down('md'))

  const findings = useMemo(() => buildFindings(review), [review])
  const counts = useMemo(() => countBySeverity(findings), [findings])
  const pageCount = pageCountOf(findings)

  const [filter, setFilter] = useState<FindingFilter>('all')
  const [selectedNo, setSelectedNo] = useState<number | null>(findings[0]?.no ?? null)
  const [page, setPage] = useState(findings[0]?.page ?? 1)
  const [tab, setTab] = useState<'findings' | 'doc'>('findings')
  const [reportOpen, setReportOpen] = useState(false)

  const visible = filter === 'all' ? findings : findings.filter((f) => f.severity === filter)

  const select = (no: number) => {
    const f = findings.find((x) => x.no === no)
    if (!f) return
    setSelectedNo(no)
    setPage(f.page)
  }
  const changeFilter = (next: FindingFilter) => {
    setFilter(next)
    const list = next === 'all' ? findings : findings.filter((f) => f.severity === next)
    if (list.length > 0 && !list.some((f) => f.no === selectedNo)) select(list[0].no)
  }
  const changePage = (next: number) => {
    setPage(next)
    const first = findings.find((f) => f.page === next)
    if (first) setSelectedNo(first.no)
  }
  const move = (delta: number) => {
    const i = visible.findIndex((f) => f.no === selectedNo)
    const target = visible[i + delta]
    if (target) select(target.no)
  }

  const pageFindings = findings.filter((f) => f.page === page)
  const annotated = review.annotated_available

  const pane = (
    <ResumeFindingsPane
      findings={visible}
      counts={counts}
      totalCount={findings.length}
      filter={filter}
      onFilterChange={changeFilter}
      selectedNo={selectedNo}
      currentPage={page}
      onSelect={select}
      showNav={!compact}
      onPrev={() => move(-1)}
      onNext={() => move(1)}
      onShowInDocument={
        compact
          ? (no) => {
              select(no)
              setTab('doc')
            }
          : undefined
      }
    />
  )
  const viewer = (
    <ResumeDocumentViewer page={page} pageCount={pageCount} findings={pageFindings} selectedNo={selectedNo} onSelect={select} onPageChange={changePage} />
  )

  return (
    <Box sx={{ pb: { xs: 12, md: 7 } }}>
      {/*
        スコア無しのときは BE が user_weight_scores へ意図的に何も書かないので、
        反映済みバナーを出さない（#1529）。既存スコア行があるユーザーでは
        差分0件でも「反映されました」と表示され、虚偽の主張になる。
      */}
      {scoresAfter && review.review.score !== null && (
        <Box sx={{ mb: 2 }}>
          <ScoreUpdateBanner beforeScores={scoresBefore} afterScores={scoresAfter} title="職務経歴書レビュー結果がプロフィールスコアに反映されました" />
        </Box>
      )}

      <ResumeResultSummary
        review={review.review}
        counts={counts}
        annotatedAvailable={annotated}
        annotateError={annotateError}
        onDownload={onDownload}
        showDownload={!compact}
      />

      {compact ? (
        <Stack spacing={2}>
          <ResumeSegmented
            label="表示の切り替え"
            value={tab}
            onChange={setTab}
            options={[
              { value: 'findings', label: `指摘 ${findings.length}` },
              { value: 'doc', label: '書類' },
            ]}
          />
          {tab === 'findings' ? (
            <>
              {pane}
              {pageCount > 1 && (
                <Typography sx={{ fontSize: 12, color: UI.inkSoft }}>※ 指摘をタップすると、書き換え案が開きます。</Typography>
              )}
            </>
          ) : (
            <>
              {viewer}
              <Box sx={{ bgcolor: UI.field, border: `1px solid ${UI.rule}`, borderRadius: '8px', overflow: 'hidden' }}>
                <Typography sx={{ px: 2, py: 1, fontSize: 14, fontWeight: 700, color: UI.mark, bgcolor: UI.markTint }}>
                  {page}ページ目の指摘 {pageFindings.length}件・タップで強調
                </Typography>
                {pageFindings.map((f) => {
                  const selected = f.no === selectedNo
                  return (
                    <ButtonBase
                      key={f.no}
                      onClick={() => select(f.no)}
                      aria-pressed={selected}
                      sx={{
                        width: '100%',
                        justifyContent: 'flex-start',
                        alignItems: 'flex-start',
                        gap: 1.25,
                        px: 2,
                        py: 1.5,
                        textAlign: 'left',
                        borderBottom: `1px solid ${UI.ruleSoft}`,
                        outline: selected ? `2px solid ${UI.mark}` : 'none',
                        outlineOffset: -2,
                      }}
                    >
                      <NoBadge no={f.no} selected={selected} />
                      <SeverityMark severity={f.severity} />
                      <Typography sx={{ flex: 1, fontSize: 14, lineHeight: '22px', color: UI.ink }}>{f.message}</Typography>
                    </ButtonBase>
                  )
                })}
              </Box>
              <ButtonBase
                onClick={() => setTab('findings')}
                sx={{ alignSelf: 'center', minHeight: 44, fontSize: 14, fontWeight: 700, color: UI.mark, textDecoration: 'underline', textUnderlinePosition: 'from-font', borderRadius: '2px' }}
              >
                指摘タブに戻って、書き換え案を見る
              </ButtonBase>
            </>
          )}
        </Stack>
      ) : (
        <Box sx={{ display: 'grid', gridTemplateColumns: 'minmax(0, 680px) minmax(0, 1fr)', gap: 3, alignItems: 'start' }}>
          {viewer}
          {pane}
        </Box>
      )}

      {ragReport && (
        <Box sx={{ mt: 3 }}>
          <ButtonBase
            onClick={() => setReportOpen((v) => !v)}
            aria-expanded={reportOpen}
            sx={{ minHeight: 44, fontSize: 14, fontWeight: 700, color: UI.mark, textDecoration: 'underline', textUnderlinePosition: 'from-font', borderRadius: '2px' }}
          >
            {reportOpen ? '企業別レビューレポートを閉じる' : '企業別レビューレポートを読む'}
          </ButtonBase>
          <Collapse in={reportOpen} unmountOnExit>
            <Box component="article" sx={{ mt: 1, p: { xs: 2, md: 2.5 }, bgcolor: UI.field, border: `1px solid ${UI.rule}`, borderRadius: '8px' }}>
              <Typography component="h2" sx={{ fontSize: 18, fontWeight: 700, color: UI.ink, mb: 1 }}>
                企業別レビューレポート
              </Typography>
              <Box sx={{ whiteSpace: 'pre-wrap', lineHeight: 1.8, color: UI.ink, overflowWrap: 'anywhere' }}>{ragReport}</Box>
              <Typography sx={{ display: 'block', mt: 1.5, fontSize: 12, color: UI.inkSoft }}>
                AIが生成した内容です。内容を確認し、ご自身の判断で書類に反映してください。
              </Typography>
            </Box>
          </Collapse>
        </Box>
      )}

      {compact && annotated && (
        <Box
          sx={{
            position: 'fixed',
            left: 0,
            right: 0,
            bottom: 'calc(56px + env(safe-area-inset-bottom))',
            zIndex: 2,
            px: 2,
            py: 1.5,
            bgcolor: UI.field,
            borderTop: `1px solid ${UI.rule}`,
          }}
        >
          <Button
            fullWidth
            variant="outlined"
            onClick={onDownload}
            startIcon={<DownloadIcon />}
            sx={{ color: UI.ink, borderColor: UI.inkFaint, borderWidth: 1.5, '&:hover': { borderWidth: 1.5, borderColor: UI.ink, bgcolor: UI.field } }}
          >
            注釈つきPDFをダウンロード
          </Button>
        </Box>
      )}
    </Box>
  )
}
