'use client'

// 企業ごとの操作（公開・一括公開・審査・非公開・情報取得）とその他メニューの開閉を受け持つ。
import { useState, type Dispatch, type SetStateAction } from 'react'
import { authService } from '@/lib/auth'
import type { CompanyListReloadKind } from '../company-list-reload'
import {
  fetchCompanyPrimary,
  formatFetchPrimaryEmptyAspects,
  formatFetchPrimarySummary,
  hasActionableSoftEmpty,
} from '@/lib/admin/company-fetch'
import { canSelectForPublish, type Company } from '../company-list-model'

type Params = {
  companies: Company[]
  selectedIds: number[]
  setSelectedIds: Dispatch<SetStateAction<number[]>>
  reloadCurrentList: (kind: CompanyListReloadKind) => Promise<void>
  setError: Dispatch<SetStateAction<string>>
  setFetchMessage: Dispatch<SetStateAction<string>>
  setFetchSeverity: Dispatch<SetStateAction<'success' | 'warning'>>
}

export type CompanyMenuAnchor = { el: HTMLElement; company: Company }

export function useCompanyRowActions({
  companies,
  selectedIds,
  setSelectedIds,
  reloadCurrentList,
  setError,
  setFetchMessage,
  setFetchSeverity,
}: Params) {
  const [fetchPrimaryId, setFetchPrimaryId] = useState<number | null>(null)
  const [menuAnchor, setMenuAnchor] = useState<CompanyMenuAnchor | null>(null)
  const [bulkPublishing, setBulkPublishing] = useState(false)

  const publishCompany = async (companyId: number): Promise<boolean> => {
    const res = await fetch(`/api/admin/companies/${companyId}/publish`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    return res.ok
  }

  const handlePublish = async (companyId: number) => {
    setError('')
    setMenuAnchor(null)
    const ok = await publishCompany(companyId)
    if (!ok) {
      setError('公開に失敗しました')
      return
    }
    setFetchSeverity('success')
    setFetchMessage('企業を学生に公開しました。')
    setSelectedIds((prev) => prev.filter((id) => id !== companyId))
    await reloadCurrentList('status')
  }

  const handleBulkPublish = async () => {
    const targets = companies.filter((c) => selectedIds.includes(c.id) && canSelectForPublish(c))
    if (targets.length === 0) {
      setFetchSeverity('warning')
      setFetchMessage('公開できる企業（情報がそろった公開前）が選択されていません。')
      return
    }

    const ok = window.confirm(`選択した ${targets.length} 社を学生に公開しますか？`)
    if (!ok) return

    setBulkPublishing(true)
    setError('')
    setFetchMessage('')
    try {
      let success = 0
      let failed = 0
      for (const company of targets) {
        if (await publishCompany(company.id)) success += 1
        else failed += 1
      }
      setSelectedIds([])
      if (failed > 0) {
        setFetchSeverity('warning')
        setFetchMessage(`${success} 社を公開しました。${failed} 社は公開できませんでした。`)
        setError(`${failed} 社の公開に失敗しました。`)
      } else {
        setFetchSeverity('success')
        setFetchMessage(`${success} 社を学生に公開しました。`)
      }
      await reloadCurrentList('status')
    } finally {
      setBulkPublishing(false)
    }
  }

  const handleSetVerified = async (companyId: number, verified: boolean) => {
    setError('')
    setMenuAnchor(null)
    const res = await fetch(`/api/admin/companies/${companyId}/verify`, {
      method: 'PATCH',
      headers: {
        'Content-Type': 'application/json',
        ...authService.getAdminFetchHeaders(),
      },
      body: JSON.stringify({ verified }),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(data?.error || `学生検索の審査を更新できませんでした (${res.status})`)
      return
    }
    setFetchSeverity('success')
    setFetchMessage(
      verified
        ? '学生検索の審査を完了しました。この企業は学生の検索とスカウト送信ができます。'
        : '学生検索の審査を取り消しました。学生の検索とスカウト送信はできません。',
    )
    await reloadCurrentList('status')
  }

  const handleReject = async (companyId: number) => {
    setError('')
    setMenuAnchor(null)
    const res = await fetch(`/api/admin/companies/${companyId}/reject`, {
      method: 'PATCH',
      headers: authService.getAdminFetchHeaders(),
    })
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      setError(data?.error || `非公開への変更に失敗しました (${res.status})`)
      return
    }
    setFetchSeverity('success')
    setFetchMessage('企業を非公開にしました。学生には表示されません。')
    await reloadCurrentList('reject')
  }

  const handleFetchPrimary = async (companyId: number, force = false) => {
    setError('')
    setFetchMessage('')
    setFetchSeverity('success')
    setFetchPrimaryId(companyId)
    setMenuAnchor(null)
    const knownIndustry = companies.find((c) => c.id === companyId)?.industry
    try {
      const { ok, status, data } = await fetchCompanyPrimary(
        companyId,
        authService.getAdminFetchHeaders(),
        force,
      )
      if (!ok) {
        setError(data?.error || `企業情報の取得に失敗しました (${status})`)
        return
      }
      const steps = [data.info_step, data.tech_step, data.relations_step]
      const fetched = steps.filter((s) => s?.status === 'fetched').length
      const allSkipped = steps.every((s) => s?.status === 'skipped')
      const hardFailed = steps.some((s) => s?.status === 'error')
      const softEmpty = hasActionableSoftEmpty(data, knownIndustry)
      const budgetHit = steps.some((s) => s?.detail === 'budget') ||
        (Array.isArray(data.errors) && data.errors.some((e) => e.includes('budget')))
      const summary = formatFetchPrimarySummary(data, knownIndustry)

      if (budgetHit) {
        setFetchSeverity('warning')
        setError('月次の情報取得上限に達しているため、新しい取得ができませんでした。コスト画面を確認するか、時間をおいて再度お試しください。')
        setFetchMessage(summary || '予算超過のため取得をスキップしました。')
      } else if (hardFailed) {
        setFetchSeverity('warning')
        setError(
          Array.isArray(data.errors) && data.errors.length > 0
            ? `一部うまくいきませんでした: ${data.errors.join(' / ')}`
            : '一部の情報取得に失敗しました。',
        )
        setFetchMessage(
          `情報の取得が一部失敗しました（${summary}）。「最新の情報に更新」で再試行できます。`,
        )
      } else if (softEmpty) {
        const emptyAspects = formatFetchPrimaryEmptyAspects(data, knownIndustry)
        setFetchSeverity('warning')
        setFetchMessage(
          emptyAspects
            ? `${emptyAspects}は公開情報から特定できませんでした（${summary}）。手入力するか、時間をおいて再度お試しください。`
            : `取得を試しましたが、公開情報から特定できない項目がありました（${summary}）。手入力するか、時間をおいて再度お試しください。`,
        )
      } else if (allSkipped) {
        setFetchSeverity('warning')
        setFetchMessage(`すでに新しい情報があるか、業種により対象外のためスキップしました（${summary}）。`)
      } else {
        setFetchSeverity('success')
        setFetchMessage(
          force
            ? `情報を更新しました（${fetched} 項目）。${summary}`
            : `情報を取得しました（${fetched} 項目）。${summary} 内容を確認して問題なければ「学生に公開」できます。`,
        )
      }
      await reloadCurrentList('catalog')
    } catch {
      setError('企業情報の取得中に通信エラーが発生しました。時間をおいて再度お試しください。')
    } finally {
      setFetchPrimaryId(null)
    }
  }

  return {
    fetchPrimaryId,
    menuAnchor,
    setMenuAnchor,
    bulkPublishing,
    handlePublish,
    handleBulkPublish,
    handleSetVerified,
    handleReject,
    handleFetchPrimary,
  }
}
