import { Box, Container, Typography, Divider } from '@mui/material'
import type { WhatsNewEntry } from '@/lib/whats-new-data'

interface WhatsNewViewProps {
  entries: WhatsNewEntry[]
  error: boolean
}

export function WhatsNewView({ entries, error }: WhatsNewViewProps) {
  return (
    <>
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
    </>
  )
}
