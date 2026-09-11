import { Container } from '@mui/material'
import { BackButton } from '@/components/common/back-button'
import { MarkWhatsNewSeen } from '@/components/common/mark-whats-new-seen'
import { fetchWhatsNewEntriesServer, type WhatsNewEntry } from '@/lib/whats-new-data'
import { getServerUserAuthHeaders, requireSessionUser } from '@/lib/server-auth'
import { WhatsNewView } from './whats-new-view'

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
      <WhatsNewView entries={entries} error={error} />
    </Container>
  )
}
