import { Badge } from "@/components/ui/badge"

type CompanyResultsHeaderProps = {
  isProvisional: boolean
}

export function CompanyResultsHeader({ isProvisional }: CompanyResultsHeaderProps) {
  return (
    <div className="mb-8 text-center">
      <h2 className="text-3xl font-bold text-foreground mb-3 text-balance">
        あなたに適した企業を10社に絞り込みました
      </h2>
      {isProvisional && (
        <div className="flex justify-center mb-2">
          <Badge variant="outline">暫定評価</Badge>
        </div>
      )}
      <p className="text-muted-foreground text-pretty">
        AIによる4段階の分析に基づいて、最適なIT企業をマッチングしました
      </p>
    </div>
  )
}
