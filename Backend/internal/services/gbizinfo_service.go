package services

import (
	"Backend/domain/repository"
	"Backend/internal/config"
	"Backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GBizInfoService struct {
	baseURL      string
	token        string
	client       *http.Client
	repo         repository.GBizInfoRepository
	companyRepo  repository.CompanyRepository
	relationRepo repository.CompanyRelationRepository
}

type GBizSearchResult struct {
	CorporateNumber string `json:"corporate_number"`
	Name            string `json:"name"`
	Location        string `json:"location"`
	CompanyURL      string `json:"company_url"`
	EmployeeNumber  int    `json:"employee_number"`
}

func (s *GBizInfoService) SearchByName(ctx context.Context, name string) ([]GBizSearchResult, error) {
	if s == nil || s.baseURL == "" || s.token == "" {
		return nil, errors.New("gbizinfo service is not configured")
	}
	var resp gbizProfileResponse
	path := "/v2/hojin?name=" + url.QueryEscape(name) + "&limit=10"
	if err := s.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	results := make([]GBizSearchResult, 0, len(resp.HojinInfos))
	for _, info := range resp.HojinInfos {
		results = append(results, GBizSearchResult{
			CorporateNumber: info.CorporateNumber,
			Name:            info.Name,
			Location:        info.Location,
			CompanyURL:      info.CompanyURL,
			EmployeeNumber:  info.EmployeeNumber,
		})
	}
	return results, nil
}

type GBizSyncResult struct {
	CompanyID        uint   `json:"company_id"`
	CorporateNumber  string `json:"corporate_number"`
	Status           string `json:"status"`
	Message          string `json:"message"`
	ProfileUpdated   bool   `json:"profile_updated"`
	ProcurementCount int    `json:"procurement_count"`
	SubsidyCount     int    `json:"subsidy_count"`
	FinanceCount     int    `json:"finance_count"`
	WorkplaceUpdated bool   `json:"workplace_updated"`
}

func NewGBizInfoService(cfg *config.Config, repo repository.GBizInfoRepository, companyRepo repository.CompanyRepository, relationRepo repository.CompanyRelationRepository) *GBizInfoService {
	baseURL := strings.TrimRight(cfg.GBizInfoBaseURL, "/")
	return &GBizInfoService{
		baseURL:      baseURL,
		token:        cfg.GBizInfoToken,
		client:       &http.Client{Timeout: 15 * time.Second},
		repo:         repo,
		companyRepo:  companyRepo,
		relationRepo: relationRepo,
	}
}

func (s *GBizInfoService) SyncCompany(ctx context.Context, companyID uint) (*GBizSyncResult, error) {
	if s == nil {
		return nil, errors.New("gbizinfo service is not configured")
	}
	company, err := s.companyRepo.FindByID(companyID)
	if err != nil {
		return nil, err
	}
	corporateNumber := strings.TrimSpace(company.CorporateNumber)
	if corporateNumber == "" {
		return s.syncFailed(company, "corporate_number is required")
	}
	if s.baseURL == "" || s.token == "" {
		return s.syncFailed(company, "GBizInfo API configuration is missing")
	}

	profile, err := s.fetchProfile(ctx, corporateNumber)
	if err != nil {
		return s.syncFailed(company, err.Error())
	}
	procurements, err := s.fetchProcurements(ctx, corporateNumber, company.ID)
	if err != nil {
		return s.syncFailed(company, err.Error())
	}
	subsidies, err := s.fetchSubsidies(ctx, corporateNumber, company.ID)
	if err != nil {
		return s.syncFailed(company, err.Error())
	}
	finances, err := s.fetchFinances(ctx, corporateNumber, company.ID)
	if err != nil {
		return s.syncFailed(company, err.Error())
	}
	workplace, err := s.fetchWorkplace(ctx, corporateNumber, company.ID)
	if err != nil {
		return s.syncFailed(company, err.Error())
	}

	now := time.Now()
	profile.CompanyID = company.ID
	profile.CorporateNumber = corporateNumber
	profile.SourceFetchedAt = now
	if err := s.repo.UpsertProfile(profile); err != nil {
		return s.syncFailed(company, err.Error())
	}
	if err := s.repo.ReplaceProcurements(company.ID, procurements); err != nil {
		return s.syncFailed(company, err.Error())
	}
	if err := s.repo.ReplaceSubsidies(company.ID, subsidies); err != nil {
		return s.syncFailed(company, err.Error())
	}
	if err := s.repo.ReplaceFinances(company.ID, finances); err != nil {
		return s.syncFailed(company, err.Error())
	}
	workplaceUpdated := false
	if workplace != nil {
		if err := s.repo.UpsertWorkplace(workplace); err != nil {
			return s.syncFailed(company, err.Error())
		}
		workplaceUpdated = true
	}
	if err := s.updateBusinessRelations(company, procurements, subsidies); err != nil {
		return s.syncFailed(company, err.Error())
	}

	applyGBizProfile(company, profile)
	company.GBizLastSyncedAt = &now
	company.GBizSyncStatus = "success"
	company.GBizSyncMessage = ""
	if err := s.companyRepo.Update(company); err != nil {
		return nil, err
	}

	return &GBizSyncResult{
		CompanyID:        company.ID,
		CorporateNumber:  corporateNumber,
		Status:           "success",
		Message:          "synced",
		ProfileUpdated:   true,
		ProcurementCount: len(procurements),
		SubsidyCount:     len(subsidies),
		FinanceCount:     len(finances),
		WorkplaceUpdated: workplaceUpdated,
	}, nil
}

func (s *GBizInfoService) syncFailed(company *models.Company, message string) (*GBizSyncResult, error) {
	now := time.Now()
	company.GBizLastSyncedAt = &now
	company.GBizSyncStatus = "failed"
	company.GBizSyncMessage = message
	_ = s.companyRepo.Update(company)
	return &GBizSyncResult{
		CompanyID:       company.ID,
		CorporateNumber: company.CorporateNumber,
		Status:          "failed",
		Message:         message,
	}, errors.New(message)
}

func (s *GBizInfoService) fetchProfile(ctx context.Context, corporateNumber string) (*models.GBizCompanyProfile, error) {
	var resp gbizProfileResponse
	if err := s.get(ctx, "/v2/hojin/"+corporateNumber, &resp); err != nil {
		return nil, err
	}
	if len(resp.HojinInfos) == 0 {
		return nil, errors.New("gbizinfo: profile not found")
	}
	info := resp.HojinInfos[0]
	return &models.GBizCompanyProfile{
		CorporateNumber: info.CorporateNumber,
		Name:            info.Name,
		NameKana:        info.Kana,
		Location:        info.Location,
		PostalCode:      info.PostalCode,
		Representative:  info.RepresentativeName,
		CapitalStock:    info.CapitalStock,
		EmployeeNumber:  info.EmployeeNumber,
		DateEstablished: info.DateOfEstablishment,
		CompanyURL:      info.CompanyURL,
		UpdateDate:      info.UpdateDate,
	}, nil
}

func (s *GBizInfoService) fetchProcurements(ctx context.Context, corporateNumber string, companyID uint) ([]models.GBizProcurement, error) {
	var resp gbizProcurementResponse
	if err := s.get(ctx, "/v2/hojin/"+corporateNumber+"/procurement", &resp); err != nil {
		return nil, err
	}
	if len(resp.HojinInfos) == 0 {
		return nil, nil
	}
	info := resp.HojinInfos[0]
	rows := make([]models.GBizProcurement, 0, len(info.Procurement))
	for _, item := range info.Procurement {
		rows = append(rows, models.GBizProcurement{
			CompanyID:             companyID,
			CorporateNumber:       corporateNumber,
			Title:                 strings.TrimSpace(item.Title),
			DateOfOrder:           item.DateOfOrder,
			Amount:                item.Amount,
			GovernmentDepartments: strings.TrimSpace(item.GovernmentDepartments),
		})
	}
	return rows, nil
}

func (s *GBizInfoService) fetchSubsidies(ctx context.Context, corporateNumber string, companyID uint) ([]models.GBizSubsidy, error) {
	var resp gbizSubsidyResponse
	if err := s.get(ctx, "/v2/hojin/"+corporateNumber+"/subsidy", &resp); err != nil {
		return nil, err
	}
	if len(resp.HojinInfos) == 0 {
		return nil, nil
	}
	info := resp.HojinInfos[0]
	rows := make([]models.GBizSubsidy, 0, len(info.Subsidy))
	for _, item := range info.Subsidy {
		rows = append(rows, models.GBizSubsidy{
			CompanyID:             companyID,
			CorporateNumber:       corporateNumber,
			Title:                 strings.TrimSpace(item.Title),
			DateOfApproval:        item.DateOfApproval,
			Amount:                item.Amount,
			GovernmentDepartments: strings.TrimSpace(item.GovernmentDepartments),
			Target:                strings.TrimSpace(item.Target),
			Note:                  strings.TrimSpace(item.Note),
		})
	}
	return rows, nil
}

func (s *GBizInfoService) fetchFinances(ctx context.Context, corporateNumber string, companyID uint) ([]models.GBizFinance, error) {
	var resp gbizFinanceResponse
	if err := s.get(ctx, "/v2/hojin/"+corporateNumber+"/finance", &resp); err != nil {
		return nil, err
	}
	if len(resp.HojinInfos) == 0 {
		return nil, nil
	}
	info := resp.HojinInfos[0]
	rows := make([]models.GBizFinance, 0, len(info.Finance.ManagementIndex))
	for _, item := range info.Finance.ManagementIndex {
		rows = append(rows, models.GBizFinance{
			CompanyID:       companyID,
			CorporateNumber: corporateNumber,
			Period:          item.Period,
			NetSales:        item.NetSales,
			NetIncomeLoss:   item.NetIncomeLoss,
			TotalAssets:     item.TotalAssets,
			NetAssets:       item.NetAssets,
			AccountingStd:   strings.TrimSpace(info.Finance.AccountingStandards),
			FiscalYearCover: strings.TrimSpace(info.Finance.FiscalYearCoverPage),
		})
	}
	return rows, nil
}

func (s *GBizInfoService) fetchWorkplace(ctx context.Context, corporateNumber string, companyID uint) (*models.GBizWorkplace, error) {
	var resp gbizWorkplaceResponse
	if err := s.get(ctx, "/v2/hojin/"+corporateNumber+"/workplace", &resp); err != nil {
		return nil, err
	}
	if len(resp.HojinInfos) == 0 {
		return nil, nil
	}
	info := resp.HojinInfos[0]
	return &models.GBizWorkplace{
		CompanyID:                         companyID,
		CorporateNumber:                   corporateNumber,
		AverageAge:                        info.WorkplaceInfo.BaseInfos.AverageAge,
		AverageContinuousServiceYears:     info.WorkplaceInfo.BaseInfos.AverageContinuousServiceYears,
		AverageContinuousServiceYearsType: info.WorkplaceInfo.BaseInfos.AverageContinuousServiceYearsType,
		MonthAveragePredeterminedOvertime: info.WorkplaceInfo.BaseInfos.MonthAveragePredeterminedOvertimeHours,
		FemaleWorkersProportion:           info.WorkplaceInfo.WomenActivityInfos.FemaleWorkersProportion,
		FemaleWorkersProportionType:       info.WorkplaceInfo.WomenActivityInfos.FemaleWorkersProportionType,
	}, nil
}

func (s *GBizInfoService) get(ctx context.Context, path string, out any) error {
	if s.baseURL == "" {
		return errors.New("gbizinfo: base url is empty")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-hojinInfo-api-token", s.token)
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("gbizinfo: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func applyGBizProfile(company *models.Company, profile *models.GBizCompanyProfile) {
	if strings.TrimSpace(company.Name) == "" && profile.Name != "" {
		company.Name = profile.Name
	}
	if strings.TrimSpace(company.Location) == "" && profile.Location != "" {
		company.Location = profile.Location
	}
	if company.EmployeeCount == 0 && profile.EmployeeNumber > 0 {
		company.EmployeeCount = profile.EmployeeNumber
	}
	if company.FoundedYear == 0 && len(profile.DateEstablished) >= 4 {
		if year, err := strconv.Atoi(profile.DateEstablished[:4]); err == nil {
			company.FoundedYear = year
		}
	}
	if strings.TrimSpace(company.WebsiteURL) == "" && profile.CompanyURL != "" {
		company.WebsiteURL = profile.CompanyURL
	}
	if strings.TrimSpace(company.CorporateNumber) == "" && profile.CorporateNumber != "" {
		company.CorporateNumber = profile.CorporateNumber
	}
	if company.SourceFetchedAt == nil {
		now := time.Now()
		company.SourceFetchedAt = &now
	}
	if strings.TrimSpace(company.SourceType) == "" {
		company.SourceType = "gbizinfo"
	}
}

func (s *GBizInfoService) updateBusinessRelations(company *models.Company, procurements []models.GBizProcurement, subsidies []models.GBizSubsidy) error {
	if s.relationRepo == nil {
		return nil
	}
	for _, item := range procurements {
		if err := s.createRelationsFromJointSignatures(company, item.Title, item.DateOfOrder, item.JointSignatures, "business_procurement"); err != nil {
			return err
		}
	}
	for _, item := range subsidies {
		if err := s.createRelationsFromJointSignatures(company, item.Title, item.DateOfApproval, item.JointSignatures, "business_subsidy"); err != nil {
			return err
		}
	}
	return nil
}

func (s *GBizInfoService) createRelationsFromJointSignatures(company *models.Company, title, date, jointSignatures, relationType string) error {
	partners := parseJointSignatures(jointSignatures)
	for _, partner := range partners {
		name := strings.TrimSpace(partner)
		if name == "" || company == nil || strings.EqualFold(name, company.Name) {
			continue
		}
		partnerCompany, err := s.companyRepo.FindByName(name)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if partnerCompany == nil || errors.Is(err, gorm.ErrRecordNotFound) {
			now := time.Now()
			partnerCompany = &models.Company{
				Name:            name,
				SourceType:      "gbizinfo",
				SourceFetchedAt: &now,
				IsProvisional:   true,
				DataStatus:      "draft",
			}
			if err := s.companyRepo.Create(partnerCompany); err != nil {
				return err
			}
		}
		description := fmt.Sprintf("%s (%s)", strings.TrimSpace(title), strings.TrimSpace(date))
		if err := s.relationRepo.UpsertBusinessRelation(company.ID, partnerCompany.ID, relationType, description); err != nil {
			return err
		}
	}
	return nil
}

func parseJointSignatures(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(raw), &names); err == nil {
		return names
	}
	return []string{}
}

type gbizProfileResponse struct {
	HojinInfos []struct {
		CorporateNumber     string `json:"corporate_number"`
		Name                string `json:"name"`
		Kana                string `json:"kana"`
		Location            string `json:"location"`
		PostalCode          string `json:"postal_code"`
		RepresentativeName  string `json:"representative_name"`
		CapitalStock        int64  `json:"capital_stock"`
		EmployeeNumber      int    `json:"employee_number"`
		DateOfEstablishment string `json:"date_of_establishment"`
		CompanyURL          string `json:"company_url"`
		UpdateDate          string `json:"update_date"`
	} `json:"hojin-infos"`
}

type gbizProcurementResponse struct {
	HojinInfos []struct {
		Procurement []struct {
			Amount                int64  `json:"amount"`
			DateOfOrder           string `json:"date_of_order"`
			GovernmentDepartments string `json:"government_departments"`
			Title                 string `json:"title"`
		} `json:"procurement"`
	} `json:"hojin-infos"`
}

type gbizSubsidyResponse struct {
	HojinInfos []struct {
		Subsidy []struct {
			Amount                string `json:"amount"`
			DateOfApproval        string `json:"date_of_approval"`
			GovernmentDepartments string `json:"government_departments"`
			Note                  string `json:"note"`
			Target                string `json:"target"`
			Title                 string `json:"title"`
		} `json:"subsidy"`
	} `json:"hojin-infos"`
}

type gbizFinanceResponse struct {
	HojinInfos []struct {
		Finance struct {
			AccountingStandards string `json:"accounting_standards"`
			FiscalYearCoverPage string `json:"fiscal_year_cover_page"`
			ManagementIndex     []struct {
				Period        string `json:"period"`
				NetSales      int64  `json:"net_sales_summary_of_business_results"`
				NetIncomeLoss int64  `json:"net_income_loss_summary_of_business_results"`
				TotalAssets   int64  `json:"total_assets_summary_of_business_results"`
				NetAssets     int64  `json:"net_assets_summary_of_business_results"`
			} `json:"management_index"`
		} `json:"finance"`
	} `json:"hojin-infos"`
}

type gbizWorkplaceResponse struct {
	HojinInfos []struct {
		WorkplaceInfo struct {
			BaseInfos struct {
				AverageAge                             float64 `json:"average_age"`
				AverageContinuousServiceYears          float64 `json:"average_continuous_service_years"`
				AverageContinuousServiceYearsType      string  `json:"average_continuous_service_years_type"`
				MonthAveragePredeterminedOvertimeHours float64 `json:"month_average_predetermined_overtime_hours"`
			} `json:"base_infos"`
			WomenActivityInfos struct {
				FemaleWorkersProportion     float64 `json:"female_workers_proportion"`
				FemaleWorkersProportionType string  `json:"female_workers_proportion_type"`
			} `json:"women_activity_infos"`
		} `json:"workplace_info"`
	} `json:"hojin-infos"`
}
