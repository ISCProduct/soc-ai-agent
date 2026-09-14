package entity

import "time"

// Company 企業ドメインエンティティ
type Company struct {
	ID                 uint
	Name               string
	Description        string
	Industry           string
	EmployeeCount      int
	EmployeeCountBasis string
	FoundedYear        int
	Location           string
	WebsiteURL         string
	LogoURL            string
	CorporateNumber    string
	SourceType         string
	SourceURL          string
	SourceFetchedAt    *time.Time
	IsProvisional      bool
	DataStatus         string // draft, published
	Culture            string
	WorkStyle          string
	WelfareDetails     string
	TechStack          string
	DevelopmentStyle   string
	MainBusiness       string
	AverageAge         float64
	FemaleRatio        float64
	IsActive           bool
	IsVerified         bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// UserApplicationStatus 応募・選考ステータスエンティティ
type UserApplicationStatus struct {
	ID              uint
	UserID          uint
	CompanyID       uint
	Company         *Company
	MatchID         uint
	Status          string // ValidStatuses: not_applied / applied / document_screening / document_passed / interview_scheduled / interview_in_progress / offered / accepted / withdrawn / rejected
	Notes           string
	AppliedAt       *time.Time
	StatusUpdatedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// IsPublished 公開済みかどうか
func (c *Company) IsPublished() bool {
	return c.DataStatus == "published" && c.IsActive
}

// CompanyWeightProfile 企業の適性プロファイル（10カテゴリの重視度）
type CompanyWeightProfile struct {
	ID                    uint
	CompanyID             uint
	JobPositionID         *uint
	TechnicalOrientation  int // 技術志向 (0-100)
	TeamworkOrientation   int // チームワーク志向 (0-100)
	LeadershipOrientation int // リーダーシップ志向 (0-100)
	CreativityOrientation int // 創造性志向 (0-100)
	StabilityOrientation  int // 安定志向 (0-100)
	GrowthOrientation     int // 成長志向 (0-100)
	WorkLifeBalance       int // ワークライフバランス (0-100)
	ChallengeSeeking      int // チャレンジ志向 (0-100)
	DetailOrientation     int // 細部志向 (0-100)
	CommunicationSkill    int // コミュニケーション力 (0-100)
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// UserCompanyMatch ユーザーと企業のマッチング結果
type UserCompanyMatch struct {
	ID                 uint
	UserID             uint
	SessionID          string
	CompanyID          uint
	Company            *Company
	MatchScore         float64 // 総合マッチ度（0-100）
	TechnicalMatch     float64
	TeamworkMatch      float64
	LeadershipMatch    float64
	CreativityMatch    float64
	StabilityMatch     float64
	GrowthMatch        float64
	WorkLifeMatch      float64
	ChallengeMatch     float64
	DetailMatch        float64
	CommunicationMatch float64
	// MatchedAxisCount は MatchScore の算出に使えた軸の数（0-10）。
	// 未計測の軸は平均に含めないため、この値が小さいほど根拠が薄い（#1124）。
	//
	// スコア行が存在すれば値が0でも「計測済み」と数える。面接経路の 0 は
	// 「5点中0点」を正規化した正当なスコア（cross_feature_integration_service.go）なので、
	// 測れていないわけではない。
	//
	// なお chat_controller.go の countEvaluatedCategories は score != 0 で数えるため、
	// 同じ「何軸で測れたか」でも値が食い違う。あちらは 0 を未計測とみなす簡略化。
	MatchedAxisCount int
	MatchReason      string
	IsViewed         bool
	IsFavorited      bool
	IsApplied        bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
