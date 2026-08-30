'use client'

import { useCallback, useEffect, useState } from 'react'
import Link from 'next/link'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { ArrowLeft, Info } from 'lucide-react'
import { authService } from '@/lib/auth'
import { cn } from '@/lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

type DailyRow = {
  date: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type MonthlyRow = {
  month: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type ModelRow = {
  model: string
  total_cost_usd: number
  total_tokens: number
  call_count: number
}

type Summary = {
  current_month_cost_usd: number
  model_breakdown: ModelRow[]
  company_search?: {
    month: string
    count: number
    limit: number
    remaining: number
    enforce: boolean
    exceeded: boolean
  }
  realtime?: {
    current_month_cost_usd: number
    active_connections: number
    user_breakdown: RealtimeUserRow[]
  }
}

type RealtimeDailyRow = {
  date: string
  total_cost_usd: number
  total_duration_seconds: number
  session_count: number
  user_count: number
  total_input_audio_tokens: number
  total_output_audio_tokens: number
  total_input_text_tokens: number
  total_output_text_tokens: number
}

type RealtimeUserRow = {
  user_id: number
  total_cost_usd: number
  total_duration_seconds: number
  session_count: number
  total_input_audio_tokens: number
  total_output_audio_tokens: number
  total_input_text_tokens: number
  total_output_text_tokens: number
}

const DAY_RANGES = [7, 30, 90] as const

async function readCostJson(res: Response): Promise<Record<string, unknown>> {
  const text = await res.text()
  if (!text.trim()) {
    throw new Error(
      res.ok ? 'サーバーから空の応答が返りました' : `コストの取得に失敗しました (${res.status})`,
    )
  }
  try {
    return JSON.parse(text) as Record<string, unknown>
  } catch {
    throw new Error(`コストの取得に失敗しました (${res.status})`)
  }
}

function costErrorMessage(data: Record<string, unknown>): string {
  const err = data.error ?? data.message
  return typeof err === 'string' && err ? err : ''
}

function Spinner({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        'size-4 animate-spin rounded-full border-2 border-current border-t-transparent',
        className,
      )}
    />
  )
}

function KpiCard({
  label,
  value,
  valueClassName,
  caption,
}: {
  label: string
  value: React.ReactNode
  valueClassName?: string
  caption?: React.ReactNode
}) {
  return (
    <Card className="flex-1 min-w-[200px]">
      <CardContent className="px-6">
        <p className="text-sm text-muted-foreground">{label}</p>
        <p className={cn('text-3xl font-bold mt-1', valueClassName)}>{value}</p>
        {caption ? <p className="text-xs text-muted-foreground mt-1">{caption}</p> : null}
      </CardContent>
    </Card>
  )
}

function CostBarChart({
  data,
  valueKey,
  labelKey,
}: {
  data: Record<string, unknown>[]
  valueKey: string
  labelKey: string
}) {
  if (data.length === 0) {
    return <p className="text-center text-sm text-muted-foreground py-8">データなし</p>
  }
  return (
    <div className="h-56 mt-2">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
          <CartesianGrid strokeDasharray="3 3" className="stroke-border" vertical={false} />
          <XAxis
            dataKey={labelKey}
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={{ className: 'stroke-border' } as never}
          />
          <YAxis
            tick={{ fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            width={64}
            tickFormatter={(v: number) => `$${v.toFixed(2)}`}
          />
          <Tooltip
            formatter={(value: number) => [`$${value.toFixed(4)}`, 'コスト']}
            contentStyle={{ fontSize: 12, borderRadius: 8 }}
          />
          <Bar dataKey={valueKey} fill="var(--chart-1)" radius={[3, 3, 0, 0]} />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}

export default function PageContent() {
  const [adminEmail, setAdminEmail] = useState('')
  const [summary, setSummary] = useState<Summary | null>(null)
  const [daily, setDaily] = useState<DailyRow[]>([])
  const [monthly, setMonthly] = useState<MonthlyRow[]>([])
  const [realtimeDaily, setRealtimeDaily] = useState<RealtimeDailyRow[]>([])
  const [dailyDays, setDailyDays] = useState<(typeof DAY_RANGES)[number]>(30)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    const user = authService.getStoredUser()
    if (!user?.is_admin) {
      window.location.href = '/'
      return
    }
    setAdminEmail(user.email)
  }, [])

  const fetchAll = useCallback(async () => {
    if (!adminEmail) return
    setLoading(true)
    setError('')
    const h = { 'X-Admin-Email': adminEmail, 'X-Admin-Token': authService.getStoredToken() || '' }
    try {
      const [sumRes, dailyRes, monthlyRes] = await Promise.all([
        fetch('/api/admin/costs', { headers: h }),
        fetch(`/api/admin/costs/daily?days=${dailyDays}`, { headers: h }),
        fetch('/api/admin/costs/monthly?months=12', { headers: h }),
      ])
      const [sumData, dailyData, monthlyData] = await Promise.all([
        readCostJson(sumRes),
        readCostJson(dailyRes),
        readCostJson(monthlyRes),
      ])
      const fail = costErrorMessage(sumData) || costErrorMessage(dailyData) || costErrorMessage(monthlyData)
      if (fail) {
        setError(fail)
        return
      }
      setSummary(sumData as Summary)
      setDaily((dailyData.daily as DailyRow[] | undefined) ?? [])
      setMonthly((monthlyData.monthly as MonthlyRow[] | undefined) ?? [])
      setRealtimeDaily((dailyData.realtime_daily as RealtimeDailyRow[] | undefined) ?? [])
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : '取得に失敗しました')
    } finally {
      setLoading(false)
    }
  }, [adminEmail, dailyDays])

  useEffect(() => { fetchAll() }, [fetchAll])

  const totalDailyCost = daily.reduce((s, r) => s + r.total_cost_usd, 0)
  const maxDailyModel = summary?.model_breakdown?.[0]?.total_cost_usd ?? 0.0001
  const realtimeDailyCost = realtimeDaily.reduce((s, r) => s + r.total_cost_usd, 0)

  return (
    <div className="max-w-[1100px] mx-auto w-full p-4 sm:p-6 md:p-8">
      <div className="flex flex-col sm:flex-row sm:items-start justify-between gap-2 mb-6">
        <div className="flex items-center gap-1.5">
          <Link
            href="/admin"
            aria-label="戻る"
            className="inline-flex size-8 items-center justify-center rounded-md hover:bg-accent text-foreground"
          >
            <ArrowLeft className="size-5" />
          </Link>
          <h1 className="text-2xl font-bold tracking-tight">APIコストモニタリング</h1>
        </div>
        {loading && <Spinner className="text-muted-foreground mt-1" />}
      </div>

      {error && (
        <div className="mb-4 rounded-md border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive flex items-center justify-between">
          <span>{error}</span>
          <button type="button" onClick={() => setError('')} className="text-xs underline">
            閉じる
          </button>
        </div>
      )}

      {/* KPI Cards */}
      <div className="flex flex-wrap gap-4 mb-6">
        <KpiCard
          label="今月の合計コスト"
          value={`$${(summary?.current_month_cost_usd ?? 0).toFixed(4)}`}
          valueClassName={
            (summary?.current_month_cost_usd ?? 0) > 50
              ? 'text-destructive'
              : (summary?.current_month_cost_usd ?? 0) > 20
                ? 'text-amber-600'
                : 'text-emerald-600'
          }
        />
        <KpiCard label={`過去${dailyDays}日合計コスト`} value={`$${totalDailyCost.toFixed(4)}`} />
        <KpiCard
          label={`過去${dailyDays}日 APIコール数`}
          value={daily.reduce((s, r) => s + r.call_count, 0).toLocaleString()}
        />
        <KpiCard
          label="Realtime 今月コスト"
          value={`$${(summary?.realtime?.current_month_cost_usd ?? 0).toFixed(4)}`}
          caption={`active: ${summary?.realtime?.active_connections ?? 0}`}
        />
        <KpiCard
          label="企業 Search 今月"
          value={
            <>
              {(summary?.company_search?.count ?? 0).toLocaleString()}
              <span className="text-base text-muted-foreground font-normal">
                {' '}/ {(summary?.company_search?.limit ?? 2000).toLocaleString()}
              </span>
            </>
          }
          valueClassName={
            summary?.company_search?.exceeded
              ? 'text-destructive'
              : (summary?.company_search?.remaining ?? 1) < 200
                ? 'text-amber-600'
                : 'text-emerald-600'
          }
          caption={`残 ${summary?.company_search?.remaining ?? '—'}${summary?.company_search?.enforce === false ? ' (observe)' : ''}`}
        />
      </div>

      {/* Daily cost chart */}
      <Card className="mb-6">
        <CardHeader className="flex-row items-center justify-between space-y-0">
          <CardTitle className="text-base">日次コスト推移</CardTitle>
          <div className="inline-flex rounded-md border p-0.5">
            {DAY_RANGES.map((d) => (
              <button
                key={d}
                type="button"
                onClick={() => setDailyDays(d)}
                className={cn(
                  'px-3 py-1 text-xs rounded-sm font-medium transition-colors',
                  dailyDays === d
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent',
                )}
              >
                {d}日
              </button>
            ))}
          </div>
        </CardHeader>
        <CardContent>
          <CostBarChart data={daily} valueKey="total_cost_usd" labelKey="date" />
          <div className="mt-4 max-h-[300px] overflow-y-auto rounded-md border">
            <Table>
              <TableHeader className="sticky top-0 bg-muted/80 backdrop-blur">
                <TableRow>
                  <TableHead>日付</TableHead>
                  <TableHead className="text-right">コスト (USD)</TableHead>
                  <TableHead className="text-right">トークン数</TableHead>
                  <TableHead className="text-right">コール数</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {[...daily].reverse().map((row) => (
                  <TableRow key={row.date}>
                    <TableCell>{row.date}</TableCell>
                    <TableCell className="text-right">${row.total_cost_usd.toFixed(6)}</TableCell>
                    <TableCell className="text-right">{row.total_tokens.toLocaleString()}</TableCell>
                    <TableCell className="text-right">{row.call_count.toLocaleString()}</TableCell>
                  </TableRow>
                ))}
                {daily.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-6">
                      データなし
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Monthly chart */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">月次コスト推移（過去12ヶ月）</CardTitle>
        </CardHeader>
        <CardContent>
          <CostBarChart data={monthly} valueKey="total_cost_usd" labelKey="month" />
          <div className="mt-4 overflow-y-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>月</TableHead>
                  <TableHead className="text-right">コスト (USD)</TableHead>
                  <TableHead className="text-right">トークン数</TableHead>
                  <TableHead className="text-right">コール数</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {[...monthly].reverse().map((row) => (
                  <TableRow key={row.month}>
                    <TableCell>{row.month}</TableCell>
                    <TableCell className="text-right">${row.total_cost_usd.toFixed(4)}</TableCell>
                    <TableCell className="text-right">{row.total_tokens.toLocaleString()}</TableCell>
                    <TableCell className="text-right">{row.call_count.toLocaleString()}</TableCell>
                  </TableRow>
                ))}
                {monthly.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-center text-muted-foreground py-6">
                      データなし
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Realtime usage */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base flex items-center gap-1.5">
            Realtime 利用状況
            <span
              title="コスト計算方法: トークン使用量が記録されている場合はトークンベース（音声入力 $100/1M・出力 $200/1M、テキスト入力 $5/1M・出力 $15/1M）、記録がない場合は時間ベース（INTERVIEW_COST_PER_MIN_USD × 利用分数）で算出します。"
              className="text-muted-foreground cursor-help"
            >
              <Info className="size-4" />
            </span>
          </CardTitle>
          <p className="text-sm text-muted-foreground">
            過去{dailyDays}日合計: ${realtimeDailyCost.toFixed(4)}
          </p>
        </CardHeader>
        <CardContent>
          <CostBarChart data={realtimeDaily} valueKey="total_cost_usd" labelKey="date" />
          <div className="mt-4 max-h-[280px] overflow-y-auto rounded-md border">
            <Table>
              <TableHeader className="sticky top-0 bg-muted/80 backdrop-blur">
                <TableRow>
                  <TableHead>日付</TableHead>
                  <TableHead className="text-right">コスト (USD)</TableHead>
                  <TableHead className="text-right">時間 (分)</TableHead>
                  <TableHead className="text-right">セッション数</TableHead>
                  <TableHead className="text-right">利用ユーザー数</TableHead>
                  <TableHead className="text-right">音声入力 (tok)</TableHead>
                  <TableHead className="text-right">音声出力 (tok)</TableHead>
                  <TableHead className="text-right">テキスト入力 (tok)</TableHead>
                  <TableHead className="text-right">テキスト出力 (tok)</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {[...realtimeDaily].reverse().map((row) => (
                  <TableRow key={row.date}>
                    <TableCell>{row.date}</TableCell>
                    <TableCell className="text-right">${row.total_cost_usd.toFixed(4)}</TableCell>
                    <TableCell className="text-right">{(row.total_duration_seconds / 60).toFixed(1)}</TableCell>
                    <TableCell className="text-right">{row.session_count.toLocaleString()}</TableCell>
                    <TableCell className="text-right">{row.user_count.toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_input_audio_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_output_audio_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_input_text_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_output_text_tokens ?? 0).toLocaleString()}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Realtime users */}
      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">Realtime ユーザー別利用（過去30日）</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-y-auto rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>User ID</TableHead>
                  <TableHead className="text-right">コスト (USD)</TableHead>
                  <TableHead className="text-right">時間 (分)</TableHead>
                  <TableHead className="text-right">セッション数</TableHead>
                  <TableHead className="text-right">音声入力 (tok)</TableHead>
                  <TableHead className="text-right">音声出力 (tok)</TableHead>
                  <TableHead className="text-right">テキスト入力 (tok)</TableHead>
                  <TableHead className="text-right">テキスト出力 (tok)</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(summary?.realtime?.user_breakdown ?? []).map((row) => (
                  <TableRow key={row.user_id}>
                    <TableCell>{row.user_id}</TableCell>
                    <TableCell className="text-right">${row.total_cost_usd.toFixed(4)}</TableCell>
                    <TableCell className="text-right">{(row.total_duration_seconds / 60).toFixed(1)}</TableCell>
                    <TableCell className="text-right">{row.session_count.toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_input_audio_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_output_audio_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_input_text_tokens ?? 0).toLocaleString()}</TableCell>
                    <TableCell className="text-right">{(row.total_output_text_tokens ?? 0).toLocaleString()}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      {/* Model breakdown */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">モデル別コスト内訳（過去30日）</CardTitle>
        </CardHeader>
        <CardContent>
          {(summary?.model_breakdown ?? []).length === 0 ? (
            <p className="text-center text-muted-foreground py-6">データなし</p>
          ) : (
            <div className="flex flex-col gap-3">
              {(summary?.model_breakdown ?? []).map((row) => (
                <div key={row.model}>
                  <div className="flex items-center gap-2 mb-1">
                    <Badge variant="outline">{row.model}</Badge>
                    <span className="text-xs text-muted-foreground">
                      {row.call_count.toLocaleString()} calls / {row.total_tokens.toLocaleString()} tokens
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <Progress
                      value={Math.min(100, (row.total_cost_usd / maxDailyModel) * 100)}
                      className="flex-1"
                    />
                    <span className="text-sm w-16 text-right tabular-nums">
                      ${row.total_cost_usd.toFixed(4)}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
