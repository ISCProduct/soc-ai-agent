'use client'

import { Chip, Link as MuiLink, Stack, Tooltip, Typography } from '@mui/material'
import InfoOutlinedIcon from '@mui/icons-material/InfoOutlined'
import { classifyProvenance, type CompanyProvenanceInput } from '@/lib/company-provenance'

interface ProvenanceBadgeProps {
  provenance: CompanyProvenanceInput | null | undefined
  /** 出どころを表示する対象（ツールチップの文脈に使う。例: 「技術スタック」） */
  target?: string
}

/**
 * 企業情報の出どころバッジ（#1125 フェーズ1）。
 *
 * AI 推定の情報が公的情報と見分けがつかない状態を解消する。
 * 表示するものが無ければ何も描画しない（バッジだらけにしない）。
 */
export function ProvenanceBadge({ provenance, target }: ProvenanceBadgeProps) {
  const label = classifyProvenance(provenance)
  if (!label) return null

  const tooltip = target ? `${target}: ${label.detail}` : label.detail

  return (
    <Stack direction="row" spacing={0.75} alignItems="center" component="span">
      <Tooltip title={tooltip} arrow>
        <Chip
          size="small"
          variant={label.kind === 'ai' ? 'outlined' : 'filled'}
          color={label.tone === 'default' ? undefined : label.tone}
          icon={<InfoOutlinedIcon />}
          label={
            label.confidenceLabel
              ? `${label.label}（${label.confidenceLabel.replace('確信度: ', '')}）`
              : label.label
          }
          // Playwright / 支援技術から参照できるようにする
          data-testid={`provenance-${label.kind}`}
          aria-label={tooltip}
        />
      </Tooltip>
      {label.evidenceUrl && (
        <Typography variant="caption" component="span">
          <MuiLink
            href={label.evidenceUrl}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="provenance-evidence-link"
          >
            根拠
          </MuiLink>
        </Typography>
      )}
    </Stack>
  )
}
