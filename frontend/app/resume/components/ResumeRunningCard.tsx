'use client'

import { useEffect, useState } from 'react'
import { Box, ButtonBase, Stack, Typography } from '@mui/material'
import CheckIcon from '@mui/icons-material/Check'
import { UI } from '@/lib/design/tokens'
import type { ReviewProgress } from '../types'

/** Backend の progress.stage（0〜3）と同じ並び */
export const REVIEW_STAGES = ['書類を読み取る', '内容を確認する', 'レポートを書く', '仕上げをする'] as const
const HEADLINES = ['書類を読み取っています', '内容を確認しています', 'レポートを書いています', '仕上げをしています'] as const

/** 通常かかる時間の目安（秒）。これを超えたら「お知らせ」を出す */
export const EXPECTED_SECONDS = { min: 30, max: 60 } as const

export function formatElapsed(seconds: number): string {
  if (seconds < 60) return `${seconds}秒`
  return `${Math.floor(seconds / 60)}分${seconds % 60}秒`
}

type ResumeRunningCardProps = {
  progress: ReviewProgress | null
  fileName: string
  targetLabel: string
  candidateLabel: string
  onCancel: () => void
}

function StageRow({ index, current }: { index: number; current: number }) {
  const state = index < current ? 'done' : index === current ? 'current' : 'todo'
  const statusText = { done: '完了', current: '進行中', todo: '待機中' }[state]
  return (
    <Stack component="li" direction="row" alignItems="center" spacing={1.5} sx={{ minHeight: 44, py: 0.5 }} aria-current={state === 'current' ? 'step' : undefined}>
      <Box
        aria-hidden
        sx={{
          width: 28,
          height: 28,
          flexShrink: 0,
          borderRadius: '14px',
          display: 'grid',
          placeItems: 'center',
          fontSize: 14,
          fontWeight: 700,
          bgcolor: state === 'done' ? 'success.main' : state === 'current' ? UI.mark : UI.field,
          color: state === 'todo' ? UI.inkSoft : 'primary.contrastText',
          border: state === 'todo' ? `1.5px solid ${UI.inkFaint}` : 'none',
        }}
      >
        {state === 'done' ? <CheckIcon sx={{ fontSize: 18 }} /> : index + 1}
      </Box>
      <Typography sx={{ flex: 1, fontSize: 16, fontWeight: state === 'current' ? 700 : 400, color: state === 'todo' ? UI.inkSoft : UI.ink }}>
        {REVIEW_STAGES[index]}
      </Typography>
      <Typography sx={{ fontSize: 15, fontWeight: state === 'current' ? 700 : 400, color: state === 'current' ? UI.mark : UI.inkSoft }}>{statusText}</Typography>
    </Stack>
  )
}

/**
 * レビュー中。進み具合（％・段階）、経過時間と目安、中止の手段を出す。
 * ％は Backend が処理の節目ごとに送る目安で、実測の進捗ではない。受け取る前は 0% から始める。
 */
export function ResumeRunningCard({ progress, fileName, targetLabel, candidateLabel, onCancel }: ResumeRunningCardProps) {
  const [elapsed, setElapsed] = useState(0)
  useEffect(() => {
    const startedAt = Date.now()
    const timer = setInterval(() => setElapsed(Math.floor((Date.now() - startedAt) / 1000)), 1000)
    return () => clearInterval(timer)
  }, [])

  // 想定外の段階番号が来ても表示が崩れないよう 0〜最終段階に収める
  const stage = Math.min(Math.max(progress?.stage ?? 0, 0), REVIEW_STAGES.length - 1)
  const percent = progress?.percent ?? 0
  const over = elapsed > EXPECTED_SECONDS.max
  const rows = [
    ['書類', fileName],
    ['応募先', targetLabel],
    ['候補者区分', candidateLabel],
  ]
  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', pt: 2, pb: { xs: 6, md: 15 } }}>
      <Stack
        spacing={2.5}
        alignItems="center"
        sx={{
          width: '100%',
          maxWidth: 640,
          pt: { xs: 4, md: 5 },
          pb: { xs: 3, md: 4 },
          px: { xs: 2.5, md: 6 },
          bgcolor: UI.field,
          border: `1px solid ${UI.rule}`,
          borderRadius: '8px',
        }}
      >
        <Typography role="status" sx={{ fontSize: { xs: 20, md: 22 }, lineHeight: '32px', fontWeight: 700, color: UI.ink, textAlign: 'center' }}>
          {HEADLINES[stage]}
        </Typography>
        <Box aria-hidden sx={{ display: 'flex', alignItems: 'flex-end', gap: 0.5, color: UI.mark }}>
          <Typography component="span" sx={{ fontSize: 56, lineHeight: '64px', fontWeight: 700 }}>
            {percent}
          </Typography>
          <Typography component="span" sx={{ fontSize: 24, lineHeight: '40px', fontWeight: 700 }}>
            %
          </Typography>
        </Box>
        <Typography sx={{ fontSize: 16, color: UI.inkSoft, textAlign: 'center' }}>
          進み具合の目安（{stage + 1} / {REVIEW_STAGES.length} 段階目）
        </Typography>
        <Box
          role="progressbar"
          aria-label="レビューの進み具合"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={percent}
          aria-valuetext={`${percent}%（${REVIEW_STAGES[stage]}）`}
          sx={{ width: '100%', height: 16, borderRadius: '8px', bgcolor: UI.rule, border: `1px solid ${UI.inkFaint}`, overflow: 'hidden' }}
        >
          <Box sx={{ width: `${percent}%`, height: '100%', bgcolor: UI.mark, transition: 'width 0.4s ease' }} />
        </Box>
        <Stack direction="row" justifyContent="space-between" sx={{ width: '100%' }}>
          <Typography sx={{ fontSize: 15, fontWeight: 700, color: UI.ink }}>経過 {formatElapsed(elapsed)}</Typography>
          <Typography sx={{ fontSize: 15, color: UI.inkSoft }}>
            {over ? `目安 ${EXPECTED_SECONDS.min}〜${EXPECTED_SECONDS.max}秒を超えています` : `目安 ${EXPECTED_SECONDS.min}〜${EXPECTED_SECONDS.max}秒`}
          </Typography>
        </Stack>
        {/* 超過の案内を読み上げるため、ライブリージョンは常に置いておき中身だけ差し込む。毎秒変わる経過時間は外に置く */}
        <Box aria-live="polite" sx={{ width: '100%', '&:empty': { display: 'none' } }}>
          {over && (
            <Box sx={{ px: 2, py: 1.5, bgcolor: UI.flagTint, border: `1px solid ${UI.flag}`, borderRadius: '8px' }}>
              <Typography sx={{ fontSize: 16, fontWeight: 700, color: UI.ink }}>お知らせ</Typography>
              <Typography sx={{ fontSize: 16, color: UI.ink }}>
                いつもより時間がかかっています。混み合っているときは2〜3分かかることがあります。この画面を開いたままお待ちください。
              </Typography>
            </Box>
          )}
        </Box>
        <Stack component="ol" sx={{ width: '100%', listStyle: 'none', m: 0, p: 0 }}>
          {REVIEW_STAGES.map((label, i) => (
            <StageRow key={label} index={i} current={stage} />
          ))}
        </Stack>
        <Stack component="dl" spacing={0.75} sx={{ width: '100%', m: 0, px: 2.5, py: 2, bgcolor: UI.tint, borderRadius: '8px', textAlign: 'left' }}>
          {rows.map(([label, value]) => (
            <Stack key={label} direction="row" spacing={2}>
              <Typography component="dt" sx={{ width: 90, flexShrink: 0, fontSize: 14, color: UI.inkSoft }}>
                {label}
              </Typography>
              <Typography component="dd" sx={{ m: 0, fontSize: 15, fontWeight: 700, color: UI.ink, overflowWrap: 'anywhere' }}>
                {value}
              </Typography>
            </Stack>
          ))}
        </Stack>
        <ButtonBase
          onClick={onCancel}
          sx={{ minHeight: 44, fontSize: 14, color: UI.ink, textDecoration: 'underline', textUnderlinePosition: 'from-font', borderRadius: '2px' }}
        >
          中止して入力に戻る
        </ButtonBase>
      </Stack>
    </Box>
  )
}
