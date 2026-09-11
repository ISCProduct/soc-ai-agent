import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Divider,
  Typography,
} from '@mui/material'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import GitHubIcon from '@mui/icons-material/GitHub'
import type { GitHubRepo, RepoSummary } from '../types'

type GitHubRepoSummariesProps = {
  repos: GitHubRepo[]
  summaries: RepoSummary[]
  loading: boolean
  expandedRepo: string | false
  summarizingRepo: string | null
  onExpandRepo: (fullName: string | false) => void
  onSummarizeRepo: (fullName: string) => void
}

export function GitHubRepoSummaries({
  repos,
  summaries,
  loading,
  expandedRepo,
  summarizingRepo,
  onExpandRepo,
  onSummarizeRepo,
}: GitHubRepoSummariesProps) {
  if (repos.length === 0 && !loading) {
    return (
      <>
        <Divider sx={{ borderColor: '#1e293b', my: 3 }} />
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
          <AutoAwesomeIcon sx={{ color: '#818cf8', fontSize: 18 }} />
          <Typography variant="subtitle2" sx={{ color: '#94a3b8' }}>リポジトリAI要約</Typography>
        </Box>
        <Typography variant="body2" sx={{ color: '#64748b' }}>
          リポジトリが見つかりません。「同期」ボタンを押してGitHubデータを取得してください。
        </Typography>
      </>
    )
  }

  if (repos.length === 0) {
    return null
  }

  return (
    <>
      <Divider sx={{ borderColor: '#1e293b', my: 3 }} />
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
        <AutoAwesomeIcon sx={{ color: '#818cf8', fontSize: 18 }} />
        <Typography variant="subtitle1" sx={{ fontWeight: 700, color: '#f1f5f9' }}>
          リポジトリAI要約
        </Typography>
        <Typography variant="caption" sx={{ color: '#475569', ml: 1 }}>
          {summaries.length}/{repos.length} 件生成済み
        </Typography>
      </Box>
      <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
        {repos.map(repo => {
          const summary = summaries.find(s => s.FullName === repo.FullName)
          const isExpanded = expandedRepo === repo.FullName
          const isSummarizing = summarizingRepo === repo.FullName

          return (
            <Box
              key={repo.FullName}
              sx={{ bgcolor: '#1e293b', borderRadius: 1, overflow: 'hidden', border: '1px solid #334155' }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  px: 2,
                  py: 1.2,
                  gap: 1,
                  cursor: summary ? 'pointer' : 'default',
                }}
                onClick={() => summary && onExpandRepo(isExpanded ? false : repo.FullName)}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap' }}>
                    <Typography variant="body2" sx={{ fontWeight: 600, color: '#94a3b8' }} noWrap>
                      {repo.Name}
                    </Typography>
                    {repo.Language && (
                      <Chip
                        label={repo.Language}
                        size="small"
                        sx={{ bgcolor: '#0f172a', color: '#64748b', border: '1px solid #334155', fontSize: '0.65rem', height: 18 }}
                      />
                    )}
                    {summary && (
                      <Chip
                        label="要約済み"
                        size="small"
                        sx={{ bgcolor: '#1e3a5f', color: '#60a5fa', fontSize: '0.65rem', height: 18 }}
                      />
                    )}
                  </Box>
                </Box>
                <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
                  {summary && (
                    <ExpandMoreIcon
                      sx={{
                        color: '#475569',
                        fontSize: 18,
                        transform: isExpanded ? 'rotate(180deg)' : 'rotate(0deg)',
                        transition: 'transform 0.2s',
                      }}
                    />
                  )}
                  <Button
                    size="small"
                    startIcon={isSummarizing ? <CircularProgress size={12} /> : <AutoAwesomeIcon />}
                    onClick={e => { e.stopPropagation(); onSummarizeRepo(repo.FullName) }}
                    disabled={summarizingRepo !== null}
                    sx={{
                      color: summary ? '#475569' : '#818cf8',
                      fontSize: '0.7rem',
                      minWidth: 'unset',
                      px: 1,
                    }}
                  >
                    {isSummarizing ? '生成中...' : summary ? '再生成' : 'AI要約を生成'}
                  </Button>
                </Box>
              </Box>

              {isExpanded && summary && (
                <Box sx={{ px: 2, pb: 2, borderTop: '1px solid #334155' }}>
                  <Typography variant="body2" sx={{ color: '#cbd5e1', mt: 1.5, mb: 1.5 }}>
                    {summary.SummaryText}
                  </Typography>
                  {[
                    { label: '技術選定の理由', value: summary.TechReason, color: '#4FC3F7' },
                    { label: '解決した課題', value: summary.Challenge, color: '#81C784' },
                    { label: '成果', value: summary.Achievement, color: '#FFB74D' },
                  ].map(item => (
                    <Box key={item.label} sx={{ mb: 1 }}>
                      <Typography variant="caption" sx={{ color: item.color, fontWeight: 700 }}>
                        {item.label}
                      </Typography>
                      <Typography variant="body2" sx={{ color: '#94a3b8' }}>{item.value}</Typography>
                    </Box>
                  ))}
                </Box>
              )}
            </Box>
          )
        })}
      </Box>
    </>
  )
}

export type GitHubSkillsErrorAlertProps = {
  error: string
  needsReauth: boolean
  connecting: boolean
  onClearError: () => void
  onConnect: () => void
}

export function GitHubSkillsErrorAlert({
  error,
  needsReauth,
  connecting,
  onClearError,
  onConnect,
}: GitHubSkillsErrorAlertProps) {
  return (
    <Alert
      severity={needsReauth ? 'warning' : 'error'}
      sx={{ mt: 2, mb: 1 }}
      onClose={onClearError}
      action={needsReauth ? (
        <Button
          color="inherit"
          size="small"
          startIcon={connecting ? <CircularProgress size={14} /> : <GitHubIcon />}
          onClick={onConnect}
          disabled={connecting}
        >
          再連携
        </Button>
      ) : undefined}
    >
      {error}
    </Alert>
  )
}
