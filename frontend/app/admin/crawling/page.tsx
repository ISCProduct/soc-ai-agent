'use client'

import { useEffect, useMemo, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  Divider,
  FormControlLabel,
  FormGroup,
  MenuItem,
  Stack,
  TextField,
  Typography,
} from '@mui/material'
import Grid from '@mui/material/GridLegacy'
import { authService } from '@/lib/auth'

type CrawlSource = {
  id: number
  name: string
  target_type: string
  source_type?: string
  source_url?: string
  schedule_type: string
  schedule_day: number
  schedule_time: string
  is_active: boolean
  last_run_at?: string
  next_run_at?: string
}

type CrawlRun = {
  id: number
  source_id: number
  status: string
  message?: string
  started_at: string
  ended_at?: string
}

type ScraperSession = {
  id: number
  site_key: string
  cookies: string
  expires_at?: string
  updated_at: string
}

const WEEKDAY_OPTIONS = [
  { value: 0, label: '日' },
  { value: 1, label: '月' },
  { value: 2, label: '火' },
  { value: 3, label: '水' },
  { value: 4, label: '木' },
  { value: 5, label: '金' },
  { value: 6, label: '土' },
]

const SITE_OPTIONS = [
  { key: 'career_tasu', label: 'キャリタス就活' },
]

export default function AdminCrawlingPage() {
  const [sources, setSources] = useState<CrawlSource[]>([])
  const [runs, setRuns] = useState<CrawlRun[]>([])
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const [name, setName] = useState('')
  const [targetType, setTargetType] = useState<'company' | 'popular_companies' | 'job_site_company' | 'job_listing'>('job_site_company')
  const [sourceType, setSourceType] = useState('official')
  const [sourceUrl, setSourceUrl] = useState('')
  const [scheduleType, setScheduleType] = useState<'weekly' | 'monthly'>('weekly')
  const [scheduleDay, setScheduleDay] = useState(1)
  const [scheduleTime, setScheduleTime] = useState('09:00')

  // マルチソースクロール（company-graph パイプライン）
  const [graphSites, setGraphSites] = useState<string[]>(['career_tasu'])
  const [graphQuery, setGraphQuery] = useState('IT')
  const [graphPages, setGraphPages] = useState(2)
  const [graphYear, setGraphYear] = useState<string>('')
  const [graphLoading, setGraphLoading] = useState(false)
  const [graphResult, setGraphResult] = useState<{ ok: boolean; logs: string; error?: string } | null>(null)
  const [autoYear, setAutoYear] = useState<number | null>(null)

  // gBizINFO企業名検索
  const [gbizSearchName, setGbizSearchName] = useState('')
  const [gbizSearchLoading, setGbizSearchLoading] = useState(false)
  const [gbizSearchError, setGbizSearchError] = useState('')
  const [gbizSearchResults, setGbizSearchResults] = useState<{
    corporate_number: string
    name: string
    location: string
    company_url: string
    employee_number: number
  }[]>([])
  const [gbizRegisterLoading, setGbizRegisterLoading] = useState<string | null>(null)
  const [gbizRegisterMessage, setGbizRegisterMessage] = useState('')

  // スクレイパーセッション管理
  const [sessions, setSessions] = useState<ScraperSession[]>([])
  const [sessionSiteKey, setSessionSiteKey] = useState('mynavi')
  const [sessionCookies, setSessionCookies] = useState('')
  const [sessionExpiresAt, setSessionExpiresAt] = useState('')
  const [sessionLoading, setSessionLoading] = useState(false)
  const [sessionError, setSessionError] = useState('')
  const [sessionSuccess, setSessionSuccess] = useState('')

  useEffect(() => {
    fetch('/api/admin/company-graph-crawl')
      .then((r) => r.json())
      .then((d) => setAutoYear(d.target_year))
      .catch(() => {})
  }, [])

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
    }
  }, [])

  const loadSources = async () => {
    setError('')
    const response = await fetch('/api/admin/crawl-sources', {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await response.json()
    if (!response.ok) {
      setError(data?.error || 'クローリング設定の取得に失敗しました')
      return
    }
    setSources(data?.sources || [])
  }

  const loadRuns = async () => {
    const response = await fetch('/api/admin/crawl-runs', {
      headers: authService.getAdminFetchHeaders(),
    })
    const data = await response.json()
    if (response.ok) {
      setRuns(data?.runs || [])
    }
  }

  const loadSessions = async () => {
    const res = await fetch('/api/admin/scraper-sessions', {
      headers: authService.getAdminFetchHeaders(),
    })
    if (res.ok) {
      const d = await res.json()
      setSessions(d?.sessions || [])
    }
  }

  const handleSessionUpsert = async () => {
    setSessionError('')
    setSessionSuccess('')
    setSessionLoading(true)
    try {
      const payload: Record<string, string> = {
        site_key: sessionSiteKey,
        cookies: sessionCookies,
      }
      if (sessionExpiresAt) payload.expires_at = new Date(sessionExpiresAt).toISOString()

      const res = await fetch('/api/admin/scraper-sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getAdminFetchHeaders() },
        body: JSON.stringify(payload),
      })
      if (!res.ok) {
        const msg = await res.text()
        setSessionError(msg || '登録に失敗しました')
      } else {
        setSessionSuccess('セッションを登録しました')
        setSessionCookies('')
        setSessionExpiresAt('')
        await loadSessions()
      }
    } finally {
      setSessionLoading(false)
    }
  }

  const handleSessionDelete = async (siteKey: string) => {
    const res = await fetch(`/api/admin/scraper-sessions/${encodeURIComponent(siteKey)}`, {
      method: 'DELETE',
      headers: authService.getAdminFetchHeaders(),
    })
    if (res.ok) {
      await loadSessions()
    }
  }

  useEffect(() => {
    loadSources()
    loadRuns()
    loadSessions()
  }, [])

  const handleCreate = async () => {
    setError('')
    setLoading(true)
    const payload = {
      name,
      target_type: targetType,
      source_type: sourceType,
      source_url: sourceUrl,
      schedule_type: scheduleType,
      schedule_day: scheduleDay,
      schedule_time: scheduleTime,
    }
    const response = await fetch('/api/admin/crawl-sources', {
      method: 'POST',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    })
    const data = await response.json()
    if (!response.ok) {
      setError(data?.error || 'スケジュール作成に失敗しました')
      setLoading(false)
      return
    }
    setName('')
    setTargetType('job_site_company')
    setSourceUrl('')
    setScheduleType('weekly')
    setScheduleDay(1)
    setScheduleTime('09:00')
    await loadSources()
    setLoading(false)
  }

  const handleToggleActive = async (source: CrawlSource) => {
    const response = await fetch(`/api/admin/crawl-sources/${source.id}`, {
      method: 'PUT',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({ is_active: !source.is_active }),
    })
    if (response.ok) {
      await loadSources()
    }
  }

  const handleRun = async (source: CrawlSource) => {
    const response = await fetch(`/api/admin/crawl-sources/${source.id}/run`, {
      method: 'POST',
      headers: authService.getAdminFetchHeaders(),
    })
    if (response.ok) {
      await loadSources()
      await loadRuns()
    }
  }

  const handleGraphCrawl = async () => {
    setGraphLoading(true)
    setGraphResult(null)
    const response = await fetch('/api/admin/company-graph-crawl', {
      method: 'POST',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        sites: graphSites,
        query: graphQuery,
        pages: graphPages,
        year: graphYear ? Number(graphYear) : undefined,
      }),
    })
    const data = await response.json()
    setGraphResult(data)
    setGraphLoading(false)
  }

  const handleGbizSearch = async () => {
    if (!gbizSearchName.trim()) return
    setGbizSearchLoading(true)
    setGbizSearchError('')
    setGbizSearchResults([])
    setGbizRegisterMessage('')
    const response = await fetch(
      `/api/admin/companies/search-gbiz?name=${encodeURIComponent(gbizSearchName)}`,
      { headers: authService.getAdminFetchHeaders() },
    )
    const data = await response.json()
    if (!response.ok) {
      setGbizSearchError(data?.error || '検索に失敗しました')
    } else {
      setGbizSearchResults(data?.results || [])
      if ((data?.results || []).length === 0) {
        setGbizSearchError('該当する企業が見つかりませんでした')
      }
    }
    setGbizSearchLoading(false)
  }

  const handleGbizRegister = async (result: { corporate_number: string; name: string; location: string; company_url: string; employee_number: number }) => {
    setGbizRegisterLoading(result.corporate_number)
    setGbizRegisterMessage('')
    const response = await fetch('/api/admin/companies', {
      method: 'POST',
      headers: { ...authService.getAdminFetchHeaders(), 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: result.name,
        location: result.location,
        website_url: result.company_url,
        corporate_number: result.corporate_number,
        employee_count: result.employee_number,
        source_type: 'gbizinfo',
      }),
    })
    const data = await response.json()
    if (!response.ok) {
      setGbizRegisterMessage(`登録失敗: ${data?.error || 'エラーが発生しました'}`)
    } else {
      setGbizRegisterMessage(`「${result.name}」を登録しました（ID: ${data?.id}）`)
    }
    setGbizRegisterLoading(null)
  }

  const toggleGraphSite = (key: string) => {
    setGraphSites((prev) =>
      prev.includes(key) ? prev.filter((s) => s !== key) : [...prev, key],
    )
  }

  const scheduleLabel = useMemo(() => {
    if (scheduleType === 'weekly') {
      const dayLabel = WEEKDAY_OPTIONS.find((d) => d.value === scheduleDay)?.label ?? '月'
      return `毎週${dayLabel} ${scheduleTime}`
    }
    return `毎月${scheduleDay}日 ${scheduleTime}`
  }, [scheduleType, scheduleDay, scheduleTime])

  return (
    <Box sx={{ p: 4, maxWidth: 1100, mx: 'auto' }}>
      <Typography variant="h4" fontWeight="bold" gutterBottom>
        自動クローリング管理
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        週次・月次で企業データを自動更新します。対象URLごとにスケジュールを設定してください。
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <Grid container spacing={2} sx={{ mb: 3 }}>
        <Grid item xs={12} md={5}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                新規スケジュール
              </Typography>
              <Stack spacing={2}>
                <TextField
                  select
                  label="対象タイプ"
                  value={targetType}
                  onChange={(e) =>
                    setTargetType(
                      e.target.value as 'company' | 'popular_companies' | 'job_site_company' | 'job_listing',
                    )
                  }
                >
                  <MenuItem value="job_site_company">会社情報（企業ページ）</MenuItem>
                  <MenuItem value="job_listing">求人情報（募集職種）</MenuItem>
                  <MenuItem value="popular_companies">人気企業一覧</MenuItem>
                  <MenuItem value="company">企業単体（名前のみ）</MenuItem>
                </TextField>
                {targetType === 'job_site_company' && (
                  <Typography variant="caption" color="text.secondary">
                    新卒求人サイトの企業詳細ページURLを指定すると、会社概要・業界・従業員数・企業文化・福利厚生などを自動取得します。
                  </Typography>
                )}
                {targetType === 'job_listing' && (
                  <Typography variant="caption" color="text.secondary">
                    新卒求人サイトの募集職種ページURLを指定すると、職種名・仕事内容・給与・勤務地・必要スキルなどを自動取得します。
                  </Typography>
                )}
                {targetType === 'popular_companies' && (
                  <Typography variant="caption" color="text.secondary">
                    人気企業ランキング等のページURLを指定すると、AIが会社名を一覧抽出してDBに登録します。
                  </Typography>
                )}
                <TextField
                  label={targetType === 'company' ? '企業名' : '設定名'}
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  required
                />
                <TextField
                  select
                  label="出典タイプ"
                  value={sourceType}
                  onChange={(e) => setSourceType(e.target.value)}
                >
                  <MenuItem value="official">公式サイト</MenuItem>
                  <MenuItem value="job_site">就活/転職サイト</MenuItem>
                  <MenuItem value="manual">手入力</MenuItem>
                </TextField>
                <TextField
                  label="出典URL"
                  value={sourceUrl}
                  onChange={(e) => setSourceUrl(e.target.value)}
                />
                <TextField
                  select
                  label="頻度"
                  value={scheduleType}
                  onChange={(e) => setScheduleType(e.target.value as 'weekly' | 'monthly')}
                >
                  <MenuItem value="weekly">毎週</MenuItem>
                  <MenuItem value="monthly">毎月</MenuItem>
                </TextField>
                {scheduleType === 'weekly' ? (
                  <TextField
                    select
                    label="曜日"
                    value={scheduleDay}
                    onChange={(e) => setScheduleDay(Number(e.target.value))}
                  >
                    {WEEKDAY_OPTIONS.map((option) => (
                      <MenuItem key={option.value} value={option.value}>
                        {option.label}
                      </MenuItem>
                    ))}
                  </TextField>
                ) : (
                  <TextField
                    type="number"
                    label="日付"
                    value={scheduleDay}
                    onChange={(e) => setScheduleDay(Number(e.target.value))}
                    inputProps={{ min: 1, max: 31 }}
                  />
                )}
                <TextField
                  type="time"
                  label="実行時刻"
                  value={scheduleTime}
                  onChange={(e) => setScheduleTime(e.target.value)}
                  InputLabelProps={{ shrink: true }}
                />
                <Chip label={scheduleLabel} size="small" sx={{ alignSelf: 'flex-start' }} />
                <Button variant="contained" onClick={handleCreate} disabled={loading}>
                  スケジュールを追加
                </Button>
              </Stack>
            </CardContent>
          </Card>
        </Grid>
        <Grid item xs={12} md={7}>
          <Card sx={{ height: '100%' }}>
            <CardContent>
              <Typography variant="h6" gutterBottom>
                スケジュール一覧
              </Typography>
              <Divider sx={{ mb: 2 }} />
              <Stack spacing={2}>
                {sources.length === 0 && (
                  <Typography variant="body2" color="text.secondary">
                    まだスケジュールが登録されていません。
                  </Typography>
                )}
                {sources.map((source) => (
                  <Box key={source.id} sx={{ border: '1px solid #eee', borderRadius: 1, p: 2 }}>
                    <Stack spacing={1}>
                      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <Typography variant="subtitle1" fontWeight="bold">
                          {source.name}
                        </Typography>
                        <Stack direction="row" spacing={1} alignItems="center">
                          <Chip
                            label={
                              source.target_type === 'company'
                                ? '企業単体'
                                : source.target_type === 'popular_companies'
                                  ? '人気企業一覧'
                                  : source.target_type === 'job_site_company'
                                    ? '会社情報'
                                    : '求人情報'
                            }
                            size="small"
                            variant="outlined"
                            color={
                              source.target_type === 'job_listing'
                                ? 'primary'
                                : source.target_type === 'job_site_company'
                                  ? 'secondary'
                                  : 'default'
                            }
                          />
                          <Chip
                            label={source.is_active ? '稼働中' : '停止中'}
                            size="small"
                            color={source.is_active ? 'success' : 'default'}
                          />
                        </Stack>
                      </Box>
                      <Typography variant="body2" color="text.secondary">
                        {source.schedule_type === 'weekly'
                          ? `毎週${WEEKDAY_OPTIONS.find((d) => d.value === source.schedule_day)?.label ?? '月'} ${source.schedule_time}`
                          : `毎月${source.schedule_day}日 ${source.schedule_time}`}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        次回: {source.next_run_at ? new Date(source.next_run_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' }) : '未設定'} / 前回: {source.last_run_at ? new Date(source.last_run_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' }) : '未実行'}
                      </Typography>
                      <Stack direction="row" spacing={1}>
                        <Button size="small" variant="outlined" onClick={() => handleRun(source)}>
                          今すぐ実行
                        </Button>
                        <Button size="small" variant="text" onClick={() => handleToggleActive(source)}>
                          {source.is_active ? '停止する' : '再開する'}
                        </Button>
                      </Stack>
                    </Stack>
                  </Box>
                ))}
              </Stack>
            </CardContent>
          </Card>
        </Grid>
      </Grid>

      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            最近の実行履歴
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={1}>
            {runs.length === 0 && (
              <Typography variant="body2" color="text.secondary">
                まだ実行履歴がありません。
              </Typography>
            )}
            {runs.map((run) => (
              <Box key={run.id} sx={{ display: 'flex', justifyContent: 'space-between' }}>
                <Typography variant="body2">
                  #{run.id} / source {run.source_id}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {run.status} · {new Date(run.started_at).toLocaleString('ja-JP', { timeZone: 'Asia/Tokyo' })}
                </Typography>
              </Box>
            ))}
          </Stack>
        </CardContent>
      </Card>

      {/* マルチソース企業グラフクロール */}
      <Card sx={{ mt: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            マルチソース企業グラフクロール（gBizINFO連携）
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            キャリタス就活から一括取得し、gBizINFO APIで法人番号に名寄せして
            企業関係グラフ（GraphML / JSON）を生成します。年度は自動計算されます（手動指定も可）。
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Grid container spacing={2}>
            <Grid item xs={12} md={6}>
              <Stack spacing={2}>
                <Box>
                  <Typography variant="body2" fontWeight="bold" sx={{ mb: 1 }}>
                    対象サイト
                  </Typography>
                  <FormGroup row>
                    {SITE_OPTIONS.map((s) => (
                      <FormControlLabel
                        key={s.key}
                        control={
                          <Checkbox
                            checked={graphSites.includes(s.key)}
                            onChange={() => toggleGraphSite(s.key)}
                            size="small"
                          />
                        }
                        label={s.label}
                      />
                    ))}
                  </FormGroup>
                </Box>
                <TextField
                  label="検索キーワード"
                  value={graphQuery}
                  onChange={(e) => setGraphQuery(e.target.value)}
                  size="small"
                  placeholder="例: IT, 製造, 金融"
                />
                <TextField
                  label="最大ページ数（サイトごと）"
                  type="number"
                  value={graphPages}
                  onChange={(e) => setGraphPages(Number(e.target.value))}
                  size="small"
                  inputProps={{ min: 1, max: 20 }}
                />
                <TextField
                  label={`年度指定（省略時は自動: ${autoYear ?? '計算中'}年度）`}
                  type="number"
                  value={graphYear}
                  onChange={(e) => setGraphYear(e.target.value)}
                  size="small"
                  placeholder={String(autoYear ?? '')}
                  helperText="4月以降は当年+2、3月以前は当年+1を自動適用"
                />
                <Button
                  variant="contained"
                  onClick={handleGraphCrawl}
                  disabled={graphLoading || graphSites.length === 0}
                >
                  {graphLoading ? 'クロール実行中...' : 'クロール実行'}
                </Button>
              </Stack>
            </Grid>
            <Grid item xs={12} md={6}>
              {graphResult && (
                <Box>
                  {graphResult.ok ? (
                    <Alert severity="success" sx={{ mb: 1 }}>
                      完了しました。GraphML / JSON を出力しました。
                    </Alert>
                  ) : (
                    <Alert severity="error" sx={{ mb: 1 }}>
                      {graphResult.error || 'エラーが発生しました'}
                    </Alert>
                  )}
                  <Box
                    component="pre"
                    sx={{
                      fontSize: 11,
                      bgcolor: '#f5f5f5',
                      p: 1.5,
                      borderRadius: 1,
                      maxHeight: 300,
                      overflow: 'auto',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-all',
                    }}
                  >
                    {graphResult.logs}
                  </Box>
                </Box>
              )}
              {!graphResult && !graphLoading && (
                <Box sx={{ color: 'text.secondary', fontSize: 14, pt: 1 }}>
                  実行するとここにログが表示されます。
                </Box>
              )}
              {graphLoading && (
                <Box sx={{ color: 'text.secondary', fontSize: 14, pt: 1 }}>
                  クロール中です。完了まで数分かかる場合があります...
                </Box>
              )}
            </Grid>
          </Grid>
        </CardContent>
      </Card>

      {/* gBizINFO企業名検索・登録 */}
      <Card sx={{ mt: 3 }}>
        <CardContent>
          <Typography variant="h6" gutterBottom>
            企業名で検索して登録（gBizINFO）
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            企業名を入力するとgBizINFO APIで法人情報を検索し、ホームページURLを含む企業データをDBに登録できます。
          </Typography>
          <Divider sx={{ mb: 2 }} />
          <Stack spacing={2} sx={{ maxWidth: 600 }}>
            <Stack direction="row" spacing={1}>
              <TextField
                label="企業名"
                value={gbizSearchName}
                onChange={(e) => setGbizSearchName(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') handleGbizSearch() }}
                size="small"
                placeholder="例: 株式会社ほげ"
                sx={{ flex: 1 }}
              />
              <Button
                variant="contained"
                onClick={handleGbizSearch}
                disabled={gbizSearchLoading || !gbizSearchName.trim()}
              >
                {gbizSearchLoading ? '検索中...' : '検索'}
              </Button>
            </Stack>
            {gbizSearchError && (
              <Alert severity="warning">{gbizSearchError}</Alert>
            )}
            {gbizRegisterMessage && (
              <Alert severity={gbizRegisterMessage.startsWith('登録失敗') ? 'error' : 'success'}>
                {gbizRegisterMessage}
              </Alert>
            )}
            {gbizSearchResults.map((r) => (
              <Box key={r.corporate_number} sx={{ border: '1px solid #eee', borderRadius: 1, p: 2 }}>
                <Stack spacing={0.5}>
                  <Typography variant="subtitle1" fontWeight="bold">{r.name}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    法人番号: {r.corporate_number}
                  </Typography>
                  {r.location && (
                    <Typography variant="body2" color="text.secondary">所在地: {r.location}</Typography>
                  )}
                  {r.company_url && (
                    <Typography variant="body2">
                      HP: <a href={r.company_url} target="_blank" rel="noopener noreferrer">{r.company_url}</a>
                    </Typography>
                  )}
                  {r.employee_number > 0 && (
                    <Typography variant="body2" color="text.secondary">従業員数: {r.employee_number}人</Typography>
                  )}
                  <Button
                    size="small"
                    variant="outlined"
                    onClick={() => handleGbizRegister(r)}
                    disabled={gbizRegisterLoading === r.corporate_number}
                    sx={{ alignSelf: 'flex-start', mt: 0.5 }}
                  >
                    {gbizRegisterLoading === r.corporate_number ? '登録中...' : 'DBに登録'}
                  </Button>
                </Stack>
              </Box>
            ))}
          </Stack>
        </CardContent>
      </Card>

      {/* ── スクレイパーセッション管理 ── */}
      <Card>
        <CardContent>
          <Typography variant="h6" gutterBottom>スクレイパーセッション管理</Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            ログイン必須のスクレイピングサイト（マイナビ等）のCookieを登録します。
            ブラウザのDevTools → Application → Cookies からコピーしてください。
          </Typography>

          {sessionError && <Alert severity="error" sx={{ mb: 2 }}>{sessionError}</Alert>}
          {sessionSuccess && <Alert severity="success" sx={{ mb: 2 }}>{sessionSuccess}</Alert>}

          {/* 登録済みセッション一覧 */}
          {sessions.length > 0 && (
            <Box sx={{ mb: 3 }}>
              <Typography variant="subtitle2" gutterBottom>登録済みセッション</Typography>
              <Stack spacing={1}>
                {sessions.map((s) => {
                  const expired = s.expires_at ? new Date(s.expires_at) < new Date() : false
                  return (
                    <Box key={s.site_key} sx={{ display: 'flex', alignItems: 'center', gap: 2, p: 1, border: '1px solid', borderColor: expired ? 'error.light' : 'divider', borderRadius: 1 }}>
                      <Chip label={s.site_key} size="small" color={expired ? 'error' : 'success'} />
                      <Typography variant="body2" sx={{ flex: 1 }}>
                        更新日: {new Date(s.updated_at).toLocaleDateString('ja-JP')}
                        {s.expires_at && (
                          <span style={{ marginLeft: 8, color: expired ? 'red' : 'inherit' }}>
                            {expired ? '⚠️ 期限切れ' : `有効期限: ${new Date(s.expires_at).toLocaleDateString('ja-JP')}`}
                          </span>
                        )}
                      </Typography>
                      <Button
                        size="small"
                        color="error"
                        variant="outlined"
                        onClick={() => handleSessionDelete(s.site_key)}
                      >
                        削除
                      </Button>
                    </Box>
                  )
                })}
              </Stack>
            </Box>
          )}

          {/* 登録フォーム */}
          <Stack spacing={2} sx={{ maxWidth: 600 }}>
            <TextField
              select
              label="サイト"
              value={sessionSiteKey}
              onChange={(e) => setSessionSiteKey(e.target.value)}
              size="small"
            >
              <MenuItem value="mynavi">マイナビ</MenuItem>
              <MenuItem value="rikunabi">リクナビ</MenuItem>
              <MenuItem value="openwork">OpenWork</MenuItem>
              <MenuItem value="other">その他</MenuItem>
            </TextField>
            <TextField
              label="Cookie文字列"
              multiline
              rows={3}
              value={sessionCookies}
              onChange={(e) => setSessionCookies(e.target.value)}
              placeholder="JSESSIONID=abc123; session_key=xyz（またはJSON配列）"
              size="small"
            />
            <TextField
              label="有効期限（任意）"
              type="datetime-local"
              value={sessionExpiresAt}
              onChange={(e) => setSessionExpiresAt(e.target.value)}
              size="small"
              InputLabelProps={{ shrink: true }}
            />
            <Button
              variant="contained"
              onClick={handleSessionUpsert}
              disabled={sessionLoading || !sessionCookies.trim()}
              sx={{ alignSelf: 'flex-start' }}
            >
              {sessionLoading ? '登録中...' : '登録・更新'}
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
