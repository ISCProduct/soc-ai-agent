'use client'

import { Box, Chip, Paper, Stack, Typography } from '@mui/material'
import { UI } from '@/lib/design/tokens'
import { parseQuestionCategoryBreakdown } from '@/lib/interview/utils'

/** 「次に練習したい」の印を付ける点数の上限。5点と4点のように、どれも十分なときは付けない */
export const WEAK_MARK_MAX_SCORE = 3

type Props = {
  /** report.category_breakdown_json。無い（この機能より前のレポート）なら何も出さない */
  json?: string | null
  paperBg: string
  paperBorder: string
  textPrimary: string
  textSecondary: string
  textMuted: string
}

/**
 * 質問の種類ごとの評価（SOCAIAGENT-610）。志望動機・経験・強み弱み など、質問の種類ごとの点数と、
 * 足りなかった点・次に練習することを出す。いちばん点の低い種類に「次に練習したい」の印を付ける。
 * 点数は AI の講評の一部なので、確定した評価ではないことを添える。
 */
export default function QuestionCategoryBreakdown({ json, paperBg, paperBorder, textPrimary, textSecondary, textMuted }: Props) {
  const items = parseQuestionCategoryBreakdown(json)
  if (items.length === 0) return null
  const lowest = Math.min(...items.map((i) => i.score))
  // 全部同じ点なら、どれか1つを弱点として目立たせない。いちばん低くても十分な点なら付けない
  const markWeakest = items.length > 1 && items.some((i) => i.score !== lowest) && lowest <= WEAK_MARK_MAX_SCORE

  return (
    <Paper component="section" aria-labelledby="question-category-title" sx={{ bgcolor: paperBg, border: paperBorder, p: 3, borderRadius: 2, order: 2 }}>
      <Typography id="question-category-title" component="h3" sx={{ color: textPrimary, fontWeight: 700, mb: 0.5 }}>
        質問の種類ごとの評価
      </Typography>
      <Typography variant="caption" sx={{ color: textMuted, display: 'block', mb: 1.5 }}>
        この面接で聞かれた質問の種類ごとに、AI が回答を 0〜5 で評価した参考値です。1種類あたりの質問は1〜2問と少ないため、点数は目安として見てください。
      </Typography>
      <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {items.map((it) => {
          const weakest = markWeakest && it.score === lowest
          return (
            <Box component="li" key={it.category} sx={{ borderLeft: `3px solid ${weakest ? UI.flag : 'transparent'}`, pl: 1.5 }}>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 0.25 }}>
                <Typography sx={{ color: textPrimary, fontWeight: 600, fontSize: 14.5 }}>{it.category}</Typography>
                <Typography sx={{ color: textSecondary, fontSize: 14, fontVariantNumeric: 'tabular-nums' }}>{it.score} / 5</Typography>
                {weakest && <Chip size="small" label="次に練習したい" variant="outlined" sx={{ borderColor: UI.flag, color: UI.flag, height: 22 }} />}
              </Box>
              {it.weakness && (
                <Typography variant="body2" sx={{ color: textSecondary, lineHeight: 1.7 }}>{it.weakness}</Typography>
              )}
              {it.practice && (
                <Typography variant="body2" sx={{ color: textSecondary, lineHeight: 1.7 }}>
                  <Box component="span" sx={{ fontWeight: 600, color: textPrimary }}>次の練習：</Box>{it.practice}
                </Typography>
              )}
            </Box>
          )
        })}
      </Stack>
    </Paper>
  )
}
