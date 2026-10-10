import { CompanyPortalFrame } from '@/components/company-portal/CompanyPortalFrame'

export default function CompanyPortalLayout({ children }: { children: React.ReactNode }) {
  return <CompanyPortalFrame>{children}</CompanyPortalFrame>
}
