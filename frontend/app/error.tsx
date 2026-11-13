'use client'

import { ServiceErrorView } from '@/components/common/ServiceErrorView'

export default function AppError({ reset }: { error: Error; reset: () => void }) {
  return <ServiceErrorView onRetry={reset} />
}
