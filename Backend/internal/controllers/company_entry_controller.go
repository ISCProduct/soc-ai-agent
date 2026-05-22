package controllers

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type CompanyEntryController struct {
	companyRepo  repository.CompanyRepository
	graduateRepo repository.GraduateEmploymentRepository
}

func NewCompanyEntryController(
	companyRepo repository.CompanyRepository,
	graduateRepo repository.GraduateEmploymentRepository,
) *CompanyEntryController {
	return &CompanyEntryController{
		companyRepo:  companyRepo,
		graduateRepo: graduateRepo,
	}
}

type companyEntryJobPosition struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	JobCategoryID   uint   `json:"job_category_id"`
	MinSalary       int    `json:"min_salary"`
	MaxSalary       int    `json:"max_salary"`
	EmploymentType  string `json:"employment_type"`
	WorkLocation    string `json:"work_location"`
	RemoteOption    bool   `json:"remote_option"`
	RequiredSkills  string `json:"required_skills"`
	PreferredSkills string `json:"preferred_skills"`
}

type companyEntryWeightProfile struct {
	TechnicalOrientation  int `json:"technical_orientation"`
	TeamworkOrientation   int `json:"teamwork_orientation"`
	LeadershipOrientation int `json:"leadership_orientation"`
	CreativityOrientation int `json:"creativity_orientation"`
	StabilityOrientation  int `json:"stability_orientation"`
	GrowthOrientation     int `json:"growth_orientation"`
	WorkLifeBalance       int `json:"work_life_balance"`
	ChallengeSeeking      int `json:"challenge_seeking"`
	DetailOrientation     int `json:"detail_orientation"`
	CommunicationSkill    int `json:"communication_skill"`
}

type companyEntryGraduate struct {
	GraduateName   string `json:"graduate_name"`
	GraduationYear int    `json:"graduation_year"`
	SchoolName     string `json:"school_name"`
	Department     string `json:"department"`
	HiredAt        string `json:"hired_at"`
	Note           string `json:"note"`
}

type companyEntryRequest struct {
	// 企業基本情報
	Name            string `json:"name"`
	Description     string `json:"description"`
	Industry        string `json:"industry"`
	Location        string `json:"location"`
	WebsiteURL      string `json:"website_url"`
	LogoURL         string `json:"logo_url"`
	CorporateNumber string `json:"corporate_number"`

	// 従業員情報
	EmployeeCount int     `json:"employee_count"`
	FoundedYear   int     `json:"founded_year"`
	AverageAge    float64 `json:"average_age"`
	FemaleRatio   float64 `json:"female_ratio"`

	// 企業文化・働き方
	Culture        string `json:"culture"`
	WorkStyle      string `json:"work_style"`
	WelfareDetails string `json:"welfare_details"`

	// 技術情報
	TechStack        string `json:"tech_stack"`
	DevelopmentStyle string `json:"development_style"`

	// 事業内容
	MainBusiness string `json:"main_business"`

	// 関連データ
	JobPositions  []companyEntryJobPosition  `json:"job_positions"`
	WeightProfile *companyEntryWeightProfile `json:"weight_profile"`
	Graduates     []companyEntryGraduate     `json:"graduates"`
}

// Submit POST /api/company-entry
func (c *CompanyEntryController) Submit(ctx echo.Context) error {
	var req companyEntryRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	if strings.TrimSpace(req.Name) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	now := time.Now()
	company := &models.Company{
		Name:             strings.TrimSpace(req.Name),
		Description:      req.Description,
		Industry:         req.Industry,
		Location:         req.Location,
		WebsiteURL:       req.WebsiteURL,
		LogoURL:          req.LogoURL,
		CorporateNumber:  req.CorporateNumber,
		EmployeeCount:    req.EmployeeCount,
		FoundedYear:      req.FoundedYear,
		AverageAge:       req.AverageAge,
		FemaleRatio:      req.FemaleRatio,
		Culture:          req.Culture,
		WorkStyle:        req.WorkStyle,
		WelfareDetails:   req.WelfareDetails,
		TechStack:        req.TechStack,
		DevelopmentStyle: req.DevelopmentStyle,
		MainBusiness:     req.MainBusiness,
		SourceType:       "manual",
		DataStatus:       "draft",
		IsProvisional:    true,
		IsActive:         true,
		SourceFetchedAt:  &now,
	}

	if err := c.companyRepo.Create(company); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create company")
	}

	// 求人情報の保存
	for _, jp := range req.JobPositions {
		if strings.TrimSpace(jp.Title) == "" {
			continue
		}
		position := &models.CompanyJobPosition{
			CompanyID:       company.ID,
			Title:           strings.TrimSpace(jp.Title),
			Description:     jp.Description,
			JobCategoryID:   jp.JobCategoryID,
			MinSalary:       jp.MinSalary,
			MaxSalary:       jp.MaxSalary,
			EmploymentType:  jp.EmploymentType,
			WorkLocation:    jp.WorkLocation,
			RemoteOption:    jp.RemoteOption,
			RequiredSkills:  jp.RequiredSkills,
			PreferredSkills: jp.PreferredSkills,
			IsActive:        true,
		}
		if err := c.companyRepo.CreateJobPosition(position); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create job position")
		}
	}

	// WeightProfile の保存
	if req.WeightProfile != nil {
		profile := &models.CompanyWeightProfile{
			CompanyID:             company.ID,
			TechnicalOrientation:  req.WeightProfile.TechnicalOrientation,
			TeamworkOrientation:   req.WeightProfile.TeamworkOrientation,
			LeadershipOrientation: req.WeightProfile.LeadershipOrientation,
			CreativityOrientation: req.WeightProfile.CreativityOrientation,
			StabilityOrientation:  req.WeightProfile.StabilityOrientation,
			GrowthOrientation:     req.WeightProfile.GrowthOrientation,
			WorkLifeBalance:       req.WeightProfile.WorkLifeBalance,
			ChallengeSeeking:      req.WeightProfile.ChallengeSeeking,
			DetailOrientation:     req.WeightProfile.DetailOrientation,
			CommunicationSkill:    req.WeightProfile.CommunicationSkill,
		}
		if err := c.companyRepo.CreateOrUpdateWeightProfile(profile); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create weight profile")
		}
	}

	// 卒業生就職情報の保存
	for _, g := range req.Graduates {
		var hiredAt *time.Time
		if strings.TrimSpace(g.HiredAt) != "" {
			if parsed, err := time.Parse("2006-01-02", g.HiredAt); err == nil {
				hiredAt = &parsed
			}
		}
		entry := &models.GraduateEmployment{
			CompanyID:      company.ID,
			GraduateName:   strings.TrimSpace(g.GraduateName),
			GraduationYear: g.GraduationYear,
			SchoolName:     strings.TrimSpace(g.SchoolName),
			Department:     strings.TrimSpace(g.Department),
			HiredAt:        hiredAt,
			Note:           strings.TrimSpace(g.Note),
		}
		if err := c.graduateRepo.Create(entry); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create graduate employment")
		}
	}

	return ctx.JSON(http.StatusCreated, map[string]interface{}{
		"message":    "送信が完了しました。内容を確認の上、掲載審査を行います。",
		"company_id": company.ID,
	})
}
