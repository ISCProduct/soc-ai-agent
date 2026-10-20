'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { authService } from '@/lib/auth'

type BudgetScope = 'user' | 'school'

type BudgetSetting = {
  id: number
  scope: BudgetScope
  scope_id: number
  daily_limit_usd: number
  monthly_limit_usd: number
}

type BudgetListResponse = {
  settings?: BudgetSetting[]
  total?: number
  limit?: number
  offset?: number
  error?: string
}
type BudgetActionResponse = { error?: string }
const PAGE_LIMIT = 50

async function readBudgetResponse<T extends BudgetListResponse | BudgetActionResponse>(
  response: Response,
): Promise<T> {
  const raw = await response.text()
  if (!raw.trim() && response.ok) return {} as T
  let data: T
  try {
    data = JSON.parse(raw) as T
  } catch {
    throw new Error(raw || `リクエストに失敗しました (${response.status})`)
  }
  if (!response.ok) {
    throw new Error(data.error || `リクエストに失敗しました (${response.status})`)
  }
  return data
}

function limitLabel(value: number): string {
  return value > 0 ? `$${value.toFixed(2)}` : '上限なし'
}

export function AIBudgetSettings() {
  const [settings, setSettings] = useState<BudgetSetting[]>([])
  const [total, setTotal] = useState(0)
  const [offset, setOffset] = useState(0)
  const [scope, setScope] = useState<BudgetScope>('user')
  const [scopeID, setScopeID] = useState('')
  const [dailyLimit, setDailyLimit] = useState('')
  const [monthlyLimit, setMonthlyLimit] = useState('')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [message, setMessage] = useState('')
  const requestID = useRef(0)

  const fetchSettings = useCallback(async (requestedOffset = 0) => {
    const currentRequestID = ++requestID.current
    setLoading(true)
    setError('')
    try {
      const user = authService.getStoredUser()
      const params = new URLSearchParams({ limit: String(PAGE_LIMIT), offset: String(requestedOffset) })
      const response = await fetch(`/api/admin/ai-budgets?${params.toString()}`, {
        headers: {
          'X-Admin-Email': user?.email ?? '',
          'X-Admin-Token': authService.getStoredToken() || '',
        },
        cache: 'no-store',
      })
      const data = await readBudgetResponse<BudgetListResponse>(response)
      if (currentRequestID === requestID.current) {
        setSettings(data.settings ?? [])
        setTotal(data.total ?? 0)
        setOffset(data.offset ?? requestedOffset)
      }
    } catch (err: unknown) {
      if (currentRequestID === requestID.current) {
        setError(err instanceof Error ? err.message : '予算設定を取得できませんでした')
      }
    } finally {
      if (currentRequestID === requestID.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void fetchSettings(0)
  }, [fetchSettings])

  const saveSetting = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const id = Number(scopeID)
    const daily = Number(dailyLimit)
    const monthly = Number(monthlyLimit)
    const validLimit = (value: string, numeric: number) =>
      /^\d{1,6}(?:\.\d{1,6})?$/.test(value.trim()) && numeric <= 999999.999999
    if (!Number.isSafeInteger(id) || id <= 0 || !Number.isFinite(daily) || daily < 0 ||
      !Number.isFinite(monthly) || monthly < 0 ||
      !validLimit(dailyLimit, daily) || !validLimit(monthlyLimit, monthly)) {
      setError('対象IDは正の整数、上限額は0〜999,999.999999の範囲で小数6桁以内にしてください')
      return
    }

    setSaving(true)
    setError('')
    setMessage('')
    try {
      const user = authService.getStoredUser()
      const response = await fetch('/api/admin/ai-budgets', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          'X-Admin-Email': user?.email ?? '',
          'X-Admin-Token': authService.getStoredToken() || '',
        },
        body: JSON.stringify({
          scope,
          scope_id: id,
          daily_limit_usd: daily,
          monthly_limit_usd: monthly,
        }),
      })
      await readBudgetResponse<BudgetActionResponse>(response)
      setMessage('AI利用上限を保存しました')
      await fetchSettings(offset)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'AI利用上限を保存できませんでした')
    } finally {
      setSaving(false)
    }
  }

  const deleteSetting = async (setting: BudgetSetting) => {
    if (!window.confirm('このAI利用上限を解除しますか？')) return
    setError('')
    setMessage('')
    try {
      const user = authService.getStoredUser()
      const params = new URLSearchParams({
        scope: setting.scope,
        scope_id: String(setting.scope_id),
      })
      const response = await fetch(`/api/admin/ai-budgets?${params}`, {
        method: 'DELETE',
        headers: {
          'X-Admin-Email': user?.email ?? '',
          'X-Admin-Token': authService.getStoredToken() || '',
        },
      })
      await readBudgetResponse<BudgetActionResponse>(response)
      setMessage('AI利用上限を解除しました')
      await fetchSettings(settings.length === 1 && offset > 0 ? Math.max(0, offset - PAGE_LIMIT) : offset)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'AI利用上限を解除できませんでした')
    }
  }

  return (
    <Card className="mb-6">
      <CardHeader>
        <CardTitle className="text-base">AI利用予算ブレーカー</CardTitle>
        <p className="text-sm text-muted-foreground">
          0ドルは上限なしです。利用額が日次または月次上限に達すると、新しいAIリクエストを停止します。
        </p>
      </CardHeader>
      <CardContent className="space-y-4">
        {error && (
          <p role="alert" className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </p>
        )}
        {message && (
          <p role="status" className="rounded-md border px-3 py-2 text-sm">
            {message}
          </p>
        )}

        <form onSubmit={saveSetting} className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          <label className="grid gap-1 text-sm">
            対象
            <select
              value={scope}
              onChange={(event) => setScope(event.target.value as BudgetScope)}
              className="h-10 rounded-md border bg-background px-3"
            >
              <option value="user">利用者</option>
              <option value="school">学校</option>
            </select>
          </label>
          <label className="grid gap-1 text-sm">
            {scope === 'user' ? '利用者ID' : '学校ID'}
            <input
              type="number"
              min="1"
              step="1"
              required
              value={scopeID}
              onChange={(event) => setScopeID(event.target.value)}
              className="h-10 rounded-md border bg-background px-3"
            />
          </label>
          <label className="grid gap-1 text-sm">
            日次上限 (USD)
            <input
              type="number"
              min="0"
              step="0.000001"
              max="999999.999999"
              required
              value={dailyLimit}
              onChange={(event) => setDailyLimit(event.target.value)}
              className="h-10 rounded-md border bg-background px-3"
            />
          </label>
          <label className="grid gap-1 text-sm">
            月次上限 (USD)
            <input
              type="number"
              min="0"
              step="0.000001"
              max="999999.999999"
              required
              value={monthlyLimit}
              onChange={(event) => setMonthlyLimit(event.target.value)}
              className="h-10 rounded-md border bg-background px-3"
            />
          </label>
          <div className="flex items-end">
            <Button type="submit" disabled={saving} className="w-full">
              {saving ? '保存中...' : '上限を保存'}
            </Button>
          </div>
        </form>

        <div className="overflow-x-auto rounded-md border">
          <table className="w-full min-w-[540px] text-sm">
            <thead className="bg-muted/60 text-left">
              <tr>
                <th className="px-3 py-2 font-medium">対象</th>
                <th className="px-3 py-2 font-medium">ID</th>
                <th className="px-3 py-2 font-medium">日次</th>
                <th className="px-3 py-2 font-medium">月次</th>
                <th className="px-3 py-2 font-medium">操作</th>
              </tr>
            </thead>
            <tbody>
              {settings.map((setting) => (
                <tr key={`${setting.scope}-${setting.scope_id}`} className="border-t">
                  <td className="px-3 py-2">{setting.scope === 'user' ? '利用者' : '学校'}</td>
                  <td className="px-3 py-2">{setting.scope_id}</td>
                  <td className="px-3 py-2">{limitLabel(setting.daily_limit_usd)}</td>
                  <td className="px-3 py-2">{limitLabel(setting.monthly_limit_usd)}</td>
                  <td className="px-3 py-2">
                    <button
                      type="button"
                      onClick={() => void deleteSetting(setting)}
                      className="text-destructive underline underline-offset-2"
                    >
                      解除
                    </button>
                  </td>
                </tr>
              ))}
              {!loading && settings.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-3 py-5 text-center text-muted-foreground">
                    予算上限は設定されていません
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
        {total > PAGE_LIMIT && (
          <div className="flex items-center justify-between gap-3 text-sm">
            <span aria-live="polite">
              {offset + 1}〜{Math.min(offset + settings.length, total)}件 / {total}件
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                onClick={() => void fetchSettings(Math.max(0, offset - PAGE_LIMIT))}
                disabled={loading || offset === 0}
                className="rounded-md border px-3 py-1 disabled:opacity-50"
                aria-label="前の予算設定ページ"
              >
                前へ
              </button>
              <button
                type="button"
                onClick={() => void fetchSettings(offset + PAGE_LIMIT)}
                disabled={loading || offset + PAGE_LIMIT >= total}
                className="rounded-md border px-3 py-1 disabled:opacity-50"
                aria-label="次の予算設定ページ"
              >
                次へ
              </button>
            </div>
          </div>
        )}
        {loading && <p role="status" className="text-sm text-muted-foreground">設定を読み込んでいます...</p>}
        {!loading && error && (
          <button type="button" onClick={() => void fetchSettings()} className="text-sm underline">
            設定を再読み込み
          </button>
        )}
      </CardContent>
    </Card>
  )
}
