// flywheel-verify はローカル統合テスト DB 上で AI フライホイール各経路が
// 実データとして回るかを検証する（本番/staging では実行しないこと）。
//
// 前提: compose.test の db が起動済み、migrate up 済み。
//
//	cd Backend && go run ./cmd/flywheel-verify
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"Backend/domain/valueobject"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/application"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/matching"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const (
	sessionID = "flywheel-verify-session"
	markEmail = "flywheel-verify@example.com"
)

func main() {
	if err := godotenv.Overload(".env.test"); err != nil {
		log.Fatalf(".env.test を読めない: %v", err)
	}
	host := os.Getenv("DB_HOST")
	if host != "127.0.0.1" && host != "localhost" {
		log.Fatalf("DB_HOST=%q はローカルではない。検証を中止する", host)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	report := &report{checks: nil}
	if err := run(db, report); err != nil {
		report.fail("fatal", err.Error())
	}
	report.print()
	if report.failed > 0 {
		os.Exit(1)
	}
}

func run(db *gorm.DB, r *report) error {
	// 前回の残骸を掃除（冪等）
	cleanup(db)

	user := models.User{
		OrganizationID:         1,
		Email:                  markEmail,
		Name:                   "Flywheel Verify",
		Role:                   "student",
		AllowCollectiveInsight: true,
	}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("user create: %w", err)
	}

	scoreRepo := repositories.NewUserWeightScoreRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	matchRepo := repositories.NewUserCompanyMatchRepository(db)
	appRepo := repositories.NewUserApplicationStatusRepository(db)
	accessRepo := repositories.NewCompanyAccessRepository(db)
	collectiveRepo := repositories.NewCollectiveInsightRepository(db)
	recalcRepo := repositories.NewProfileRecalculationRepository(db)

	cross := flywheel.NewCrossFeatureIntegrationService(scoreRepo)
	matchingSvc := matching.NewMatchingService(scoreRepo, companyRepo, matchRepo, nil)
	collective := flywheel.NewCollectiveInsightService(collectiveRepo, scoreRepo)
	appSvc := application.NewApplicationService(appRepo, matchRepo, accessRepo)
	appSvc.SetCollectiveInsight(collective)
	appSvc.SetScoreSnapshotReader(scoreRepo)
	recalcSvc := flywheel.NewProfileRecalculationService(recalcRepo, companyRepo)

	// ① チャット診断スコア seed
	for _, cat := range valueobject.AllWeightCategories() {
		score := 55
		if cat == valueobject.CategoryTechnical {
			score = 90
		}
		if cat == valueobject.CategoryDetail {
			score = 40
		}
		if err := scoreRepo.SetScore(user.ID, sessionID, string(cat), score); err != nil {
			return fmt.Errorf("seed score %s: %w", cat, err)
		}
	}
	resolved, err := cross.ResolveDiagnosisSessionID(user.ID)
	if err != nil || resolved != sessionID {
		r.fail("①診断セッション解決", fmt.Sprintf("got %q err=%v want %q", resolved, err, sessionID))
	} else {
		r.pass("①診断セッション解決", "ResolveDiagnosisSessionID → "+resolved)
	}

	// 公開企業 + プロファイル
	company := models.Company{
		Name:       "Flywheel Verify Corp",
		DataStatus: "published",
		IsActive:   true,
		Industry:   "IT",
	}
	if err := db.Create(&company).Error; err != nil {
		return fmt.Errorf("company: %w", err)
	}
	profile := models.CompanyWeightProfile{
		CompanyID:             company.ID,
		TechnicalOrientation:  90,
		TeamworkOrientation:   50,
		LeadershipOrientation: 50,
		CreativityOrientation: 50,
		StabilityOrientation:  50,
		GrowthOrientation:     50,
		WorkLifeBalance:       50,
		ChallengeSeeking:      50,
		DetailOrientation:     40,
		CommunicationSkill:    50,
	}
	if err := db.Create(&profile).Error; err != nil {
		return fmt.Errorf("profile: %w", err)
	}

	// ② チャット → マッチング
	if err := matchingSvc.CalculateMatching(context.Background(), user.ID, sessionID); err != nil {
		r.fail("②チャット→マッチング", err.Error())
	} else {
		var matchCount int64
		db.Model(&models.UserCompanyMatch{}).
			Where("user_id = ? AND session_id = ?", user.ID, sessionID).
			Count(&matchCount)
		if matchCount == 0 {
			r.fail("②チャット→マッチング", "matches=0")
		} else {
			r.pass("②チャット→マッチング", fmt.Sprintf("matches=%d", matchCount))
		}
	}

	// ③ 面接スコア → UserWeightScore
	beforeComm, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, sessionID, "コミュニケーション力")
	reportJSON, _ := json.Marshal(map[string]int{
		"communication": 5,
		"logic":         4,
		"specificity":   4,
		"ownership":     3,
		"enthusiasm":    5,
	})
	ivReport := &models.InterviewReport{ScoresJSON: string(reportJSON), EvidenceJSON: "{}"}
	if err := cross.UpdateScoresFromInterviewReport(user.ID, sessionID, ivReport, flywheel.InterviewTranscriptStats{}); err != nil {
		r.fail("③面接→スコア", err.Error())
	} else {
		afterComm, err := scoreRepo.FindByUserSessionAndCategory(user.ID, sessionID, "コミュニケーション力")
		if err != nil || afterComm == nil || (beforeComm != nil && afterComm.Score == beforeComm.Score) {
			r.fail("③面接→スコア", fmt.Sprintf("コミュニケーション力が変わっていない before=%v after=%v err=%v",
				scoreOf(beforeComm), scoreOf(afterComm), err))
		} else {
			r.pass("③面接→スコア", fmt.Sprintf("コミュニケーション力 %d→%d", scoreOf(beforeComm), scoreOf(afterComm)))
		}
	}

	// ④ 面接コンテキスト注入
	ctxText := cross.BuildInterviewContextFromUser(user.ID)
	if !strings.Contains(ctxText, "技術志向") {
		r.fail("④面接コンテキスト注入", "強み傾向が空: "+truncate(ctxText, 80))
	} else {
		r.pass("④面接コンテキスト注入", truncate(ctxText, 60))
	}

	// ⑤ 履歴書 → スコア（診断セッションへ）+ 再マッチ
	beforeDetail, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, sessionID, "細部志向")
	resumeScore := 95
	if err := cross.UpdateScoresFromResumeReview(user.ID, sessionID, &models.ResumeReview{Score: &resumeScore}, nil); err != nil {
		r.fail("⑤履歴書→スコア", err.Error())
	} else {
		afterDetail, err := scoreRepo.FindByUserSessionAndCategory(user.ID, sessionID, "細部志向")
		if err != nil || afterDetail == nil || scoreOf(afterDetail) == scoreOf(beforeDetail) {
			r.fail("⑤履歴書→スコア", fmt.Sprintf("細部志向が変わっていない before=%v after=%v", scoreOf(beforeDetail), scoreOf(afterDetail)))
		} else {
			r.pass("⑤履歴書→スコア", fmt.Sprintf("細部志向 %d→%d（診断 session へ）", scoreOf(beforeDetail), scoreOf(afterDetail)))
		}
	}
	if err := matchingSvc.CalculateMatching(context.Background(), user.ID, sessionID); err != nil {
		r.fail("⑤履歴書後再マッチ", err.Error())
	} else {
		r.pass("⑤履歴書後再マッチ", "CalculateMatching OK")
	}

	// ⑥ 履歴書 RAG コンテキスト
	ragCtx := cross.BuildResumeContextFromUser(user.ID)
	if !strings.Contains(ragCtx, "候補者の強み傾向") {
		r.fail("⑥履歴書RAGコンテキスト", truncate(ragCtx, 80))
	} else {
		r.pass("⑥履歴書RAGコンテキスト", truncate(ragCtx, 60))
	}

	// ⑦ 応募 → 集合知 applied
	var match models.UserCompanyMatch
	if err := db.Where("user_id = ? AND session_id = ? AND company_id = ?", user.ID, sessionID, company.ID).
		First(&match).Error; err != nil {
		return fmt.Errorf("match lookup: %w", err)
	}
	beforeLogs := countCollective(db, company.ID, "applied")
	if _, err := appSvc.Apply(user.ID, company.ID, match.ID); err != nil {
		r.fail("⑦応募→集合知applied", err.Error())
	} else {
		afterLogs := countCollective(db, company.ID, "applied")
		if afterLogs <= beforeLogs {
			r.fail("⑦応募→集合知applied", fmt.Sprintf("logs %d→%d", beforeLogs, afterLogs))
		} else {
			r.pass("⑦応募→集合知applied", fmt.Sprintf("collective_insight_logs applied %d→%d", beforeLogs, afterLogs))
		}
	}

	// ⑧ 通過 → 集合知 passed
	var app models.UserApplicationStatus
	if err := db.Where("user_id = ? AND company_id = ?", user.ID, company.ID).First(&app).Error; err != nil {
		return fmt.Errorf("app lookup: %w", err)
	}
	// applied → document_screening → document_passed
	if _, err := appSvc.UpdateStatus(app.ID, user.ID, "document_screening", nil, true); err != nil {
		r.fail("⑧通過準備", "document_screening: "+err.Error())
	} else {
		beforePass := countCollective(db, company.ID, "passed")
		if _, err := appSvc.UpdateStatus(app.ID, user.ID, "document_passed", nil, true); err != nil {
			r.fail("⑧通過→集合知passed", err.Error())
		} else {
			afterPass := countCollective(db, company.ID, "passed")
			if afterPass <= beforePass {
				r.fail("⑧通過→集合知passed", fmt.Sprintf("logs %d→%d", beforePass, afterPass))
			} else {
				r.pass("⑧通過→集合知passed", fmt.Sprintf("collective_insight_logs passed %d→%d", beforePass, afterPass))
			}
		}
	}

	// ⑨ 企業プロファイル再計算（通過サンプルが1件だと閾値でスキップされうる）
	// 閾値を1にして更新されることを確認する
	res, err := recalcSvc.RecalculateCompany(company.ID, 1)
	if err != nil {
		r.fail("⑨企業プロファイル再計算", err.Error())
	} else if res == nil || !res.Updated {
		r.fail("⑨企業プロファイル再計算", fmt.Sprintf("updated=%v skip=%q samples=%d",
			res != nil && res.Updated, skipOf(res), sampleOf(res)))
	} else {
		r.pass("⑨企業プロファイル再計算", fmt.Sprintf("updated samples=%d", res.SampleCount))
	}

	// ⑩ 集合知レコメンド（類似ユーザーが必要なので 2人目を用意）
	peer := models.User{
		OrganizationID:         1,
		Email:                  "flywheel-verify-peer@example.com",
		Name:                   "Flywheel Peer",
		Role:                   "student",
		AllowCollectiveInsight: true,
	}
	if err := db.Create(&peer).Error; err != nil {
		return fmt.Errorf("peer user: %w", err)
	}
	peerSession := sessionID + "-peer"
	for _, cat := range valueobject.AllWeightCategories() {
		score := 55
		if cat == valueobject.CategoryTechnical {
			score = 88
		}
		if cat == valueobject.CategoryDetail {
			score = 42
		}
		_ = scoreRepo.SetScore(peer.ID, peerSession, string(cat), score)
	}
	if err := matchingSvc.CalculateMatching(context.Background(), peer.ID, peerSession); err != nil {
		return fmt.Errorf("peer match: %w", err)
	}
	var peerMatch models.UserCompanyMatch
	if err := db.Where("user_id = ? AND session_id = ? AND company_id = ?", peer.ID, peerSession, company.ID).
		First(&peerMatch).Error; err != nil {
		return fmt.Errorf("peer match lookup: %w", err)
	}
	if _, err := appSvc.Apply(peer.ID, company.ID, peerMatch.ID); err != nil {
		// 同一企業に2人目が応募できる前提。失敗したら Recalculate 用の通過は1人目で足りる
		log.Printf("peer Apply: %v (continue)", err)
	} else {
		var peerApp models.UserApplicationStatus
		if err := db.Where("user_id = ? AND company_id = ?", peer.ID, company.ID).First(&peerApp).Error; err == nil {
			_, _ = appSvc.UpdateStatus(peerApp.ID, peer.ID, "document_screening", nil, true)
			_, _ = appSvc.UpdateStatus(peerApp.ID, peer.ID, "document_passed", nil, true)
		}
	}

	items, err := collective.GetCollectiveRecommendations(user.ID, sessionID, nil)
	if err != nil {
		r.fail("⑩集合知レコメンド", err.Error())
	} else if len(items) == 0 {
		r.fail("⑩集合知レコメンド", "items=0（類似ユーザーの通過ログから企業が引けない）")
	} else {
		r.pass("⑩集合知レコメンド", fmt.Sprintf("items=%d first=%s score=%.1f", len(items), items[0].CompanyName, items[0].CollectiveScore))
	}

	return nil
}

func cleanup(db *gorm.DB) {
	emails := []string{markEmail, "flywheel-verify-peer@example.com"}
	var userIDs []uint
	for _, email := range emails {
		var u models.User
		if err := db.Where("email = ?", email).First(&u).Error; err == nil {
			userIDs = append(userIDs, u.ID)
			db.Where("user_id = ?", u.ID).Delete(&models.UserWeightScore{})
			db.Where("user_id = ?", u.ID).Delete(&models.UserCompanyMatch{})
			db.Where("user_id = ?", u.ID).Delete(&models.UserApplicationStatus{})
			db.Where("email = ?", email).Delete(&models.User{})
		}
	}
	_ = userIDs
	var companies []models.Company
	db.Unscoped().Where("name = ?", "Flywheel Verify Corp").Find(&companies)
	for _, c := range companies {
		db.Where("company_id = ?", c.ID).Delete(&models.CompanyWeightProfile{})
		db.Where("company_id = ?", c.ID).Delete(&models.CollectiveInsightLog{})
		db.Where("company_id = ?", c.ID).Delete(&models.AnonymizedBehaviorSummary{})
		db.Unscoped().Delete(&c)
	}
}

func countCollective(db *gorm.DB, companyID uint, action string) int64 {
	var n int64
	db.Model(&models.CollectiveInsightLog{}).
		Where("company_id = ? AND action_type = ?", companyID, action).
		Count(&n)
	return n
}

func scoreOf(s *models.UserWeightScore) int {
	if s == nil {
		return -1
	}
	return s.Score
}

func skipOf(r *flywheel.RecalculationResult) string {
	if r == nil {
		return ""
	}
	return r.SkipReason
}

func sampleOf(r *flywheel.RecalculationResult) int {
	if r == nil {
		return 0
	}
	return r.SampleCount
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

type check struct {
	name   string
	ok     bool
	detail string
}

type report struct {
	checks []check
	failed int
}

func (r *report) pass(name, detail string) {
	r.checks = append(r.checks, check{name, true, detail})
	log.Printf("PASS  %s — %s", name, detail)
}

func (r *report) fail(name, detail string) {
	r.failed++
	r.checks = append(r.checks, check{name, false, detail})
	log.Printf("FAIL  %s — %s", name, detail)
}

func (r *report) print() {
	fmt.Println()
	fmt.Println("=== AI Flywheel Verify ===")
	fmt.Printf("time: %s\n", time.Now().Format(time.RFC3339))
	ok := 0
	for _, c := range r.checks {
		mark := "FAIL"
		if c.ok {
			mark = "PASS"
			ok++
		}
		fmt.Printf("[%s] %s\n       %s\n", mark, c.name, c.detail)
	}
	fmt.Printf("\nresult: %d/%d passed\n", ok, len(r.checks))
}
