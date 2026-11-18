'use client'

import { useEffect, useRef, useState } from 'react'
import { Alert, Box, ButtonBase, InputAdornment, LinearProgress, Stack, TextField, Typography } from '@mui/material'
import SearchIcon from '@mui/icons-material/Search'
import type { CompanyCandidate } from '../types'
import { UI } from '@/lib/design/tokens'
import { ResumeCard } from './ResumeCard'
import { ResumeSegmented } from './ResumeSegmented'

type ResumeTargetCardProps = {
  mode: 'company' | 'job'
  onModeChange: (mode: 'company' | 'job') => void
  companyQuery: string
  onCompanyQueryChange: (value: string) => void
  companyName: string
  companyValidated: boolean
  companySearchLoading: boolean
  companySearchError: string
  selectedCompanyMeta: CompanyCandidate | null
  companyCandidates: CompanyCandidate[]
  onSearchCompanies: (includeWebSearch: boolean) => void | Promise<void>
  onSelectCompany: (candidate: CompanyCandidate) => void
  jobTitle: string
  onJobTitleChange: (value: string) => void
  candidateType: string
  onCandidateTypeChange: (value: string) => void
}

const linkSx = {
  display: 'inline-flex',
  alignItems: 'center',
  p: 0,
  minHeight: 44,
  font: 'inherit',
  fontSize: 14,
  fontWeight: 700,
  color: UI.mark,
  textDecoration: 'underline',
  textUnderlinePosition: 'from-font',
  verticalAlign: 'baseline',
  borderRadius: '2px',
} as const

const rowSx = { display: 'flex', alignItems: 'center', flexWrap: 'wrap', columnGap: 0.75 } as const

function sourceText(source: string) {
  return source === 'db' ? '掲載企業' : 'Web で見つかった企業'
}

/** ② 応募先。企業を検索して選ぶか、職種だけで見るかを決める。 */
export function ResumeTargetCard({
  mode,
  onModeChange,
  companyQuery,
  onCompanyQueryChange,
  companyName,
  companyValidated,
  companySearchLoading,
  companySearchError,
  selectedCompanyMeta,
  companyCandidates,
  onSearchCompanies,
  onSelectCompany,
  jobTitle,
  onJobTitleChange,
  candidateType,
  onCandidateTypeChange,
}: ResumeTargetCardProps) {
  const [jobTouched, setJobTouched] = useState(false)
  const searchTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // 入力が止まったら、掲載企業から自動で探す。選び終わったあとは探し直さない
  useEffect(() => {
    if (mode !== 'company' || companyValidated || !companyQuery.trim()) return
    searchTimer.current = setTimeout(() => void onSearchCompanies(false), 400)
    return () => {
      if (searchTimer.current) clearTimeout(searchTimer.current)
    }
  }, [companyQuery, mode, companyValidated])

  return (
    <ResumeCard title="2　応募先を決める">
      <ResumeSegmented
        label="応募先の決め方"
        value={mode}
        onChange={onModeChange}
        options={[
          { value: 'company', label: '企業を選ぶ' },
          { value: 'job', label: '職種だけで見る' },
        ]}
      />

      {mode === 'company' ? (
        <>
          <TextField
            value={companyQuery}
            onChange={(e) => onCompanyQueryChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                // Enter で今すぐ探すときは、入力待ちの自動検索を止めて二重に走らせない
                if (searchTimer.current) clearTimeout(searchTimer.current)
                void onSearchCompanies(false)
              }
            }}
            placeholder="企業名を入力して検索"
            fullWidth
            slotProps={{
              htmlInput: { 'aria-label': '応募企業名を検索' },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon sx={{ color: UI.inkSoft }} />
                  </InputAdornment>
                ),
              },
            }}
            sx={{ '& .MuiOutlinedInput-root': { borderRadius: '6px', '& fieldset': { borderWidth: 1.5, borderColor: UI.inkFaint } } }}
          />
          {companySearchLoading && <LinearProgress aria-label="企業を探しています" />}
          {companySearchError && <Alert severity="warning">{companySearchError}</Alert>}

          {companyCandidates.length > 0 && (
            <Stack role="radiogroup" aria-label="応募企業の候補" spacing={1}>
              {companyCandidates.map((c) => {
                const selected = companyValidated && selectedCompanyMeta?.name === c.name && selectedCompanyMeta?.source === c.source
                return (
                  <ButtonBase
                    key={`${c.source}-${c.company_id ?? c.name}`}
                    role="radio"
                    aria-checked={selected}
                    onClick={() => onSelectCompany(c)}
                    sx={{
                      width: '100%',
                      justifyContent: 'flex-start',
                      gap: 1.5,
                      px: 2,
                      py: 1.75,
                      textAlign: 'left',
                      borderRadius: '8px',
                      bgcolor: selected ? UI.markTint : UI.field,
                      border: selected ? `2px solid ${UI.mark}` : `1px solid ${UI.rule}`,
                      // 枠の太さが変わっても中身がずれないよう、非選択側で1px分を補う
                      ...(selected ? {} : { m: '1px' }),
                    }}
                  >
                    <Box
                      aria-hidden
                      sx={{
                        width: 20,
                        height: 20,
                        flexShrink: 0,
                        borderRadius: '50%',
                        border: `2px solid ${selected ? UI.ink : UI.inkFaint}`,
                        display: 'grid',
                        placeItems: 'center',
                        bgcolor: UI.field,
                      }}
                    >
                      {selected && <Box sx={{ width: 10, height: 10, borderRadius: '50%', bgcolor: UI.ink }} />}
                    </Box>
                    <Box sx={{ minWidth: 0 }}>
                      <Typography sx={{ fontSize: 16, lineHeight: '24px', fontWeight: 700, color: UI.ink, overflowWrap: 'anywhere' }}>
                        {c.name}
                      </Typography>
                      <Typography sx={{ fontSize: 13, lineHeight: '20px', color: UI.inkSoft }}>{sourceText(c.source)}</Typography>
                    </Box>
                  </ButtonBase>
                )
              })}
            </Stack>
          )}

          {companyQuery.trim() ? (
            <Box sx={{ ...rowSx, fontSize: 14, color: UI.inkSoft }}>
              候補にない場合は
              <ButtonBase onClick={() => void onSearchCompanies(true)} disabled={!companyQuery.trim()} sx={linkSx}>
                Web から探す
              </ButtonBase>
            </Box>
          ) : (
            <Box sx={{ ...rowSx, fontSize: 14, color: UI.inkSoft }}>
              企業を決めずに
              <ButtonBase onClick={() => onModeChange('job')} sx={linkSx}>
                職種だけで見る
              </ButtonBase>
              こともできます
            </Box>
          )}
        </>
      ) : (
        <TextField
          label="応募職種"
          value={jobTitle}
          onChange={(e) => onJobTitleChange(e.target.value)}
          onBlur={() => setJobTouched(true)}
          placeholder="例: ソフトウェアエンジニア"
          required
          fullWidth
          error={jobTouched && !jobTitle.trim()}
          helperText={jobTouched && !jobTitle.trim() ? '職種を入力してください' : '企業を決めずに、職種に向けた一般的なレビューをします'}
        />
      )}

      <Stack spacing={1}>
        <Typography component="h3" sx={{ fontSize: 14, lineHeight: '20px', fontWeight: 700, color: UI.inkSoft }}>
          候補者区分
        </Typography>
        <ResumeSegmented
          label="候補者区分"
          value={candidateType as 'new_grad' | 'mid_career'}
          onChange={onCandidateTypeChange}
          options={[
            { value: 'new_grad', label: '新卒' },
            { value: 'mid_career', label: '中途' },
          ]}
        />
      </Stack>
    </ResumeCard>
  )
}
