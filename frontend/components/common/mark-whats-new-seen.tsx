'use client'

import { useEffect } from 'react'
import { markWhatsNewAsSeen, type WhatsNewEntry } from '@/lib/whats-new-data'

export function MarkWhatsNewSeen({ entries }: { entries: WhatsNewEntry[] }) {
  useEffect(() => {
    markWhatsNewAsSeen(entries)
  }, [entries])
  return null
}
