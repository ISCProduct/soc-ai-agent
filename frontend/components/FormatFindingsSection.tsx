'use client'

import { useState } from 'react'
import { Alert, Box, Button, Card, CardContent, Chip, Stack, Typography } from '@mui/material'
import { getSeverityConfig } from '@/app/resume/utils'

/**
 * 形式チェックの指摘（#1530）。ES 添削の format_findings と、履歴書レビューの ResumeFormatFinding。
 * start / end は UTF-16 の位置（JavaScript の文字列の位置と同じ）。履歴書ではブロックの中の位置。
 */
export type FormatFinding = {
  rule: string
  severity: string
  start: number
  end: number
  message: string
  fix: string
  page_hint?: number
  block_index?: number
}

/** 最初に表示する件数。残りは折りたたむ */
export const FORMAT_FINDINGS_VISIBLE = 5
const EXCERPT_MAX = 30
const SEVERITY_RANK: Record<string, number> = { critical: 0, warning: 1, info: 2 }

/** 重要度順（重大 → 注意 → 情報）、同じ重要度ならページ・ブロック・位置の順に並べる */
export function sortFormatFindings(findings: FormatFinding[]): FormatFinding[] {
  return [...findings].sort(
    (a, b) =>
      (SEVERITY_RANK[a.severity] ?? 3) - (SEVERITY_RANK[b.severity] ?? 3) ||
      (a.page_hint ?? 0) - (b.page_hint ?? 0) ||
      (a.block_index ?? 0) - (b.block_index ?? 0) ||
      a.start - b.start,
  )
}

function excerptOf(text: string | undefined, f: FormatFinding): string | null {
  if (!text || f.start < 0 || f.end <= f.start || f.end > text.length) return null
  const s = text.slice(f.start, f.end).replace(/\s+/g, ' ').trim()
  if (!s) return null
  return s.length > EXCERPT_MAX ? `${s.slice(0, EXCERPT_MAX)}…` : s
}

type FormatFindingsSectionProps = {
  /** 応答に format_findings が無い（古いバックエンド）ときは何も表示しない */
  findings: FormatFinding[] | null | undefined
  /** 検査した本文。渡すと、指摘された箇所を抜き出して表示する（ES） */
  text?: string
  /** ページ番号を表示する（履歴書） */
  showPage?: boolean
}

/** ルールで見つけた形式の指摘を「必ず直す」として、AI の改善案とは分けて表示する */
export default function FormatFindingsSection({ findings, text, showPage = false }: FormatFindingsSectionProps) {
  const [expanded, setExpanded] = useState(false)
  if (!findings) return null

  const sorted = sortFormatFindings(findings)
  const hidden = sorted.length - FORMAT_FINDINGS_VISIBLE
  const visible = expanded ? sorted : sorted.slice(0, FORMAT_FINDINGS_VISIBLE)

  return (
    <Box component="section" aria-labelledby="format-findings-heading" data-testid="format-findings">
      <Typography id="format-findings-heading" variant="h6" fontWeight="bold" gutterBottom>
        必ず直す
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
        表記や形式のルールで見つけた点です。提出前に確認して直しましょう。
      </Typography>
      {sorted.length === 0 ? (
        <Alert severity="success" role="status">
          形式の問題は見つかりませんでした。
        </Alert>
      ) : (
        <Stack spacing={1.5}>
          {visible.map((f, i) => {
            const config = getSeverityConfig(f.severity)
            const excerpt = excerptOf(text, f)
            return (
              <Card
                key={`${f.rule}-${f.page_hint ?? 0}-${f.block_index ?? 0}-${f.start}-${i}`}
                data-testid="format-finding"
                variant="outlined"
                sx={{ borderLeft: 4, borderLeftColor: config.borderColor }}
              >
                <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 0.5, flexWrap: 'wrap' }}>
                    <Chip label={config.label} color={config.color} size="small" />
                    {showPage && f.page_hint ? (
                      <Typography variant="caption" color="text.secondary">
                        ページ {f.page_hint}
                      </Typography>
                    ) : null}
                    {excerpt && (
                      <Typography variant="body2" fontWeight="bold" sx={{ overflowWrap: 'anywhere' }}>
                        「{excerpt}」
                      </Typography>
                    )}
                  </Box>
                  <Typography variant="body2">{f.message}</Typography>
                  {f.fix && (
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                      直し方: {f.fix}
                    </Typography>
                  )}
                </CardContent>
              </Card>
            )
          })}
          {hidden > 0 && (
            <Box>
              <Button size="small" onClick={() => setExpanded((v) => !v)} aria-expanded={expanded}>
                {expanded ? '折りたたむ' : `残り${hidden}件を表示`}
              </Button>
            </Box>
          )}
        </Stack>
      )}
    </Box>
  )
}
