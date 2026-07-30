import {
  Box,
  Button,
  Chip,
  CircularProgress,
  Paper,
  Typography,
} from '@mui/material'
import GitHubIcon from '@mui/icons-material/GitHub'
import RefreshIcon from '@mui/icons-material/Refresh'
import { AXES_ORDER, CATEGORY_COLORS, CATEGORY_LABELS } from '../constants'
import { hasSkillScores } from '../utils'
import type { GitHubProfile, GitHubRepo, LanguageStat, RepoSummary, SkillScore } from '../types'
import { RadarChart } from './RadarChart'
import { GitHubRepoSummaries, GitHubSkillsErrorAlert } from './GitHubRepoSummaries'

type GitHubSkillsViewProps = {
  scores: SkillScore[]
  profile: GitHubProfile | null
  langStats: LanguageStat[]
  repos: GitHubRepo[]
  summaries: RepoSummary[]
  loading: boolean
  syncing: boolean
  needsReauth: boolean
  error: string | null
  connecting: boolean
  summarizingRepo: string | null
  expandedRepo: string | false
  setExpandedRepo: (fullName: string | false) => void
  handleSummarizeRepo: (fullName: string) => void
  handleConnect: () => void
  handleSync: () => void
  clearError: () => void
}

export function GitHubSkillsView({
  scores,
  profile,
  langStats,
  repos,
  summaries,
  loading,
  syncing,
  needsReauth,
  error,
  connecting,
  summarizingRepo,
  expandedRepo,
  setExpandedRepo,
  handleSummarizeRepo,
  handleConnect,
  handleSync,
  clearError,
}: GitHubSkillsViewProps) {
  const showScores = hasSkillScores(scores)

  return (
    <Paper sx={{ p: 3, mt: 3, bgcolor: '#0f172a', color: '#e2e8f0', borderRadius: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <GitHubIcon sx={{ color: '#94a3b8' }} />
          <Typography variant="h6" sx={{ fontWeight: 700, color: '#f1f5f9' }}>
            GitHub スキル分析
          </Typography>
          {profile && (
            <Typography variant="body2" sx={{ color: '#64748b', ml: 1 }}>
              @{profile.GitHubLogin}
            </Typography>
          )}
        </Box>
        <Box sx={{ display: 'flex', gap: 1 }}>
          {profile && (
            <Button
              size="small"
              startIcon={connecting ? <CircularProgress size={14} /> : <GitHubIcon />}
              onClick={handleConnect}
              disabled={connecting || syncing}
              sx={{ color: '#64748b', fontSize: '0.75rem' }}
            >
              {connecting ? '連携中...' : '権限を更新'}
            </Button>
          )}
          <Button
            size="small"
            startIcon={syncing ? <CircularProgress size={14} /> : <RefreshIcon />}
            onClick={handleSync}
            disabled={syncing}
            sx={{ color: '#64748b', fontSize: '0.75rem' }}
          >
            {syncing ? '同期中...' : '同期'}
          </Button>
        </Box>
      </Box>

      {profile && (
        <Box sx={{ display: 'flex', gap: 3, mb: 3 }}>
          {[
            { label: 'コントリビューション', value: profile.TotalContributions },
            { label: 'リポジトリ', value: profile.PublicRepos },
            { label: 'フォロワー', value: profile.Followers },
          ].map(item => (
            <Box key={item.label} sx={{ textAlign: 'center' }}>
              <Typography variant="h6" sx={{ color: '#4FC3F7', fontWeight: 700 }}>
                {item.value}
              </Typography>
              <Typography variant="caption" sx={{ color: '#64748b' }}>
                {item.label}
              </Typography>
            </Box>
          ))}
        </Box>
      )}

      <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 3, alignItems: 'flex-start' }}>
        {showScores && (
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <RadarChart scores={scores} size={240} />
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1, mt: 1, justifyContent: 'center', maxWidth: 240 }}>
              {AXES_ORDER.map(cat => {
                const s = scores.find(x => x.Category === cat)
                if (!s) return null
                return (
                  <Chip
                    key={cat}
                    label={`${CATEGORY_LABELS[cat]} ${s.Score.toFixed(1)}`}
                    size="small"
                    sx={{
                      bgcolor: CATEGORY_COLORS[cat] + '33',
                      color: CATEGORY_COLORS[cat],
                      border: `1px solid ${CATEGORY_COLORS[cat]}55`,
                      fontSize: '0.7rem',
                    }}
                  />
                )
              })}
            </Box>
          </Box>
        )}

        {langStats.length > 0 && (
          <Box sx={{ flex: 1, minWidth: 180 }}>
            <Typography variant="subtitle2" sx={{ color: '#94a3b8', mb: 1.5 }}>
              使用言語
            </Typography>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
              {langStats.map(stat => (
                <Chip
                  key={stat.Language}
                  label={`${stat.Language} ${stat.Percentage.toFixed(1)}%`}
                  size="small"
                  sx={{
                    bgcolor: '#1e293b',
                    color: '#94a3b8',
                    border: '1px solid #334155',
                    fontSize: '0.75rem',
                  }}
                />
              ))}
            </Box>
          </Box>
        )}
      </Box>

      {error && (
        <GitHubSkillsErrorAlert
          error={error}
          needsReauth={needsReauth}
          connecting={connecting}
          onClearError={clearError}
          onConnect={handleConnect}
        />
      )}

      {!showScores && !loading && (
        <Typography variant="body2" sx={{ color: '#64748b', textAlign: 'center', py: 2 }}>
          スキルデータがありません。「同期」ボタンでGitHubデータを取得してください。
        </Typography>
      )}

      {profile && (
        <GitHubRepoSummaries
          repos={repos}
          summaries={summaries}
          loading={loading}
          expandedRepo={expandedRepo}
          summarizingRepo={summarizingRepo}
          onExpandRepo={setExpandedRepo}
          onSummarizeRepo={handleSummarizeRepo}
        />
      )}
    </Paper>
  )
}
