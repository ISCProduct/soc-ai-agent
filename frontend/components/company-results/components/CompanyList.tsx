import type { Company } from "../types"
import { CompanyCard } from "./CompanyCard"

type CompanyListProps = {
  companies: Company[]
  applyingId: number | null
  onShowDetail: (company: Company) => void
  onApply: (company: Company) => void
}

export function CompanyList({ companies, applyingId, onShowDetail, onApply }: CompanyListProps) {
  return (
    <>
      <div className="mb-4">
        <h3 className="text-xl font-bold text-foreground mb-4">選定企業一覧（マッチ度順）</h3>
      </div>

      <div className="space-y-4 mb-8">
        {companies.map((company, index) => (
          <CompanyCard
            key={company.id}
            company={company}
            index={index}
            isApplying={applyingId === company.matchId}
            onShowDetail={onShowDetail}
            onApply={onApply}
          />
        ))}
      </div>
    </>
  )
}
