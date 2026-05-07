package models

import "gorm.io/gorm"

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&User{},
		&Industry{},
		&JobCategory{},
		&IndustryJobCategory{},
		&AIQuestionTemplate{},
		&ConversationContext{},
		&AIGeneratedQuestion{},
		&WeightRule{},
		&QuestionWeight{},
		&PredefinedQuestion{}, // 事前定義質問（ルールベース判定用）
		&ChatMessage{},
		&UserWeightScore{},
		&AnalysisPhase{},
		&UserAnalysisProgress{},
		&SessionValidation{},
		&UserEmbedding{},
		&JobCategoryEmbedding{},
		// 企業関連モデル
		&Company{},
		&CompanyJobPosition{},
		&CompanyWeightProfile{},
		&UserCompanyMatch{},
		&UserApplicationStatus{},
		&CompanyProfileUpdateHistory{},
		&CompanyReview{},
		&CompanyBenefit{},
		&GBizCompanyProfile{},
		&GBizProcurement{},
		&GBizSubsidy{},
		&GBizFinance{},
		&GBizWorkplace{},
		&CompanyRelation{},
		&CompanyMarketInfo{},
		&CompanyPopularityRecord{},
		&GraduateEmployment{},
		&ResumeDocument{},
		&ResumeTextBlock{},
		&ResumeReview{},
		&ResumeReviewItem{},
		&CrawlSource{},
		&CrawlRun{},
		&ScraperSession{},
		&AuditLog{},
		&InterviewSession{},
		&InterviewUtterance{},
		&InterviewReport{},
		&InterviewVideo{},
		&RealtimeUsageLog{},
		&PendingRegistration{},
		// 選考スケジュール
		&ScheduleEvent{},
		// APIコストモニタリング
		&APICallLog{},
		// 集合知レコメンド
		&CollectiveInsightLog{},
		&AnonymizedBehaviorSummary{},
		// スコア精度検証・A/Bテスト
		&QuestionVariant{},
		&VariantAssignment{},
		&ScoreCalibrationWeight{},
		// GitHub連携
		&GitHubProfile{},
		&GitHubRepo{},
		&GitHubLanguageStat{},
		&GitHubRepoSummary{},
		&SkillScore{},
	)
}
