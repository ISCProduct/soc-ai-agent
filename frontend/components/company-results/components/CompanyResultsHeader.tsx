import { Badge } from "@/components/ui/badge"

type CompanyResultsHeaderProps = {
  isProvisional: boolean
  diagnosisSummary?: string | null
}

export function CompanyResultsHeader({ isProvisional, diagnosisSummary }: CompanyResultsHeaderProps) {
  return (
    <div className="mb-8 text-center">
      <h2 className="text-3xl font-bold text-foreground mb-3 text-balance">
        あなたに適した企業を10社に絞り込みました
      </h2>
      {isProvisional && (
        <div className="flex flex-col items-center gap-2 mb-3">
          <Badge variant="outline">暫定評価</Badge>
          <p className="text-sm text-muted-foreground max-w-xl text-pretty">
            {diagnosisSummary ||
              '回答の根拠がまだ薄いため、適合度は参考値です。選択肢に理由を添えると精度が上がります。'}
          </p>
        </div>
      )}
      <p className="text-muted-foreground text-pretty">
        {isProvisional
          ? '現時点の回答から仮マッチしています。会話を続けると根拠が厚くなります'
          : 'AIによる4段階の分析に基づいて、最適なIT企業をマッチングしました'}
      </p>
    </div>
  )
}
