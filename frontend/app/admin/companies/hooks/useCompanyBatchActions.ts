'use client'

// 一覧全体へのまとめて操作（足りない情報のまとめて取得・マッチング用データの準備/更新）を受け持つ。
import { useState, type Dispatch, type SetStateAction } from 'react'
import { authService } from '@/lib/auth'
import type { CompanyListReloadKind } from '../company-list-reload'
import {
  applyBatchWave,
  batchItemFailuresFromResponse,
  batchWaveFromResponse,
  candidateCountFromResponse,
  emptyBatchProgress,
  formatBatchFailureDetail,
  formatBatchProgressLabel,
  shouldContinueBatch,
  type BatchItemFailure,
  type BatchProgress,
} from '@/lib/admin/company-batch-progress'
import {
  GLOBAL_BATCH_CONCURRENCY,
  GLOBAL_BATCH_MAX_ROUNDS,
  GLOBAL_BATCH_PREVIEW_LIMIT,
  GLOBAL_BATCH_WAVE_LIMIT,
  WARM_L1_WAVE_LIMIT,
  type L1Coverage,
} from '../company-list-model'

type Params = {
  coverage: L1Coverage | null
  setCoverage: Dispatch<SetStateAction<L1Coverage | null>>
  fetchCoverage: () => Promise<void>
  reloadCurrentList: (kind: CompanyListReloadKind) => Promise<void>
  setError: Dispatch<SetStateAction<string>>
  setFetchMessage: Dispatch<SetStateAction<string>>
  setFetchSeverity: Dispatch<SetStateAction<'success' | 'warning'>>
}

export function useCompanyBatchActions({
  coverage,
  setCoverage,
  fetchCoverage,
  reloadCurrentList,
  setError,
  setFetchMessage,
  setFetchSeverity,
}: Params) {
  const [warming, setWarming] = useState(false)
  const [warmMessage, setWarmMessage] = useState('')
  const [missingBatchLoading, setMissingBatchLoading] = useState(false)
  const [batchProgress, setBatchProgress] = useState<BatchProgress | null>(null)

  const handleSeedL1 = async () => {
    setWarming(true)
    setWarmMessage('')
    setError('')
    try {
      const res = await fetch('/api/admin/companies/seed-l1', {
        method: 'POST',
        headers: authService.getAdminFetchHeaders(),
      })
      const data = await res.json()
      if (!res.ok) {
        setError(data?.error || data?.message || 'マッチング用データの準備に失敗しました')
        return
      }
      setWarmMessage(
        `準備完了: 新規 ${data.created ?? 0} 社 / 更新 ${data.updated ?? 0} 社 / 変更なし ${data.skipped ?? 0} 社`,
      )
      await reloadCurrentList('catalog')
    } finally {
      setWarming(false)
    }
  }

  const postCompanyBatch = async (
    path: '/api/admin/companies/fetch-missing-batch' | '/api/admin/companies/warm-l1',
    body: Record<string, unknown>,
  ): Promise<{ ok: boolean; status: number; data: Record<string, unknown> }> => {
    const res = await fetch(path, {
      method: 'POST',
      headers: {
        ...authService.getAdminFetchHeaders(),
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(body),
    })
    const data = (await res.json().catch(() => ({}))) as Record<string, unknown>
    return { ok: res.ok, status: res.status, data }
  }

  const handleWarmL1 = async (dryRun: boolean) => {
    setWarming(true)
    setWarmMessage('')
    setBatchProgress(null)
    setError('')
    try {
      if (dryRun) {
        const { ok, data } = await postCompanyBatch('/api/admin/companies/warm-l1', {
          limit: 100,
          dry_run: true,
          include_info: true,
          include_persona: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || 'マッチング用データの更新に失敗しました'))
          return
        }
        setWarmMessage(`確認のみ: 対象は約 ${candidateCountFromResponse(data)} 社です（一度に最大 ${data.limit ?? 100} 社）`)
        if (data.coverage && typeof data.coverage === 'object') {
          setCoverage(data.coverage as L1Coverage)
        }
        return
      }

      const preview = await postCompanyBatch('/api/admin/companies/warm-l1', {
        limit: 100,
        dry_run: true,
        include_info: true,
        include_persona: true,
      })
      let progress = emptyBatchProgress(
        Math.max(candidateCountFromResponse(preview.data), coverage?.needs_warm ?? 0),
      )
      setBatchProgress(progress)
      setWarmMessage(formatBatchProgressLabel(progress, true))

      for (;;) {
        const { ok, data } = await postCompanyBatch('/api/admin/companies/warm-l1', {
          limit: WARM_L1_WAVE_LIMIT,
          dry_run: false,
          include_info: true,
          include_persona: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || 'マッチング用データの更新に失敗しました'))
          break
        }
        const wave = {
          ...batchWaveFromResponse(data),
          persona_ok: Number(data.persona_ok ?? 0),
        }
        progress = applyBatchWave(progress, wave, WARM_L1_WAVE_LIMIT)
        setBatchProgress(progress)
        setWarmMessage(formatBatchProgressLabel(progress, true))
        if (data.coverage && typeof data.coverage === 'object') {
          setCoverage(data.coverage as L1Coverage)
        }
        if (!shouldContinueBatch(wave, WARM_L1_WAVE_LIMIT, progress.rounds, GLOBAL_BATCH_MAX_ROUNDS)) {
          break
        }
      }
      setWarmMessage(formatBatchProgressLabel(progress, false))
      await fetchCoverage()
    } finally {
      setWarming(false)
    }
  }

  const handleFetchMissingBatch = async (dryRun: boolean) => {
    setMissingBatchLoading(true)
    setFetchMessage('')
    setFetchSeverity('success')
    setBatchProgress(null)
    setError('')
    try {
      if (dryRun) {
        const { ok, status, data } = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
          limit: GLOBAL_BATCH_PREVIEW_LIMIT,
          dry_run: true,
          primary_only: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || `まとめて取得に失敗しました (${status})`))
          return
        }
        const names = Array.isArray(data.items)
          ? (data.items as { name?: string }[])
              .slice(0, 5)
              .map((i) => i.name)
              .filter(Boolean)
              .join('、')
          : ''
        const n = candidateCountFromResponse(data)
        if (n === 0) {
          setFetchSeverity('warning')
          setFetchMessage('いま取得対象になる企業はありません（会社概要・技術情報・関連企業がすでにそろっているか、対象候補が見つかりませんでした）。')
        } else {
          setFetchMessage(
            `情報が足りない企業は ${n}${n >= GLOBAL_BATCH_PREVIEW_LIMIT ? ' 社以上' : ' 社'}です` +
              (names ? `（例: ${names}${n > 5 ? ' など' : ''}）` : '') +
              '。取得を始めると件数の進捗が表示されます。',
          )
        }
        return
      }

      const preview = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
        limit: GLOBAL_BATCH_PREVIEW_LIMIT,
        dry_run: true,
        primary_only: true,
        concurrency: GLOBAL_BATCH_CONCURRENCY,
      })
      let progress = emptyBatchProgress(candidateCountFromResponse(preview.data))
      setBatchProgress(progress)
      setFetchMessage(formatBatchProgressLabel(progress, true))
      const failures: BatchItemFailure[] = []

      for (;;) {
        const { ok, status, data } = await postCompanyBatch('/api/admin/companies/fetch-missing-batch', {
          limit: GLOBAL_BATCH_WAVE_LIMIT,
          dry_run: false,
          primary_only: true,
          concurrency: GLOBAL_BATCH_CONCURRENCY,
        })
        if (!ok) {
          setError(String(data.error || data.message || `まとめて取得に失敗しました (${status})`))
          break
        }
        const wave = batchWaveFromResponse(data)
        progress = applyBatchWave(progress, wave, GLOBAL_BATCH_WAVE_LIMIT)
        failures.push(...batchItemFailuresFromResponse(data))
        setBatchProgress(progress)
        setFetchSeverity(progress.errors > 0 ? 'warning' : 'success')
        setFetchMessage(formatBatchProgressLabel(progress, true))
        if (!shouldContinueBatch(wave, GLOBAL_BATCH_WAVE_LIMIT, progress.rounds, GLOBAL_BATCH_MAX_ROUNDS)) {
          break
        }
      }

      if (progress.processed === 0) {
        setFetchSeverity('warning')
        setFetchMessage('取得対象の企業が見つかりませんでした。一覧で「まだ足りない情報があります」と出ている企業がある場合は、時間をおいて再度お試しください。')
      } else {
        setFetchSeverity(progress.errors > 0 ? 'warning' : 'success')
        setFetchMessage(formatBatchProgressLabel(progress, false) + '。内容を確認してから公開してください。')
        if (progress.errors > 0) {
          setError(
            formatBatchFailureDetail(failures) ||
              `${progress.errors} 社で取得に失敗しました。時間をおいて再度お試しください。`,
          )
        }
      }
      await reloadCurrentList('catalog')
    } finally {
      setMissingBatchLoading(false)
    }
  }

  return {
    warming,
    warmMessage,
    missingBatchLoading,
    batchProgress,
    handleSeedL1,
    handleWarmL1,
    handleFetchMissingBatch,
  }
}
