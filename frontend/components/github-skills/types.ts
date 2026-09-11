export interface SkillScore {
  ID: number
  UserID: number
  Category: string
  Score: number
}

export interface LanguageStat {
  Language: string
  Percentage: number
}

export interface GitHubProfile {
  GitHubLogin: string
  TotalContributions: number
  PublicRepos: number
  Followers: number
}

export interface GitHubRepo {
  FullName: string
  Name: string
  Language: string
  Stars: number
  IsForked: boolean
}

export interface RepoSummary {
  ID: number
  FullName: string
  SummaryText: string
  TechReason: string
  Challenge: string
  Achievement: string
}

export type GitHubSkillsProps = {
  userId: number
  targetRole?: string
}

export interface RadarChartProps {
  scores: SkillScore[]
  size?: number
}
