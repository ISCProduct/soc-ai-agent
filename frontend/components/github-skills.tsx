'use client'

import { useEffect, useState } from 'react'
import { Box, Typography, Chip, CircularProgress, Paper, Alert, Button, Divider } from '@mui/material'
import GitHubIcon from '@mui/icons-material/GitHub'
import RefreshIcon from '@mui/icons-material/Refresh'
import AutoAwesomeIcon from '@mui/icons-material/AutoAwesome'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { BACKEND_URL } from '@/lib/backend-url'
import { authService } from '@/lib/auth'

interface SkillScore {
  ID: number
  UserID: number
  Category: string
  Score: number
}

interface LanguageStat {
  Language: string
  Percentage: number
}

interface GitHubProfile {
  GitHubLogin: string
  TotalContributions: number
  PublicRepos: number
  Followers: number
}

interface GitHubRepo {
  FullName: string
  Name: string
  Language: string
  Stars: number
  IsForked: boolean
}

interface RepoSummary {
  ID: number
  FullName: string
  SummaryText: string
  TechReason: string
  Challenge: string
  Achievement: string
}

// カテゴリ別の表示色
const CATEGORY_COLORS: Record<string, string> = {
  Frontend:       '#4FC3F7',
  Backend:        '#81C784',
  Infrastructure: '#FFB74D',
  Database:       '#F48FB1',
  Other:          '#CE93D8',
}

const CATEGORY_LABELS: Record<string, string> = {
  Frontend:       'フロントエンド',
  Backend:        'バックエンド',
  Infrastructure: 'インフラ',
  Database:       'DB',
  Other:          'その他',
}

const AXES_ORDER = ['Frontend', 'Backend', 'Infrastructure', 'Database', 'Other']

// --- SVG レーダーチャート ---

interface RadarChartProps {
  scores: SkillScore[]
  size?: number
}

function RadarChart({ scores, size = 240 }: RadarChartProps) {
  const center = size / 2
  const radius = size * 0.38
  const n = AXES_ORDER.length
  const scoreMap = Object.fromEntries(scores.map(s => [s.Category, s.Score]))

  const angleOf = (i: number) => (Math.PI * 2 * i) / n - Math.PI / 2

  const pointOnAxis = (i: number, ratio: number) => {
    const a = angleOf(i)
    return {
      x: center + radius * ratio * Math.cos(a),
      y: center + radius * ratio * Math.sin(a),
    }
  }

  // グリッド（25, 50, 75, 100）
  const gridLevels = [0.25, 0.5, 0.75, 1.0]
  const gridPolygons = gridLevels.map(level => {
    const pts = AXES_ORDER.map((_, i) => {
      const p = pointOnAxis(i, level)
      return `${p.x},${p.y}`
    }).join(' ')
    return pts
  })

  // データポリゴン
  const dataPoints = AXES_ORDER.map((cat, i) => {
    const score = scoreMap[cat] ?? 0
    const p = pointOnAxis(i, score / 100)
    return `${p.x},${p.y}`
  }).join(' ')

  // 軸ラベル
  const labelOffset = 22
  const labels = AXES_ORDER.map((cat, i) => {
    const a = angleOf(i)
    return {
      x: center + (radius + labelOffset) * Math.cos(a),
      y: center + (radius + labelOffset) * Math.sin(a),
      label: CATEGORY_LABELS[cat] ?? cat,
      color: CATEGORY_COLORS[cat] ?? '#888',
    }
  })

  return (
    <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
      {/* グリッド */}
      {gridPolygons.map((pts, i) => (
        <polygon
          key={i}
          points={pts}
          fill="none"
          stroke="#334155"
          strokeWidth="1"
          strokeDasharray={i < gridPolygons.length - 1 ? '3,3' : undefined}
        />
      ))}
      {/* 軸線 */}
      {AXES_ORDER.map((_, i) => {
        const outer = pointOnAxis(i, 1)
        return (
          <line
            key={i}
            x1={center}
            y1={center}
            x2={outer.x}
            y2={outer.y}
            stroke="#334155"
            strokeWidth="1"
          />
        )
      })}
      {/* データ領域 */}
      <polygon
        points={dataPoints}
        fill="rgba(79,195,247,0.25)"
        stroke="#4FC3F7"
        strokeWidth="2"
      />
      {/* 頂点の丸 */}
      {AXES_ORDER.map((cat, i) => {
        const score = scoreMap[cat] ?? 0
        const p = pointOnAxis(i, score / 100)
        return (
          <circle
            key={i}
            cx={p.x}
            cy={p.y}
            r="4"
            fill={CATEGORY_COLORS[cat] ?? '#4FC3F7'}
          />
        )
      })}
      {/* ラベル */}
      {labels.map((l, i) => (
        <text
          key={i}
          x={l.x}
          y={l.y}
          textAnchor="middle"
          dominantBaseline="middle"
          fontSize="10"
          fill={l.color}
          fontWeight="600"
        >
          {l.label}
        </text>
      ))}
    </svg>
  )
}

// --- メインコンポーネント ---

export default function GitHubSkills({ userId, targetRole = '' }: { userId: number; targetRole?: string }) {
  const [scores, setScores] = useState<SkillScore[]>([])
  const [profile, setProfile] = useState<GitHubProfile | null>(null)
  const [langStats, setLangStats] = useState<LanguageStat[]>([])
  const [repos, setRepos] = useState<GitHubRepo[]>([])
  const [summaries, setSummaries] = useState<RepoSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [syncing, setSyncing] = useState(false)
  const [notLinked, setNotLinked] = useState(false)
  const [needsReauth, setNeedsReauth] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [connecting, setConnecting] = useState(false)
  const [summarizingRepo, setSummarizingRepo] = useState<string | null>(null)
  const [expandedRepo, setExpandedRepo] = useState<string | false>(false)

  useEffect(() => {
    fetchAll()
  }, [userId])

  const fetchAll = async () => {
    setLoading(true)
    setError(null)
    setNotLinked(false)
    try {
      const headers = authService.getUserFetchHeaders()
      const [skillsRes, profileRes, summariesRes] = await Promise.all([
        fetch(`${BACKEND_URL}/api/github/skills?user_id=${userId}`, { headers }),
        fetch(`${BACKEND_URL}/api/github/profile?user_id=${userId}`, { headers }),
        fetch(`${BACKEND_URL}/api/github/repo/summaries?user_id=${userId}`, { headers }),
      ])

      if (skillsRes.ok) {
        const data = await skillsRes.json()
        setScores(Array.isArray(data) ? data : [])
      }

      if (profileRes.ok) {
        const data = await profileRes.json()
        setProfile(data.profile ?? null)
        setLangStats(
          Array.isArray(data.language_stats)
            ? data.language_stats.sort((a: LanguageStat, b: LanguageStat) => b.Percentage - a.Percentage).slice(0, 8)
            : []
        )
        setRepos(Array.isArray(data.repositories) ? data.repositories : [])
      } else if (profileRes.status === 404) {
        setNotLinked(true)
      }

      if (summariesRes.ok) {
        const data = await summariesRes.json()
        setSummaries(Array.isArray(data) ? data : [])
      }
    } catch {
      setError('データの取得に失敗しました')
    } finally {
      setLoading(false)
    }
  }

  const handleSummarizeRepo = async (fullName: string) => {
    setSummarizingRepo(fullName)
    setError(null)
    try {
      const res = await fetch(`${BACKEND_URL}/api/github/repo/summarize?user_id=${userId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({ full_name: fullName, target_role: targetRole }),
      })
      if (!res.ok) {
        const msg = await res.text()
        throw new Error(msg || `HTTP ${res.status}`)
      }
      const newSummary: RepoSummary = await res.json()
      setSummaries(prev => {
        const filtered = prev.filter(s => s.FullName !== fullName)
        return [newSummary, ...filtered]
      })
    } catch (e) {
      setError(`要約生成に失敗しました: ${e instanceof Error ? e.message : '不明なエラー'}`)
    } finally {
      setSummarizingRepo(null)
    }
  }

  const handleConnect = () => {
    setConnecting(true)
    window.location.href = `${BACKEND_URL}/api/auth/github`
  }

  const handleSync = async () => {
    setSyncing(true)
    setError(null)
    setNeedsReauth(false)
    try {
      // sync/wait で同期的に実行してスコープ不足エラーを検出する
      const res = await fetch(`${BACKEND_URL}/api/github/sync/wait?user_id=${userId}`, {
        method: 'POST',
        headers: authService.getUserFetchHeaders(),
      })
      if (res.status === 403) {
        const msg = await res.text()
        setNeedsReauth(true)
        setError(msg)
        return
      }
      if (!res.ok) {
        setError('同期に失敗しました')
        return
      }
      await fetchAll()
    } catch {
      setError('同期に失敗しました')
    } finally {
      setSyncing(false)
    }
  }

  if (loading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', py: 4 }}>
        <CircularProgress size={32} />
      </Box>
    )
  }

  if (notLinked) {
    return (
      <Paper sx={{ p: 3, mt: 3, bgcolor: '#0f172a', borderRadius: 2 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
          <GitHubIcon sx={{ color: '#94a3b8' }} />
          <Typography variant="h6" sx={{ fontWeight: 700, color: '#f1f5f9' }}>
            GitHub スキル分析
          </Typography>
        </Box>
        <Alert
          severity="info"
          sx={{ mb: 2, bgcolor: '#1e293b', color: '#94a3b8', '& .MuiAlert-icon': { color: '#4FC3F7' } }}
        >
          GitHubアカウントが連携されていません。GitHubでログインするとスキル分析が自動的に生成されます。
        </Alert>
        <Button
          variant="contained"
          startIcon={connecting ? <CircularProgress size={16} /> : <GitHubIcon />}
          onClick={handleConnect}
          disabled={connecting}
          sx={{ bgcolor: '#24292e', '&:hover': { bgcolor: '#444d56' } }}
        >
          {connecting ? '連携中...' : 'GitHubと連携する'}
        </Button>
      </Paper>
    )
  }

  const hasScores = scores.some(s => s.Score > 0)

  return (
    <Paper sx={{ p: 3, mt: 3, bgcolor: '#0f172a', color: '#e2e8f0', borderRadius: 2 }}>
      {/* ヘッダー */}
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

      {/* プロフィール統計 */}
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
        {/* レーダーチャート */}
        {hasScores && (
          <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
            <RadarChart scores={scores} size={240} />
            {/* スコア凡例 */}
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

        {/* 言語スキルタグ */}
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
        <Alert
          severity={needsReauth ? 'warning' : 'error'}
          sx={{ mt: 2, mb: 1 }}
          onClose={() => { setError(null); setNeedsReauth(false) }}
          action={needsReauth ? (
            <Button
              color="inherit"
              size="small"
              startIcon={connecting ? <CircularProgress size={14} /> : <GitHubIcon />}
              onClick={handleConnect}
              disabled={connecting}
            >
              再連携
            </Button>
          ) : undefined}
        >
          {error}
        </Alert>
      )}

      {!hasScores && !loading && (
        <Typography variant="body2" sx={{ color: '#64748b', textAlign: 'center', py: 2 }}>
          スキルデータがありません。「同期」ボタンでGitHubデータを取得してください。
        </Typography>
      )}

      {/* リポジトリAI要約セクション */}
      {profile && repos.length === 0 && !loading && (
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
      )}
      {repos.length > 0 && (
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
                  {/* リポジトリ行 */}
                  <Box
                    sx={{
                      display: 'flex',
                      alignItems: 'center',
                      px: 2,
                      py: 1.2,
                      gap: 1,
                      cursor: summary ? 'pointer' : 'default',
                    }}
                    onClick={() => summary && setExpandedRepo(isExpanded ? false : repo.FullName)}
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
                        onClick={e => { e.stopPropagation(); handleSummarizeRepo(repo.FullName) }}
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

                  {/* 要約内容（展開時） */}
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
      )}
    </Paper>
  )
}
