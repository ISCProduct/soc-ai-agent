'use client'

import { useEffect, useState } from 'react'
import { Box, Container, Typography, Divider, Button } from '@mui/material'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import { useRouter } from 'next/navigation'
import { WhatsNewEntry, fetchWhatsNewEntries, markWhatsNewAsSeen } from '@/lib/whats-new-data'
import { PageLoading } from '@/components/common/PageLoading'

export default function WhatsNewPage() {
  const router = useRouter()
  const [entries, setEntries] = useState<WhatsNewEntry[] | null>(null)
  const [error, setError] = useState(false)

  useEffect(() => {
    fetchWhatsNewEntries()
      .then((data) => {
        setEntries(data)
        markWhatsNewAsSeen(data)
      })
      .catch(() => setError(true))
  }, [])

  return (
    <Container maxWidth="md" sx={{ py: 6 }}>
      <Button startIcon={<ArrowBackIcon />} onClick={() => router.back()} sx={{ mb: 3 }}>
        戻る
      </Button>

      <Typography variant="h4" fontWeight="bold" gutterBottom>
        更新情報
      </Typography>
      <Divider sx={{ my: 3 }} />

      {error ? (
        <Typography variant="body1" color="text.secondary">
          更新情報の取得に失敗しました。時間をおいて再度お試しください。
        </Typography>
      ) : entries === null ? (
        <PageLoading message="更新情報を取得しています..." />
      ) : entries.length === 0 ? (
        <Typography variant="body1" color="text.secondary">
          更新情報はまだありません。
        </Typography>
      ) : (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {entries.map((entry) => (
            <Box key={entry.merged_at + entry.title}>
              <Typography variant="caption" color="text.secondary">
                {new Date(entry.merged_at).toLocaleDateString('ja-JP')}
              </Typography>
              <Typography variant="h6" fontWeight="bold">
                {entry.title}
              </Typography>
              <Typography variant="body1" color="text.secondary" sx={{ mt: 0.5 }}>
                {entry.summary}
              </Typography>
            </Box>
          ))}
        </Box>
      )}
    </Container>
  )
}
