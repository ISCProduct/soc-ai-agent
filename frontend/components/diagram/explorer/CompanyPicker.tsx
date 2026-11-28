'use client'

import { useEffect, useMemo, useState } from 'react'
import { Box, Button, ButtonBase, TextField, Typography } from '@mui/material'
import { Alert } from '@/components/common/Alert'
import ArrowForward from '@mui/icons-material/ArrowForward'
import { PageLoading } from '@/components/common/PageLoading'
import { fetchCompanyRelations, type CapitalRelation } from '@/lib/company/data'
import { UI } from '@/lib/design/tokens'

export function CompanyPicker({ onSelect }: { onSelect: (id: number, mode: 'capital' | 'business') => void }) {
  const [relations, setRelations] = useState<CapitalRelation[] | null>(null)
  const [error, setError] = useState(false)
  const [retry, setRetry] = useState(0)
  const [query, setQuery] = useState('')
  useEffect(() => {
    let active = true
    setError(false)
    fetchCompanyRelations().then((data) => { if (active) setRelations(data) }).catch(() => { if (active) setError(true) })
    return () => { active = false }
  }, [retry])
  const companies = useMemo(() => {
    const found = new Map<number, { id: number; name: string; neighbors: Set<number>; capital: boolean }>()
    for (const relation of relations ?? []) {
      for (const [a, b] of [[relation.parent, relation.child], [relation.from, relation.to]]) {
        if (!a || !b || a.id === b.id) continue
        for (const [company, other] of [[a, b], [b, a]]) {
          const entry = found.get(company.id) ?? { id: company.id, name: company.name, neighbors: new Set<number>(), capital: false }
          entry.neighbors.add(other.id)
          entry.capital ||= relation.relation_type.startsWith('capital')
          found.set(company.id, entry)
        }
      }
    }
    return [...found.values()].sort((a, b) => a.name.localeCompare(b.name, 'ja') || a.id - b.id)
  }, [relations])
  const normalize = (value: string) => value.normalize('NFKC').toLocaleLowerCase('ja').replace(/\s/g, '')
  const visible = companies.filter((company) => normalize(company.name).includes(normalize(query)))
  return <Box component="section" aria-labelledby="company-picker-title" sx={{ width: '100%', maxWidth: 760, mx: 'auto', mt: { xs: 2, md: 4 }, minHeight: 0, overflowY: 'auto', pb: 3, px: 0.5 }}>
    <Typography id="company-picker-title" component="h2" sx={{ fontSize: 22, fontWeight: 700, mb: 2 }}>気になる企業から探す</Typography>
    <Typography sx={{ mb: 3 }}>企業を選ぶと、つながりのある会社とその事業を見ることができます。</Typography>
    <TextField fullWidth label="企業名で検索" value={query} onChange={(event) => setQuery(event.target.value)} sx={{ mb: 2, '& .MuiOutlinedInput-root': { borderRadius: '12px' } }} />
    {error ? <Alert severity="error" action={<Button onClick={() => setRetry((value) => value + 1)}>再試行</Button>}>企業一覧を取得できませんでした。</Alert>
      : !relations ? <PageLoading message="企業一覧を読み込んでいます…" />
        : <>
          <Typography role="status" sx={{ mb: 3 }}>{visible.length}社</Typography>
          {visible.length === 0 && <Typography>{companies.length === 0 ? '相関図を表示できる企業はまだ登録されていません。' : '条件に合う企業が見つかりません。企業名を変えて検索してください。'}</Typography>}
          <Box sx={{ display: 'grid', gap: 1.5 }}>
            {visible.map((company) => <ButtonBase key={company.id} onClick={() => onSelect(company.id, company.capital ? 'capital' : 'business')}
              sx={{ p: 2.5, minHeight: 96, border: `1px solid ${UI.rule}`, borderRadius: '12px', textAlign: 'left', justifyContent: 'space-between', gap: 2, '&:hover': { bgcolor: UI.paper }, '&.Mui-focusVisible': { outline: `2px solid ${UI.mark}`, outlineOffset: 2 } }}>
              <Box sx={{ minWidth: 0, overflowWrap: 'anywhere' }}><Typography component="span" sx={{ display: 'block', fontWeight: 700, fontSize: 18 }}>{company.name}</Typography><Typography component="span" sx={{ display: 'block', mt: 0.5, fontSize: 14, color: UI.inkSoft }}>つながりのある企業：{company.neighbors.size}社</Typography></Box>
              <ArrowForward aria-hidden sx={{ flexShrink: 0 }} />
            </ButtonBase>)}
          </Box>
        </>}
  </Box>
}
