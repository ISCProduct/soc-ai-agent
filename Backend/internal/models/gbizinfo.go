package models

import "time"

// GBizCompanyProfile gBizINFOから取得した法人基本情報
type GBizCompanyProfile struct {
	ID              uint `gorm:"primaryKey" json:"id"`
	CompanyID       uint `gorm:"not null;uniqueIndex" json:"company_id"`
	Company         Company
	CorporateNumber string `gorm:"type:varchar(13);index" json:"corporate_number"`
	Name            string `gorm:"type:varchar(255)" json:"name"`
	NameKana        string `gorm:"type:varchar(255)" json:"name_kana"`
	Location        string `gorm:"type:varchar(255)" json:"location"`
	PostalCode      string `gorm:"type:varchar(20)" json:"postal_code"`
	Representative  string `gorm:"type:varchar(255)" json:"representative"`
	CapitalStock    int64  `json:"capital_stock"`
	EmployeeNumber  int    `json:"employee_number"`
	DateEstablished string `gorm:"type:varchar(20)" json:"date_established"`
	CompanyURL      string `gorm:"type:varchar(500)" json:"company_url"`
	// BusinessSummary は gBizINFO の business_summary(事業概要)。
	// 「メディア事業\nインターネット広告事業」のように複数行で返ることがある。
	BusinessSummary string `gorm:"type:text" json:"business_summary"`
	UpdateDate      string `gorm:"type:varchar(20)" json:"update_date"`
	SourceFetchedAt time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GBizProcurement gBizINFO調達情報
type GBizProcurement struct {
	ID                    uint   `gorm:"primaryKey" json:"id"`
	CompanyID             uint   `gorm:"not null;index" json:"company_id"`
	CorporateNumber       string `gorm:"type:varchar(13);index" json:"corporate_number"`
	Title                 string `gorm:"type:varchar(500)" json:"title"`
	DateOfOrder           string `gorm:"type:varchar(20)" json:"date_of_order"`
	Amount                int64  `json:"amount"`
	GovernmentDepartments string `gorm:"type:varchar(255)" json:"government_departments"`
	JointSignatures       string `gorm:"type:text" json:"joint_signatures"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GBizSubsidy gBizINFO補助金情報
type GBizSubsidy struct {
	ID                    uint   `gorm:"primaryKey" json:"id"`
	CompanyID             uint   `gorm:"not null;index" json:"company_id"`
	CorporateNumber       string `gorm:"type:varchar(13);index" json:"corporate_number"`
	Title                 string `gorm:"type:varchar(500)" json:"title"`
	DateOfApproval        string `gorm:"type:varchar(20)" json:"date_of_approval"`
	Amount                string `gorm:"type:varchar(50)" json:"amount"`
	GovernmentDepartments string `gorm:"type:varchar(255)" json:"government_departments"`
	Target                string `gorm:"type:varchar(255)" json:"target"`
	Note                  string `gorm:"type:text" json:"note"`
	SubsidyResource       string `gorm:"type:varchar(255)" json:"subsidy_resource"`
	JointSignatures       string `gorm:"type:text" json:"joint_signatures"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// GBizFinance gBizINFO財務情報
type GBizFinance struct {
	ID              uint   `gorm:"primaryKey" json:"id"`
	CompanyID       uint   `gorm:"not null;index" json:"company_id"`
	CorporateNumber string `gorm:"type:varchar(13);index" json:"corporate_number"`
	Period          string `gorm:"type:varchar(50)" json:"period"`
	NetSales        int64  `json:"net_sales"`
	NetIncomeLoss   int64  `json:"net_income_loss"`
	TotalAssets     int64  `json:"total_assets"`
	NetAssets       int64  `json:"net_assets"`
	AccountingStd   string `gorm:"type:varchar(100)" json:"accounting_std"`
	FiscalYearCover string `gorm:"type:varchar(50)" json:"fiscal_year_cover"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GBizWorkplace gBizINFO職場情報
type GBizWorkplace struct {
	ID                                uint    `gorm:"primaryKey" json:"id"`
	CompanyID                         uint    `gorm:"not null;uniqueIndex" json:"company_id"`
	CorporateNumber                   string  `gorm:"type:varchar(13);index" json:"corporate_number"`
	AverageAge                        float64 `json:"average_age"`
	AverageContinuousServiceYears     float64 `json:"average_continuous_service_years"`
	AverageContinuousServiceYearsType string  `gorm:"type:varchar(10)" json:"average_continuous_service_years_type"`
	MonthAveragePredeterminedOvertime float64 `json:"month_average_predetermined_overtime_hours"`
	FemaleWorkersProportion           float64 `json:"female_workers_proportion"`
	FemaleWorkersProportionType       string  `gorm:"type:varchar(10)" json:"female_workers_proportion_type"`
	CreatedAt                         time.Time
	UpdatedAt                         time.Time
}
