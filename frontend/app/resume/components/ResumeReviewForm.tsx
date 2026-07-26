'use client'

import {
  Alert,
  Box,
  Button,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  LinearProgress,
  MenuItem,
  Paper,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import type { CompanyCandidate } from '../types'

type ResumeReviewFormProps = {
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
  onClearSelectedCompany: () => void
  jobTitle: string
  onJobTitleChange: (value: string) => void
  candidateType: string
  onCandidateTypeChange: (value: string) => void
  documentId: number | null
  reviewLoading: boolean
  ragReport: string
  reviewError: string
  reviewCompleted: boolean
  onReview: () => void
}

export function ResumeReviewForm({
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
  onClearSelectedCompany,
  jobTitle,
  onJobTitleChange,
  candidateType,
  onCandidateTypeChange,
  documentId,
  reviewLoading,
  ragReport,
  reviewError,
  reviewCompleted,
  onReview,
}: ResumeReviewFormProps) {
  return (
    <Paper sx={{ p: 3 }} elevation={2}>
      <Stack spacing={2}>
        <Typography variant="h6">レビュー実行</Typography>
        <Typography variant="body2" color="text.secondary">
          企業を指定する場合は、DB検索またはWEB実在確認で候補を選択してください（自由入力のみではレビューできません）。
          企業なしで職種のみの一般レビューも可能です。
        </Typography>
        <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1} alignItems={{ sm: 'flex-start' }}>
          <TextField
            label="応募企業名を検索"
            value={companyQuery}
            onChange={(e) => onCompanyQueryChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void onSearchCompanies(false)
              }
            }}
            fullWidth
            helperText={companyValidated ? `選択済み: ${companyName}` : '未選択（職種のみレビュー可）'}
          />
          <Button
            variant="outlined"
            onClick={() => onSearchCompanies(false)}
            disabled={companySearchLoading || !companyQuery.trim()}
            sx={{ whiteSpace: 'nowrap', minWidth: 100 }}
          >
            DB検索
          </Button>
          <Button
            variant="contained"
            onClick={() => onSearchCompanies(true)}
            disabled={companySearchLoading || !companyQuery.trim()}
            sx={{ whiteSpace: 'nowrap', minWidth: 140 }}
          >
            WEBで実在確認
          </Button>
        </Stack>
        {companySearchLoading && <LinearProgress />}
        {companySearchError && <Alert severity="warning">{companySearchError}</Alert>}
        {selectedCompanyMeta && (
          <Alert
            severity="success"
            action={
              <Button color="inherit" size="small" onClick={onClearSelectedCompany}>
                クリア
              </Button>
            }
          >
            確定: {selectedCompanyMeta.name}
            {selectedCompanyMeta.source ? `（${selectedCompanyMeta.source}）` : ''}
            {selectedCompanyMeta.evidence_urls?.[0] ? ` / ${selectedCompanyMeta.evidence_urls[0]}` : ''}
          </Alert>
        )}
        {companyCandidates.length > 0 && (
          <Stack spacing={1}>
            <Typography variant="subtitle2">候補から選択</Typography>
            {companyCandidates.map((c) => (
              <Card
                key={`${c.source}-${c.company_id ?? c.name}`}
                variant="outlined"
                sx={{ '&:hover': { borderColor: 'primary.main' } }}
              >
                <CardActionArea onClick={() => onSelectCompany(c)}>
                  <CardContent sx={{ py: 1.5, '&:last-child': { pb: 1.5 } }}>
                    <Typography fontWeight="bold">{c.name}</Typography>
                    {c.description && (
                      <Typography variant="body2" color="text.secondary">
                        {c.description}
                      </Typography>
                    )}
                    <Chip size="small" label={c.source} sx={{ mt: 0.5 }} />
                  </CardContent>
                </CardActionArea>
              </Card>
            ))}
          </Stack>
        )}
        <TextField
          label={companyName.trim() ? '応募職種 (任意)' : '応募職種 (企業名未選択の場合は必須)'}
          value={jobTitle}
          onChange={(e) => onJobTitleChange(e.target.value)}
          fullWidth
          required={!companyName.trim()}
          error={!companyName.trim() && !jobTitle.trim()}
          helperText={!companyName.trim() ? '企業未選択の場合は職種を入力すると一般レビューが実行されます' : ''}
        />
        <TextField
          select
          label="候補者区分"
          value={candidateType}
          onChange={(e) => onCandidateTypeChange(e.target.value)}
          fullWidth
        >
          <MenuItem value="new_grad">新卒</MenuItem>
          <MenuItem value="mid_career">中途</MenuItem>
        </TextField>
        <Button variant="contained" onClick={onReview} disabled={reviewLoading || !documentId}>
          レビューを生成
        </Button>
        {reviewLoading && (
          <Box>
            <LinearProgress />
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              {ragReport ? '企業別レビューレポートを生成中...' : 'PDFを解析中...（通常30〜60秒かかります）'}
            </Typography>
          </Box>
        )}
        {reviewError && (
          <Alert severity="error">{reviewError}</Alert>
        )}
        {reviewCompleted && !reviewLoading && (
          <Alert severity="success">レビューが完了しました。下の指摘事項をご確認ください。</Alert>
        )}
      </Stack>
    </Paper>
  )
}
