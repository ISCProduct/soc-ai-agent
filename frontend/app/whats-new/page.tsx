import { Box, Container, Typography, Divider } from '@mui/material'
import { BackButton } from '@/components/common/back-button'
import { MarkWhatsNewSeen } from '@/components/common/mark-whats-new-seen'
import { fetchWhatsNewEntriesServer, type WhatsNewEntry } from '@/lib/whats-new-data'
import { getServerUserAuthHeaders, requireSessionUser } from '@/lib/server-auth'

export default async function WhatsNewPage() {
  await requireSessionUser()
  const authHeaders = await getServerUserAuthHeaders()
  let entries: WhatsNewEntry[] = []
  let error = false

  if (authHeaders) {
    try {
      entries = await fetchWhatsNewEntriesServer(authHeaders)
    } catch {
      error = true
    }
  } else {
    error = true
  }

  return (
    <Container maxWidth="md" sx={{ py: 6 }}>
      <BackButton />
      <MarkWhatsNewSeen entries={entries} />

      <Typography variant="h4" fontWeight="bold" gutterBottom>
        更新情報
      </Typography>
      <Divider sx={{ my: 3 }} />

      {error ? (
        <Typography variant="body1" color="text.secondary">
          更新情報の取得に失敗しました。時間をおいて再度お試しください。
        </Typography>
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
