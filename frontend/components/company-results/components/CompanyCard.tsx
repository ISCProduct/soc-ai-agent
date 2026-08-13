import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Building2, MapPin, Users, TrendingUp, ExternalLink } from "lucide-react"
import { authService } from "@/lib/auth"
import { getGuestApplicationsButtonProps } from "@/lib/guest-limits"
import type { Company } from "../types"

type CompanyCardProps = {
  company: Company
  index: number
  isApplying: boolean
  onShowDetail: (company: Company) => void
  onApply: (company: Company) => void
}

export function CompanyCard({ company, index, isApplying, onShowDetail, onApply }: CompanyCardProps) {
  const isGuest = authService.getStoredUser()?.is_guest
  const guestButtonProps = getGuestApplicationsButtonProps(isGuest)
  const applyDisabled = company.isApplied || isApplying || guestButtonProps.disabled

  return (
    <Card className="border-2 hover:border-primary/50 transition-all">
      <CardHeader>
        <div className="flex items-start justify-between gap-4">
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-2">
              <div className="flex items-center justify-center w-8 h-8 rounded-full bg-primary text-primary-foreground font-bold text-sm">
                {index + 1}
              </div>
              <div className="p-2 rounded-lg bg-primary/10">
                <Building2 className="w-5 h-5 text-primary" />
              </div>
              <CardTitle className="text-xl">{company.name}</CardTitle>
            </div>
            <CardDescription className="text-base">{company.industry}</CardDescription>
          </div>
          <div className="text-right">
            <div className="text-3xl font-bold text-primary">{company.matchScore}%</div>
            <div className="text-xs text-muted-foreground">マッチ度</div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <p className="text-card-foreground mb-4 leading-relaxed">{company.description}</p>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-3 mb-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <MapPin className="w-4 h-4" />
            {company.location}
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Users className="w-4 h-4" />
            {company.employees}
          </div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <TrendingUp className="w-4 h-4" />
            {company.industry}
          </div>
        </div>

        <div className="mb-3">
          <div className="text-xs text-muted-foreground mb-1">技術スタック:</div>
          <div className="flex flex-wrap gap-1">
            {company.techStack.map((tech, techIndex) => (
              <Badge key={techIndex} variant="secondary" className="text-xs">
                {tech}
              </Badge>
            ))}
          </div>
        </div>

        <div className="flex flex-wrap gap-2 mb-4">
          {company.tags.map((tag, tagIndex) => (
            <Badge key={tagIndex} variant="outline">
              {tag}
            </Badge>
          ))}
        </div>

        <div className="flex gap-2">
          <Button className="flex-1 md:flex-initial" onClick={() => onShowDetail(company)}>
            <ExternalLink className="w-4 h-4 mr-2" />
            詳細ページへ
          </Button>
          <Button
            variant={company.isApplied ? "default" : "outline"}
            className="flex-1 md:flex-initial"
            disabled={applyDisabled}
            title={guestButtonProps.title || undefined}
            onClick={() => onApply(company)}
          >
            {company.isApplied ? "応募済み" : isApplying ? "応募中..." : "応募する"}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}
