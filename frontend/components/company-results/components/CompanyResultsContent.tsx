import { Button } from "@/components/ui/button"
import { RotateCcw } from "lucide-react"
import type { Company, UserData } from "../types"
import { CompanyResultsHeader } from "./CompanyResultsHeader"
import { AnalysisReportCard } from "./AnalysisReportCard"
import { CompanyList } from "./CompanyList"
import { IndustryMapPlaceholder } from "./IndustryMapPlaceholder"

type CompanyResultsContentProps = {
  userData: UserData
  companies: Company[]
  isProvisional: boolean
  applyingId: number | null
  onShowDetail: (company: Company) => void
  onApply: (company: Company) => void
  onResetAction: () => void
}

export function CompanyResultsContent({
  userData,
  companies,
  isProvisional,
  applyingId,
  onShowDetail,
  onApply,
  onResetAction,
}: CompanyResultsContentProps) {
  return (
    <>
      <CompanyResultsHeader isProvisional={isProvisional} />
      <AnalysisReportCard userData={userData} />
      <CompanyList companies={companies} applyingId={applyingId} onShowDetail={onShowDetail} onApply={onApply} />
      <IndustryMapPlaceholder />

      <div className="text-center">
        <Button variant="outline" onClick={onResetAction} size="lg">
          <RotateCcw className="w-4 h-4 mr-2" />
          最初からやり直す
        </Button>
      </div>
    </>
  )
}
