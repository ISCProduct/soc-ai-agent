package main

// ローカル/運用向け: 中央寄りの company_weight_profiles にコントラストを付け直し、
// 既存マッチを再計算する。LLM は呼ばない。

import (
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/company"
	"Backend/internal/services/matching"
	"context"
	"fmt"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	var profiles []models.CompanyWeightProfile
	if err := db.Where("job_position_id IS NULL").Find(&profiles).Error; err != nil {
		log.Fatalf("load profiles: %v", err)
	}

	updated := 0
	for i := range profiles {
		p := &profiles[i]
		if !company.EnsureProfileContrast(p) {
			continue
		}
		if err := db.Model(p).Select(
			"technical_orientation", "teamwork_orientation", "leadership_orientation",
			"creativity_orientation", "stability_orientation", "growth_orientation",
			"work_life_balance", "challenge_seeking", "detail_orientation", "communication_skill",
		).Updates(p).Error; err != nil {
			log.Printf("update company_id=%d failed: %v", p.CompanyID, err)
			continue
		}
		updated++
	}
	fmt.Printf("profiles: total=%d updated=%d\n", len(profiles), updated)

	type pair struct {
		UserID    uint
		SessionID string
	}
	var pairs []pair
	if err := db.Model(&models.UserCompanyMatch{}).
		Select("DISTINCT user_id, session_id").
		Find(&pairs).Error; err != nil {
		log.Fatalf("list sessions: %v", err)
	}

	scoreRepo := repositories.NewUserWeightScoreRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	matchRepo := repositories.NewUserCompanyMatchRepository(db)
	svc := matching.NewMatchingService(scoreRepo, companyRepo, matchRepo, nil)

	rematched := 0
	for _, p := range pairs {
		if err := svc.CalculateMatching(context.Background(), p.UserID, p.SessionID); err != nil {
			log.Printf("rematch user=%d session=%s: %v", p.UserID, p.SessionID, err)
			continue
		}
		rematched++
		matches, err := matchRepo.FindTopMatchesByUserAndSession(p.UserID, p.SessionID, 5)
		if err == nil && len(matches) > 0 {
			fmt.Printf("rematch ok user=%d session=%s top=%.1f axes=%d\n",
				p.UserID, p.SessionID, matches[0].MatchScore, matches[0].MatchedAxisCount)
		}
	}
	fmt.Printf("sessions rematched=%d/%d\n", rematched, len(pairs))
}
