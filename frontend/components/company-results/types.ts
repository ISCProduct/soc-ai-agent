export type UserData = {
  scores?: {
    category: string
    score: number
    reason: string
  }[]
}

export type UserScore = {
  [category: string]: number
}

export type Company = {
  id: number
  matchId?: number
  isApplied?: boolean
  name: string
  industry: string
  location: string
  employees: string
  description: string
  matchScore: number
  tags: string[]
  techStack: string[]
  projectTypes: string[]
  salary: string
  benefits: string[]
  culture: string[]
  founded: string
  website: string
  size: string
  parentCompany?: string
  subsidiaries?: string[]
  partnerships?: string[]
  capitalStructure?: {
    shareholders: { name: string; percentage: number }[]
  }
}

export type CompanyResultsProps = {
  userData: UserData
  onResetAction: () => void
}
