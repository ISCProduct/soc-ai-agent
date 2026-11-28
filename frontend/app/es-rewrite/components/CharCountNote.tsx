import { Box, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import { UI } from '@/lib/design/tokens'
import type { CharLimitResult } from '../types'

/**
 * 生成結果の字数表示(#1523)。
 *
 * 字数はサーバが数えた値をそのまま出す（改行と前後の空白を数えない数え方は
 * RAG 側の count_es_chars が唯一の定義。画面で数え直すと定義が二重になる）。
 */
export default function CharCountNote({ result, charLimit }: { result: CharLimitResult; charLimit: number | null }) {
  const length = result.improved_text_length
  const overLimit = result.char_limit_satisfied === false
  const tooLong = result.improved_length_issue === 'long'
  // 理由が無い応答は false を短さとみなす。長い文章に「短くなりました」とは出さない。
  const tooShort = result.improved_length_issue === 'short'
    || (result.improved_length_issue == null && result.improved_length_satisfied === false)
  const lengthKnown = typeof length === 'number' && length > 0
  // 字数が0でも「短い」と分かっているときは警告を出す。ここで return すると、
  // 空の改善文が説明なしで画面に残る（#1606）。
  if (!lengthKnown && !tooShort && !tooLong) return null
  return (
    <Box sx={{ mt: 1.5 }}>
      {lengthKnown && (
        <Typography sx={{ fontSize: 13, fontWeight: 600, color: overLimit || tooShort || tooLong ? '#b45309' : UI.inkFaint }}>
          {charLimit === null ? `${length} 文字` : `${length} / ${charLimit} 文字`}
          （改行と前後の空白は数えません）
        </Typography>
      )}
      {overLimit && (
        <Alert severity="warning" role="status" sx={{ mt: 1, borderRadius: 2, fontSize: 13 }}>
          指定字数に収まりませんでした。勝手に切り詰めずそのまま出しているので、削る箇所を選んでから提出してください。もう一度実行すると収まることもあります。
        </Alert>
      )}
      {tooShort && (
        <Alert severity="warning" role="status" sx={{ mt: 1, borderRadius: 2, fontSize: 13 }}>
          生成された文章が元の文章より短くなりました。要約になっていて、元の内容が落ちていることがあります。このまま提出せず、元の文章と見比べてください。
        </Alert>
      )}
      {tooLong && (
        <Alert severity="warning" role="status" sx={{ mt: 1, borderRadius: 2, fontSize: 13 }}>
          生成された文章が元の文章の130%を超えています。このまま提出せず、残す内容を選んで短くしてください。
        </Alert>
      )}
    </Box>
  )
}
