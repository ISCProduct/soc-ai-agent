'use client'

import { useState } from 'react'
import { Box, Button, Collapse, MenuItem, Stack, TextField, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import ExpandLessIcon from '@mui/icons-material/ExpandLess'
import { UI } from '@/lib/design/tokens'
import { exampleTags, guideFor, insertIntoDraft } from '@/lib/es-templates'

type Props = {
  questionType: string
  draft: string
  maxLength: number
  onInsert: (next: string) => void
}

/**
 * 書き出しガイドと例文（SOCAIAGENT-613）。白紙から書き出す学生向けに、設問の種類ごとの書き方の手順・ひな形・
 * 例文を出し、入力欄へ入れられるようにする。AI は使わない（静的な辞書）。
 * 入れた文章は【 】を自分の経験で埋めて使う。書きかけの文章は消さず、末尾に足す。
 */
export default function EsTemplatePanel({ questionType, draft, maxLength, onInsert }: Props) {
  const [open, setOpen] = useState(draft.trim() === '')
  const [tag, setTag] = useState('')
  const [notice, setNotice] = useState('')
  const guide = guideFor(questionType)
  const tags = exampleTags(guide)
  const examples = guide.examples.filter((e) => tag === '' || e.tag === tag)

  const insert = (text: string) => {
    const next = insertIntoDraft(draft, text, maxLength)
    if (next === null) {
      setNotice(`入力欄の上限（${maxLength}文字）を超えるため入れられません。書いた文章を短くしてから試してください。`)
      return
    }
    setNotice('')
    onInsert(next)
  }

  const appendLabel = draft.trim() === '' ? '入力欄に入れる' : '入力欄の末尾に足す'

  return (
    <Box sx={{ mb: 2.5, border: `1px solid ${UI.ruleSoft}`, borderRadius: 2, bgcolor: UI.paper }}>
      <Button
        fullWidth
        onClick={() => setOpen((v) => !v)}
        aria-expanded={open}
        aria-controls="es-template-panel"
        endIcon={open ? <ExpandLessIcon /> : <ExpandMoreIcon />}
        sx={{ justifyContent: 'space-between', px: 2, py: 1, textTransform: 'none', color: UI.ink, fontWeight: 700 }}
      >
        書き出しガイドと例文（{questionType}）
      </Button>
      <Collapse in={open} unmountOnExit>
        <Box id="es-template-panel" sx={{ px: 2, pb: 2 }}>
          <Typography component="h3" sx={{ fontSize: 13.5, fontWeight: 700, color: UI.ink, mb: 0.5 }}>書く順番</Typography>
          <Box component="ol" sx={{ m: 0, pl: 2.5, mb: 1.5 }}>
            {guide.steps.map((s) => (
              <li key={s}><Typography component="span" sx={{ fontSize: 13.5, color: UI.inkSoft, lineHeight: 1.8 }}>{s}</Typography></li>
            ))}
          </Box>

          <Typography component="h3" sx={{ fontSize: 13.5, fontWeight: 700, color: UI.ink, mb: 0.5 }}>書き出しの例</Typography>
          <Box component="ul" sx={{ m: 0, pl: 2.5, mb: 1.5 }}>
            {guide.openings.map((o) => (
              <li key={o}><Typography component="span" sx={{ fontSize: 13.5, color: UI.inkSoft, lineHeight: 1.8 }}>{o}</Typography></li>
            ))}
          </Box>

          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, flexWrap: 'wrap', mb: 0.5 }}>
            <Typography component="h3" sx={{ fontSize: 13.5, fontWeight: 700, color: UI.ink }}>ひな形</Typography>
            <Button size="small" variant="outlined" onClick={() => insert(guide.template)} aria-label={`ひな形を${appendLabel}`}>
              {appendLabel}
            </Button>
          </Box>
          <Typography sx={{ fontSize: 13, color: UI.inkSoft, whiteSpace: 'pre-wrap', lineHeight: 1.8, bgcolor: UI.tint, borderRadius: 1, p: 1.25, mb: 1.5 }}>
            {guide.template}
          </Typography>

          {guide.examples.length > 0 && (
            <>
              <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} sx={{ alignItems: { sm: 'center' }, justifyContent: 'space-between', mb: 1 }}>
                <Typography component="h3" sx={{ fontSize: 13.5, fontWeight: 700, color: UI.ink }}>例文</Typography>
                <TextField select size="small" label={guide.tagLabel} value={tag} onChange={(e) => setTag(e.target.value)} sx={{ minWidth: 200 }}>
                  <MenuItem value="">すべて</MenuItem>
                  {tags.map((t) => <MenuItem key={t} value={t}>{t}</MenuItem>)}
                </TextField>
              </Stack>
              <Stack component="ul" spacing={1.5} sx={{ listStyle: 'none', m: 0, p: 0 }}>
                {examples.map((e) => (
                  <Box component="li" key={e.title} sx={{ border: `1px solid ${UI.ruleSoft}`, borderRadius: 1, p: 1.25 }}>
                    <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, flexWrap: 'wrap', mb: 0.5 }}>
                      <Typography sx={{ fontSize: 13.5, fontWeight: 600, color: UI.ink }}>{e.title}</Typography>
                      <Button size="small" onClick={() => insert(e.text)} aria-label={`例文「${e.title}」を${appendLabel}`}>{appendLabel}</Button>
                    </Box>
                    <Typography sx={{ fontSize: 13, color: UI.inkSoft, whiteSpace: 'pre-wrap', lineHeight: 1.8 }}>{e.text}</Typography>
                  </Box>
                ))}
              </Stack>
            </>
          )}
          {notice && <Alert severity="warning" sx={{ mt: 1.5 }}>{notice}</Alert>}
          <Typography sx={{ fontSize: 12, color: UI.inkFaint, mt: 1.5 }}>
            例文は書き方の型を示すものです。【 】の部分を自分の経験に置き換え、そのまま提出しないでください。
          </Typography>
        </Box>
      </Collapse>
    </Box>
  )
}
