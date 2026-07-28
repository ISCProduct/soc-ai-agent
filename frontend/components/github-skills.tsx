'use client'

import { useGitHubSkills } from './github-skills/hooks/useGitHubSkills'
import { GitHubSkillsLoading } from './github-skills/components/GitHubSkillsLoading'
import { GitHubSkillsNotLinked } from './github-skills/components/GitHubSkillsNotLinked'
import { GitHubSkillsView } from './github-skills/components/GitHubSkillsView'
import type { GitHubSkillsProps } from './github-skills/types'

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
