import type { Company } from "../types"
import { CompanyCard } from "./CompanyCard"

type CompanyListProps = {
  companies: Company[]
  onShowDetail: (company: Company) => void
}

export function CompanyList({ companies, onShowDetail }: CompanyListProps) {
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
            onShowDetail={onShowDetail}
          />
        ))}
      </div>
    </>
  )
}
