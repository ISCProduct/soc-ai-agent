export type CompanyReadingContext = {
  reading: string
  readingResolved: boolean
}

type CompanyReadingResponse = {
  company_reading?: unknown
  company_reading_resolved?: unknown
}

export function applyCompanyReadingResponse(
  current: CompanyReadingContext,
  response: CompanyReadingResponse,
  responseGeneration: number,
  currentGeneration: number,
): CompanyReadingContext {
  if (responseGeneration !== currentGeneration) return current

  const reading = typeof response.company_reading === 'string'
    ? response.company_reading
    : current.reading
  return {
    reading,
    readingResolved: current.readingResolved
      || response.company_reading_resolved === true
      || Boolean(response.company_reading),
  }
}
