import { Box, Button, Chip, CircularProgress, Paper, TextField, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import AutoFixHighIcon from '@mui/icons-material/AutoFixHigh'
import RateReviewIcon from '@mui/icons-material/RateReview'
import { UI } from '@/lib/design/tokens'
import EsTemplatePanel from '@/components/es/EsTemplatePanel'
import { PLACEHOLDER_PATTERN } from '@/lib/es-templates'
import {
  CHAR_LIMIT_MAX,
  CHAR_LIMIT_MIN,
  ES_TEXT_MAX_LENGTH,
  QUESTION_TYPES,
  type CharLimitMode,
  type EsMode,
} from '../types'

type Props = {
  mode: EsMode
  questionType: string
  setQuestionType: (value: string) => void
  question: string
  setQuestion: (value: string) => void
  originalText: string
  setOriginalText: (value: string) => void
  charLimit: string
  setCharLimit: (value: string) => void
  parsedCharLimit: number | null
  charLimitInvalid: boolean
  charLimitMode: CharLimitMode
  setCharLimitMode: (value: CharLimitMode) => void
  techStack: string
  setTechStack: (value: string) => void
  companyName: string
  setCompanyName: (value: string) => void
  loading: boolean
  error: string | null
  onSubmit: () => void
}

// 左側の入力欄（質問種別・設問・本文・文字数上限・企業名など）と送信ボタン。
// 入力値は呼び出し側が持つ（結果表示や送信処理も同じ値を読むため）
export default function EsInputForm({
  mode,
  questionType,
  setQuestionType,
  question,
  setQuestion,
  originalText,
  setOriginalText,
  charLimit,
  setCharLimit,
  parsedCharLimit,
  charLimitInvalid,
  charLimitMode,
  setCharLimitMode,
  techStack,
  setTechStack,
  companyName,
  setCompanyName,
  loading,
  error,
  onSubmit,
}: Props) {
  return (
    <Paper data-testid="es-rewrite-form" elevation={0} sx={{ p: 3, borderRadius: 2, border: `1px solid ${UI.ruleSoft}`, bgcolor: UI.field }}>
      <Typography sx={{ fontWeight: 700, fontSize: 17, mb: 2.5 }}>
        {mode === 'review' ? 'ES添削' : 'ESリライト'} — 元の文章を入力
      </Typography>

      {/* 質問種別 */}
      <Box sx={{ mb: 2.5 }}>
        <Typography sx={{ fontSize: 13, fontWeight: 600, color: UI.inkSoft, mb: 1 }}>質問種別</Typography>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
          {QUESTION_TYPES.map(type => (
            <Chip
              key={type}
              label={type}
              onClick={() => setQuestionType(type)}
              sx={{
                cursor: 'pointer', fontWeight: 600,
                bgcolor: questionType === type ? UI.mark : UI.tint,
                color: questionType === type ? '#fff' : UI.inkSoft,
                '&:hover': { bgcolor: questionType === type ? `color-mix(in srgb, ${UI.mark} 88%, transparent)` : UI.ruleSoft },
              }}
            />
          ))}
        </Box>
      </Box>

      {/* 書き出しガイドと例文（SOCAIAGENT-613）。種別を変えたら例文の絞り込みも戻す */}
      <EsTemplatePanel
        key={questionType}
        questionType={questionType}
        draft={originalText}
        maxLength={ES_TEXT_MAX_LENGTH}
        onInsert={setOriginalText}
      />

      {/* 設問（#1531） */}
      <Box sx={{ mb: 2.5 }}>
        <TextField
          fullWidth
          size="small"
          label="設問（任意）"
          placeholder="例: 学生時代に力を入れたことを教えてください"
          value={question}
          onChange={e => setQuestion(e.target.value)}
          inputProps={{ maxLength: 500 }}
          helperText="同じ設問で添削し直すと、前回のスコアと比べられます"
        />
      </Box>

      {/* ES本文 */}
      <TextField
        multiline
        rows={10}
        fullWidth
        value={originalText}
        onChange={e => setOriginalText(e.target.value)}
        placeholder="例）チームで開発した経験があります。最初は上手くいきませんでしたが、話し合いを重ねて最終的には完成させることができました。この経験から協調性の大切さを学びました。"
        // APIの上限は6,000字（RAG側 models.ES_TEXT_MAX_LENGTH）。超えた分を送るとRAGのバリデーションエラーになり、
        // 利用者には何が悪いのか伝わらないため入力段階で止める(#1521)
        slotProps={{ htmlInput: { maxLength: ES_TEXT_MAX_LENGTH } }}
        helperText={`${originalText.length} / ${ES_TEXT_MAX_LENGTH} 文字`}
        sx={{
          mb: 2.5,
          '& .MuiOutlinedInput-root': {
            fontSize: 14, lineHeight: 1.8,
            '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
            '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
          },
        }}
      />

      {/* 設問の文字数上限(#1523)。「400字以内」が前提のESに合わせる */}
      <Box sx={{ mb: 3 }}>
        <TextField
          fullWidth
          size="small"
          type="number"
          value={charLimit}
          onChange={e => setCharLimit(e.target.value)}
          label="設問の文字数上限（任意）"
          placeholder="例: 400"
          error={charLimitInvalid}
          helperText={
            charLimitInvalid
              ? `${CHAR_LIMIT_MIN}〜${CHAR_LIMIT_MAX} の整数で入力してください`
              : '指定すると、書き直し後の文章をこの字数に収めます（未指定なら元の文章の長さを基準にします）'
          }
          slotProps={{
            htmlInput: { min: CHAR_LIMIT_MIN, max: CHAR_LIMIT_MAX, step: 50, inputMode: 'numeric' },
          }}
          sx={{
            '& .MuiOutlinedInput-root': {
              '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: charLimitInvalid ? undefined : UI.mark },
              '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: charLimitInvalid ? undefined : UI.mark },
            },
            '& .MuiInputLabel-root.Mui-focused': { color: charLimitInvalid ? undefined : UI.mark },
          }}
        />
        {parsedCharLimit !== null && !charLimitInvalid && (
          <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 1.5 }}>
            {([
              { value: 'within', label: `${parsedCharLimit}字以内` },
              { value: 'around', label: `${parsedCharLimit}字程度` },
            ] as const).map(({ value, label }) => (
              <Chip
                key={value}
                label={label}
                onClick={() => setCharLimitMode(value)}
                sx={{
                  cursor: 'pointer', fontWeight: 600,
                  bgcolor: charLimitMode === value ? UI.mark : UI.tint,
                  color: charLimitMode === value ? '#fff' : UI.inkSoft,
                  '&:hover': { bgcolor: charLimitMode === value ? `color-mix(in srgb, ${UI.mark} 88%, transparent)` : UI.ruleSoft },
                }}
              />
            ))}
          </Box>
        )}
      </Box>

      {/* 添削モード: 志望企業 / リライトモード: 技術スタック */}
      {mode === 'review' ? (
        <>
          <TextField
            fullWidth
            size="small"
            value={companyName}
            onChange={e => setCompanyName(e.target.value)}
            label="志望企業名（任意・入力で企業適合性を評価）"
            placeholder="例: 株式会社サイバーエージェント"
            sx={{
              mb: 3,
              '& .MuiOutlinedInput-root': {
                '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
                '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
              },
              '& .MuiInputLabel-root.Mui-focused': { color: UI.mark },
            }}
          />
          {companyName.trim() !== '' && loading && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
              <CircularProgress size={16} />
              <Typography variant="body2" sx={{ color: UI.inkFaint }}>企業情報を分析中...</Typography>
            </Box>
          )}
        </>
      ) : (
        <>
          <TextField
            fullWidth
            size="small"
            value={techStack}
            onChange={e => setTechStack(e.target.value)}
            label="使用技術スタック（任意）"
            placeholder="例: React, Node.js, PostgreSQL, Docker"
            sx={{
              mb: 3,
              '& .MuiOutlinedInput-root': {
                '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
                '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
              },
              '& .MuiInputLabel-root.Mui-focused': { color: UI.mark },
            }}
          />
          <TextField
            fullWidth
            size="small"
            value={companyName}
            onChange={e => setCompanyName(e.target.value)}
            label="志望企業名（任意・入力で企業情報を参照）"
            placeholder="例: 株式会社サイバーエージェント"
            sx={{
              mb: 3,
              '& .MuiOutlinedInput-root': {
                '&:hover .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
                '&.Mui-focused .MuiOutlinedInput-notchedOutline': { borderColor: UI.mark },
              },
              '& .MuiInputLabel-root.Mui-focused': { color: UI.mark },
            }}
          />
          {companyName.trim() !== '' && loading && (
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 3 }}>
              <CircularProgress size={16} />
              <Typography variant="body2" sx={{ color: UI.inkFaint }}>企業情報を分析中...</Typography>
            </Box>
          )}
        </>
      )}

      {/* ひな形・例文の【 】が残ったまま送ると、AI がそのまま書き直してしまう */}
      {PLACEHOLDER_PATTERN.test(originalText) && (
        <Alert severity="warning" role="status" sx={{ mb: 2, borderRadius: 2, fontSize: 13 }}>
          【 】の部分が残っています。自分の経験に置き換えてから送ってください。
        </Alert>
      )}

      <Button
        variant="contained"
        fullWidth
        size="large"
        disabled={!originalText.trim() || loading || charLimitInvalid}
        onClick={onSubmit}
        startIcon={loading ? <CircularProgress size={18} sx={{ color: '#fff' }} /> : (mode === 'review' ? <RateReviewIcon /> : <AutoFixHighIcon />)}
        sx={{
          bgcolor: UI.mark, '&:hover': { bgcolor: `color-mix(in srgb, ${UI.mark} 88%, transparent)` },
          borderRadius: 2, py: 1.5, fontWeight: 700, fontSize: 16,
          textTransform: 'none',
          '&:disabled': { bgcolor: UI.ruleSoft, color: UI.inkFaint },
        }}
      >
        {loading
          ? (mode === 'review' ? '添削中...' : 'リライト中...')
          : (mode === 'review' ? '添削する' : '書き直す')
        }
      </Button>

      {error && (
        <Alert severity="error" sx={{ mt: 2, borderRadius: 2 }}>{error}</Alert>
      )}
    </Paper>
  )
}
