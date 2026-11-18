'use client'

import { useCallback, useEffect, useRef, useState, type MouseEvent } from 'react'
import { useRouter } from 'next/navigation'
import { Alert, Box, Button, Snackbar, Typography } from '@mui/material'
import { authService } from '@/lib/auth'
import { getResultsPathOrChat } from '@/lib/results-navigation'
import { BottomNavSpacer } from '@/components/common/BottomNavSpacer'
import { EmptyState } from '@/components/common/EmptyState'
import { ListSkeleton } from '@/components/common/ListSkeleton'
import { ABOVE_BOTTOM_NAV_SX } from '@/lib/sidebar-nav'
import { CONTENT_MAX_WIDTH } from '@/lib/design/tokens'
import FavoritesCompareList from '@/app/results/components/FavoritesCompareList'
import type { Company, SnackbarState } from '@/app/results/types'
import { mapRecommendationToCompany } from '@/app/results/utils'

/**
 * 診断をまたいだお気に入りだけを見る画面。
 * 結果画面は user_id と session_id が無いと開かないが、お気に入りAPIはユーザー単位なので
 * 今の診断セッションが無くても読める。
 */
export default function PageContent() {
  const router = useRouter()
  const requestRef = useRef(0)
  const [companies, setCompanies] = useState<Company[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [favoritingId, setFavoritingId] = useState<number | null>(null)
  const [snackbar, setSnackbar] = useState<SnackbarState>({ open: false, message: '', severity: 'success' })

  const loadFavorites = useCallback(async () => {
    const id = ++requestRef.current
    setLoading(true)
    setLoadError(null)
    try {
      const res = await fetch('/api/chat/favorites?limit=100', {
        headers: authService.getUserFetchHeaders(),
        cache: 'no-store',
      })
      if (id !== requestRef.current) return
      if (!res.ok) throw new Error('お気に入りを読み込めませんでした')
      const data = await res.json()
      if (id !== requestRef.current) return
      if (!data || !Array.isArray(data.recommendations)) {
        throw new Error('お気に入りの形式が正しくありません')
      }
      setCompanies(
        data.recommendations.map((rec: unknown, index: number) => mapRecommendationToCompany(rec, index)),
      )
    } catch (err) {
      if (id !== requestRef.current) return
      setCompanies([])
      setLoadError(err instanceof Error ? err.message : 'お気に入りを読み込めませんでした')
    } finally {
      if (id === requestRef.current) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadFavorites()
  }, [loadFavorites])

  const handleUnfavorite = async (e: MouseEvent, company: Company) => {
    e.stopPropagation()
    const companyID = Number(company.id)
    if (!companyID || favoritingId !== null) return
    requestRef.current += 1
    setLoading(false)
    setFavoritingId(company.matchId ?? companyID)
    try {
      const res = await fetch('/api/chat/favorites/remove', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...authService.getUserFetchHeaders() },
        body: JSON.stringify({ company_id: companyID }),
      })
      if (!res.ok) {
        setSnackbar({ open: true, message: 'お気に入りの更新に失敗しました', severity: 'error' })
        return
      }
      setCompanies((prev) => prev.filter((item) => item.id !== company.id))
      setSnackbar({ open: true, message: 'すべての診断のお気に入りから外しました', severity: 'success' })
    } catch {
      setSnackbar({ open: true, message: 'お気に入りの更新に失敗しました', severity: 'error' })
    } finally {
      setFavoritingId(null)
    }
  }

  return (
    <Box sx={{ maxWidth: CONTENT_MAX_WIDTH, mx: 'auto', p: { xs: 2, sm: 3 } }}>
      <Typography variant="h5" component="h1" fontWeight="bold">
        お気に入り
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mt: 1, mb: 3 }}>
        今までの診断でお気に入りにした企業です。同じ企業は1社にまとめています。適合度は、いちばん新しい診断のものです。
      </Typography>

      {loading ? (
        <ListSkeleton rows={3} label="お気に入りを読み込み中" />
      ) : loadError ? (
        <Alert
          severity="error"
          action={
            <Button color="inherit" onClick={() => void loadFavorites()}>
              再読み込み
            </Button>
          }
        >
          {loadError}
        </Alert>
      ) : companies.length === 0 ? (
        <EmptyState
          title="お気に入りにした企業はまだありません"
          description="マッチング結果でハートを押すと、ここに集まります。"
          titleComponent="h2"
          action={
            <Button variant="contained" onClick={() => router.push(getResultsPathOrChat())}>
              おすすめ企業を見る
            </Button>
          }
        />
      ) : (
        <FavoritesCompareList
          companies={companies}
          favoritingId={favoritingId}
          onSelectCompany={(company) => router.push(`/company/${company.id}`)}
          onToggleFavorite={(e, company) => void handleUnfavorite(e, company)}
        />
      )}

      <Snackbar
        open={snackbar.open}
        autoHideDuration={4000}
        onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        sx={ABOVE_BOTTOM_NAV_SX}
      >
        <Alert severity={snackbar.severity} onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}>
          {snackbar.message}
        </Alert>
      </Snackbar>
      <BottomNavSpacer />
    </Box>
  )
}
