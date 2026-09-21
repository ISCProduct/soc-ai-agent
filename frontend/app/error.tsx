'use client'

import { ServiceErrorView } from '@/components/ServiceErrorView'

export default function AppError({ reset }: { error: Error; reset: () => void }) {
  return <ServiceErrorView onRetry={reset} />
}
