'use client'

import { ServiceErrorView } from '@/components/service-error-view'

export default function AppError({ reset }: { error: Error; reset: () => void }) {
  return <ServiceErrorView onRetry={reset} />
}
