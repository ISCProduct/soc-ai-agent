'use client'

import { useEffect, useMemo, useState } from 'react'
import { Stack, Typography } from '@mui/material'
import { authService } from '@/lib/auth'
import { useRequirePlatformAdmin } from '@/lib/admin/use-require-platform-admin'
import { PageContainer, ADMIN_PAGE_WIDTH } from '@/components/admin/PageContainer'
import { AdminPageHeader } from '@/components/admin/AdminPageHeader'
import { AdminPanel, AdminPanelBody } from '@/components/admin/AdminPanel'
import { AdminListCard } from '@/components/admin/AdminListCard'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { ErrorAlert } from '@/components/common/ErrorAlert'
import {
  confidenceBand,
  needsReview,
  parseFlags,
  type DiagnosisQualityReport,
} from '@/lib/admin/diagnosis-quality'

function ReportCard({ report }: { report: DiagnosisQualityReport }) {
  const flags = parseFlags(report.FlagsJSON)
  const band = confidenceBand(report.Confidence)
  return (
    <AdminListCard>
      <Stack spacing={0.5}>
        <Typography variant="subtitle2" fontWeight="bold">
          信頼度 {report.Confidence}（{band}）
        </Typography>
        <Typography variant="body2" color="text.secondary">
          ユーザー {report.UserID} ／ セッション {report.SessionID}
        </Typography>
        {flags.length > 0 && (
          <Typography variant="body2">フラグ: {flags.join('、')}</Typography>
        )}
        <Typography variant="body2">
          自動評価: {report.Summary || '要約はありません'}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          これは自動評価です。学生の適性として確定したものではありません。
        </Typography>
      </Stack>
    </AdminListCard>
  )
}

export default function PageContent() {
  const platformReady = useRequirePlatformAdmin()
  const [items, setItems] = useState<DiagnosisQualityReport[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    if (!platformReady) return
    let stale = false
    setLoading(true)
    setError('')
    fetch('/api/admin/diagnosis-quality?limit=100', {
      headers: authService.getAdminFetchHeaders(),
    })
      .then(async (response) => {
        const data = await response.json()
        if (!response.ok) throw new Error(data?.error || '診断品質の取得に失敗しました')
        if (!stale) setItems(Array.isArray(data?.items) ? data.items : [])
      })
      .catch((err: unknown) => {
        if (!stale) setError(err instanceof Error ? err.message : '診断品質の取得に失敗しました')
      })
      .finally(() => {
        if (!stale) setLoading(false)
      })
    return () => {
      stale = true
    }
  }, [platformReady])

  const review = useMemo(() => items.filter(needsReview), [items])
  const bands = useMemo(() => {
    const counts = { 低い: 0, ふつう: 0, 高い: 0 }
    for (const item of items) counts[confidenceBand(item.Confidence)] += 1
    return counts
  }, [items])

  if (!platformReady) return null

  return (
    <PageContainer maxWidth={ADMIN_PAGE_WIDTH.standard}>
      <AdminPageHeader
        title="診断品質"
        description="自動評価の信頼度が低い診断を先に確認します。学生に見せる適性結果ではありません。"
        backHref="/admin"
      />
      <ErrorAlert error={error} />
      {loading ? (
        <ListSkeleton rows={3} label="診断品質を読み込み中" />
      ) : items.length === 0 && !error ? (
        <EmptyState
          title="診断品質の記録はまだありません"
          description="診断が評価されると、信頼度の低いものからここに並びます。"
        />
      ) : (
        <Stack spacing={2}>
          <AdminPanel title="確認したい診断">
            <AdminPanelBody>
              {review.length === 0 ? (
                <EmptyState
                  title="確認が必要な診断はありません"
                  description="信頼度60未満、またはフラグが付いた診断がここに出ます。"
                />
              ) : (
                <Stack spacing={1}>
                  <Typography variant="body2" color="text.secondary">
                    {review.length}件。信頼度60未満、またはフラグがある診断です。
                  </Typography>
                  {review.map((report) => (
                    <ReportCard key={report.ID} report={report} />
                  ))}
                </Stack>
              )}
            </AdminPanelBody>
          </AdminPanel>
          <AdminPanel title="信頼度の内訳">
            <AdminPanelBody>
              <Typography variant="body2">
                低い {bands['低い']}件、ふつう {bands['ふつう']}件、高い {bands['高い']}件（直近{items.length}件）
              </Typography>
            </AdminPanelBody>
          </AdminPanel>
          <AdminPanel title="最近の診断">
            <AdminPanelBody>
              <Stack spacing={1}>
                {items.map((report) => (
                  <ReportCard key={`all-${report.ID}`} report={report} />
                ))}
              </Stack>
            </AdminPanelBody>
          </AdminPanel>
        </Stack>
      )}
    </PageContainer>
  )
}
