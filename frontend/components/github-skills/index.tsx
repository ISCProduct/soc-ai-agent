'use client'

import { useGitHubSkills } from './hooks/useGitHubSkills'
import { GitHubSkillsLoading } from './components/GitHubSkillsLoading'
import { GitHubSkillsNotLinked } from './components/GitHubSkillsNotLinked'
import { GitHubSkillsView } from './components/GitHubSkillsView'
import type { GitHubSkillsProps } from './types'

/**
 * GitHub 連携ユーザーのスキル分析・リポジトリ要約 UI。
 * 状態・副作用は useGitHubSkills、表示は各コンポーネントに委譲する。
 */
export default function GitHubSkills({ userId, targetRole = '' }: GitHubSkillsProps) {
  const skills = useGitHubSkills(userId, targetRole)

  if (skills.loading) {
    return <GitHubSkillsLoading />
  }

  if (skills.notLinked) {
    return (
      <GitHubSkillsNotLinked
        connecting={skills.connecting}
        onConnect={skills.handleConnect}
      />
    )
  }

  return <GitHubSkillsView {...skills} />
}
