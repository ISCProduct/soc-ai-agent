package controllers

import (
	"Backend/internal/middleware"
	"Backend/internal/services"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type CompanyEntryController struct {
	service *services.CompanyEntryService
}

func NewCompanyEntryController(service *services.CompanyEntryService) *CompanyEntryController {
	return &CompanyEntryController{service: service}
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
	Name             string                     `json:"name"`
	Description      string                     `json:"description"`
	Industry         string                     `json:"industry"`
	Location         string                     `json:"location"`
	WebsiteURL       string                     `json:"website_url"`
	LogoURL          string                     `json:"logo_url"`
	CorporateNumber  string                     `json:"corporate_number"`
	EmployeeCount    int                        `json:"employee_count"`
	FoundedYear      int                        `json:"founded_year"`
	AverageAge       float64                    `json:"average_age"`
	FemaleRatio      float64                    `json:"female_ratio"`
	Culture          string                     `json:"culture"`
	WorkStyle        string                     `json:"work_style"`
	WelfareDetails   string                     `json:"welfare_details"`
	TechStack        string                     `json:"tech_stack"`
	DevelopmentStyle string                     `json:"development_style"`
	MainBusiness     string                     `json:"main_business"`
	JobPositions     []companyEntryJobPosition  `json:"job_positions"`
	WeightProfile    *companyEntryWeightProfile `json:"weight_profile"`
	Graduates        []companyEntryGraduate     `json:"graduates"`

	ContactEmail   string `json:"contact_email"`
	ContactName    string `json:"contact_name"`
	PrivacyConsent bool   `json:"privacy_consent"`
	// ハニーポット（見た目は非表示）。ボットが埋めると拒否する
	CompanyFax string `json:"company_fax"`
}

// Submit POST /api/company-entry
func (c *CompanyEntryController) Submit(ctx echo.Context) error {
	var req companyEntryRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}

	in := services.CompanyEntryInput{
		Name:             req.Name,
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
		ContactEmail:     req.ContactEmail,
		ContactName:      req.ContactName,
		PrivacyConsent:   req.PrivacyConsent,
		Honeypot:         req.CompanyFax,
		SourceIP:         middleware.GetClientIP(ctx.Request()),
	}
	for _, jp := range req.JobPositions {
		in.JobPositions = append(in.JobPositions, services.CompanyEntryJobInput{
			Title:           jp.Title,
			Description:     jp.Description,
			JobCategoryID:   jp.JobCategoryID,
			MinSalary:       jp.MinSalary,
			MaxSalary:       jp.MaxSalary,
			EmploymentType:  jp.EmploymentType,
			WorkLocation:    jp.WorkLocation,
			RemoteOption:    jp.RemoteOption,
			RequiredSkills:  jp.RequiredSkills,
			PreferredSkills: jp.PreferredSkills,
		})
	}
	if req.WeightProfile != nil {
		in.WeightProfile = &services.CompanyEntryWeightInput{
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
	}
	for _, g := range req.Graduates {
		in.Graduates = append(in.Graduates, services.CompanyEntryGraduateInput{
			GraduateName:   g.GraduateName,
			GraduationYear: g.GraduationYear,
			SchoolName:     g.SchoolName,
			Department:     g.Department,
			HiredAt:        g.HiredAt,
			Note:           g.Note,
		})
	}

	result, err := c.service.Submit(in)
	if err != nil {
		msg := err.Error()
		switch {
		case msg == "rejected":
			// ハニーポットは成功風に返す（ボットにヒントを与えない）
			return ctx.JSON(http.StatusCreated, map[string]any{"message": "送信が完了しました。"})
		case strings.Contains(msg, "required"), strings.Contains(msg, "invalid"), strings.Contains(msg, "must be"):
			return echo.NewHTTPError(http.StatusBadRequest, msg)
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create company")
		}
	}

	return ctx.JSON(http.StatusCreated, map[string]any{
		"message":       result.Message,
		"company_id":    result.CompanyID,
		"submission_id": result.SubmissionID,
		"email_queued":  result.EmailQueued,
	})
}

// ResendEmail POST /api/admin/company-entry-submissions/:id/resend-email
func (c *CompanyEntryController) ResendEmail(ctx echo.Context) error {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil || id == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}
	if err := c.service.ResendEmail(uint(id)); err != nil {
		if errors.Is(err, services.ErrCompanyEntrySubmissionNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "submission not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return ctx.JSON(http.StatusOK, map[string]string{"message": "email resent"})
}
