// flywheel-verify-extra はコア以外のフライホイール経路を実DBで検証する。
//
//	cd Backend && go run ./cmd/flywheel-verify-extra
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"Backend/domain/valueobject"
	"Backend/internal/config"
	"Backend/internal/models"
	"Backend/internal/repositories"
	"Backend/internal/services/admin"
	"Backend/internal/services/application"
	"Backend/internal/services/esreview"
	"Backend/internal/services/flywheel"
	"Backend/internal/services/matching"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const (
	email   = "flywheel-extra@example.com"
	session = "flywheel-extra-session"
)

func main() {
	if err := godotenv.Overload(".env.test"); err != nil {
		log.Fatalf(".env.test: %v", err)
	}
	if h := os.Getenv("DB_HOST"); h != "127.0.0.1" && h != "localhost" {
		log.Fatalf("DB_HOST=%q not local", h)
	}
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatal(err)
	}

	r := &report{}
	if err := run(db, r); err != nil {
		r.fail("fatal", err.Error())
	}
	r.print()
	if r.failed > 0 {
		os.Exit(1)
	}
}

func run(db *gorm.DB, r *report) error {
	cleanup(db)

	user := models.User{
		OrganizationID: 1, Email: email, Name: "Flywheel Extra",
		Role: "student", AllowCollectiveInsight: true,
	}
	if err := db.Create(&user).Error; err != nil {
		return err
	}

	scoreRepo := repositories.NewUserWeightScoreRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	matchRepo := repositories.NewUserCompanyMatchRepository(db)
	appRepo := repositories.NewUserApplicationStatusRepository(db)
	accessRepo := repositories.NewCompanyAccessRepository(db)
	collectiveRepo := repositories.NewCollectiveInsightRepository(db)
	esRepo := repositories.NewESReviewRepository(db)
	scoreValRepo := repositories.NewScoreValidationRepository(db)

	cross := flywheel.NewCrossFeatureIntegrationService(scoreRepo)
	matchingSvc := matching.NewMatchingService(scoreRepo, companyRepo, matchRepo, nil)
	matchingSvc.SetCalibrationWeightReader(scoreValRepo)
	collective := flywheel.NewCollectiveInsightService(collectiveRepo, scoreRepo)
	appSvc := application.NewApplicationService(appRepo, matchRepo, accessRepo)
	appSvc.SetCollectiveInsight(collective)
	appSvc.SetScoreSnapshotReader(scoreRepo)
	esSvc := esreview.NewService(esRepo, cross)
	scoreVal := admin.NewScoreValidationService(scoreValRepo)

	// seed diagnosis scores
	for _, cat := range valueobject.AllWeightCategories() {
		_ = scoreRepo.SetScore(user.ID, session, string(cat), 60)
	}

	company := models.Company{Name: "Flywheel Extra Corp", DataStatus: "published", IsActive: true, Industry: "IT"}
	if err := db.Create(&company).Error; err != nil {
		return err
	}
	_ = db.Create(&models.CompanyWeightProfile{
		CompanyID:            company.ID,
		TechnicalOrientation: 70, TeamworkOrientation: 50, LeadershipOrientation: 50,
		CreativityOrientation: 50, StabilityOrientation: 50, GrowthOrientation: 50,
		WorkLifeBalance: 50, ChallengeSeeking: 50, DetailOrientation: 50, CommunicationSkill: 50,
	}).Error
	if err := matchingSvc.CalculateMatching(context.Background(), user.ID, session); err != nil {
		return err
	}
	var match models.UserCompanyMatch
	if err := db.Where("user_id = ? AND company_id = ?", user.ID, company.ID).First(&match).Error; err != nil {
		return err
	}

	// A. ES → スコア（診断セッションへ）
	// Save の戻り値は MarkScored 前の構造体なので ScoredAt はメモリ上 nil のまま。
	// 成否は DB の scored_at とスコア変化で見る。
	beforeComm, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "コミュニケーション力")
	esText := fmt.Sprintf("学生時代に塾講師として生徒の成績を伸ばしました。具体的には週3回の補習を設計し平均点を15点上げました。%d", time.Now().UnixNano())
	rev, _, err := esSvc.Save(esreview.Input{
		UserID: user.ID, OrganizationID: 1,
		Question: "学生時代に力を入れたこと", QuestionType: "gakuchika",
		ESText: esText, SpecificityScore: 9, StarScore: 8, LengthBalanceScore: 7,
		Feedback: "良い", ImprovedText: "改善案",
	})
	if err != nil {
		r.fail("A.ES→スコア", err.Error())
	} else if rev == nil {
		r.fail("A.ES→スコア", "review nil")
	} else {
		var stored models.ESReview
		_ = db.First(&stored, rev.ID).Error
		afterComm, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "コミュニケーション力")
		if stored.ScoredAt == nil {
			r.fail("A.ES→スコア", fmt.Sprintf("DB scored_at nil score %d→%d", scoreOf(beforeComm), scoreOf(afterComm)))
		} else if scoreOf(afterComm) == scoreOf(beforeComm) {
			r.fail("A.ES→スコア", fmt.Sprintf("コミュニケーション力 unchanged %d", scoreOf(beforeComm)))
		} else {
			r.pass("A.ES→スコア", fmt.Sprintf("コミュニケーション力 %d→%d DB scored_at OK", scoreOf(beforeComm), scoreOf(afterComm)))
		}
	}

	// B. ES 同一本文の再投稿は動かさない
	before2, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "コミュニケーション力")
	_, _, err = esSvc.Save(esreview.Input{
		UserID: user.ID, OrganizationID: 1,
		Question: "学生時代に力を入れたこと", QuestionType: "gakuchika",
		ESText: esText, SpecificityScore: 10, StarScore: 10, LengthBalanceScore: 10,
		Feedback: "良い2", ImprovedText: "改善案2",
	})
	after2, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "コミュニケーション力")
	if err != nil {
		r.fail("B.ES同一本文ガード", err.Error())
	} else if scoreOf(after2) != scoreOf(before2) {
		r.fail("B.ES同一本文ガード", fmt.Sprintf("動いてしまった %d→%d", scoreOf(before2), scoreOf(after2)))
	} else {
		r.pass("B.ES同一本文ガード", "同一本文ではスコア不動")
	}

	// C. 診断セッション無しユーザーは ES 反映しない
	orphan := models.User{OrganizationID: 1, Email: "flywheel-extra-orphan@example.com", Name: "Orphan", Role: "student"}
	_ = db.Create(&orphan).Error
	applied, err := cross.UpdateScoresFromESReview(orphan.ID, &models.ESReview{
		SpecificityScore: 9, StarScore: 9, LengthBalanceScore: 9,
	})
	if err != nil {
		r.fail("C.ES診断なしスキップ", err.Error())
	} else if applied {
		r.fail("C.ES診断なしスキップ", "applied=true は不可")
	} else {
		var n int64
		db.Model(&models.UserWeightScore{}).Where("user_id = ?", orphan.ID).Count(&n)
		if n != 0 {
			r.fail("C.ES診断なしスキップ", fmt.Sprintf("scores written=%d", n))
		} else {
			r.pass("C.ES診断なしスキップ", "診断なしでは書き込まない")
		}
	}

	// D. 履歴書スコア→診断セッション + 再マッチ（persistReview と同じ部品を実DBで）
	// ResumeService.persistReview の配線自体は go test persist_flywheel で固定済み。
	target, err := cross.ResolveDiagnosisSessionID(user.ID)
	if err != nil || target != session {
		r.fail("D.履歴書→再マッチ実DB", fmt.Sprintf("resolve=%q err=%v", target, err))
	} else {
		beforeDetail, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "細部志向")
		score := 92
		done := make(chan error, 1)
		if err := cross.UpdateScoresFromResumeReview(user.ID, target, &models.ResumeReview{Score: &score}, nil); err != nil {
			r.fail("D.履歴書→再マッチ実DB", err.Error())
		} else {
			go func() { done <- matchingSvc.CalculateMatching(context.Background(), user.ID, target) }()
			select {
			case err := <-done:
				afterDetail, _ := scoreRepo.FindByUserSessionAndCategory(user.ID, session, "細部志向")
				if err != nil {
					r.fail("D.履歴書→再マッチ実DB", "rematch: "+err.Error())
				} else if scoreOf(afterDetail) == scoreOf(beforeDetail) {
					r.fail("D.履歴書→再マッチ実DB", "細部志向 unchanged")
				} else {
					r.pass("D.履歴書→再マッチ実DB", fmt.Sprintf("細部志向 %d→%d + rematch OK", scoreOf(beforeDetail), scoreOf(afterDetail)))
				}
			case <-time.After(10 * time.Second):
				r.fail("D.履歴書→再マッチ実DB", "rematch timeout")
			}
		}
	}

	// E. 応募時 score_snapshot + 集合知 rejected
	if _, err := appSvc.Apply(user.ID, company.ID, match.ID); err != nil {
		r.fail("E.集合知rejected準備", err.Error())
	} else {
		var app models.UserApplicationStatus
		_ = db.Where("user_id = ? AND company_id = ?", user.ID, company.ID).First(&app).Error
		if app.ScoreSnapshot == "" || app.ScoreSnapshot == "{}" {
			r.fail("E.#203応募score_snapshot", "score_snapshot empty")
		} else if !containsCategory(app.ScoreSnapshot, "技術志向") {
			r.fail("E.#203応募score_snapshot", "missing 技術志向: "+app.ScoreSnapshot)
		} else {
			r.pass("E.#203応募score_snapshot", "snapshot len="+fmt.Sprintf("%d", len(app.ScoreSnapshot)))
		}
		_, _ = appSvc.UpdateStatus(app.ID, user.ID, "document_screening", nil, true)
		before := countAction(db, company.ID, "rejected")
		if _, err := appSvc.UpdateStatus(app.ID, user.ID, "rejected", nil, true); err != nil {
			r.fail("E.集合知rejected", err.Error())
		} else {
			after := countAction(db, company.ID, "rejected")
			if after <= before {
				r.fail("E.集合知rejected", fmt.Sprintf("%d→%d", before, after))
			} else {
				r.pass("E.集合知rejected", fmt.Sprintf("rejected logs %d→%d", before, after))
			}
		}
	}

	// F. RecordAction viewed + RebuildSummaries
	if err := collective.RecordAction(user.ID, session, company.ID, "viewed"); err != nil {
		r.fail("F.viewed+rebuild", err.Error())
	} else if err := collective.RebuildSummaries(); err != nil {
		r.fail("F.viewed+rebuild", "rebuild: "+err.Error())
	} else {
		var sum models.AnonymizedBehaviorSummary
		err := db.Where("company_id = ?", company.ID).First(&sum).Error
		if err != nil {
			r.fail("F.viewed+rebuild", "summary missing: "+err.Error())
		} else if sum.ViewCount < 1 && sum.ApplyCount < 1 {
			r.fail("F.viewed+rebuild", fmt.Sprintf("view=%d apply=%d pass=%d", sum.ViewCount, sum.ApplyCount, sum.PassCount))
		} else {
			r.pass("F.viewed+rebuild", fmt.Sprintf("summary view=%d apply=%d pass=%d passRate=%.1f",
				sum.ViewCount, sum.ApplyCount, sum.PassCount, sum.PassRate))
		}
	}

	// G. #203 キャリブレーションAPI + 自動適用 + マッチング適用
	cal, err := scoreVal.RunCalibration()
	switch {
	case errors.Is(err, admin.ErrInsufficientSamples):
		r.pass("G.#203キャリブレーションAPI", "サンプル不足で期待どおり拒否: "+err.Error())
	case errors.Is(err, admin.ErrCalibrationUnchanged):
		r.pass("G.#203キャリブレーションAPI", "変化なしスキップ: "+err.Error())
	case err != nil:
		r.fail("G.#203キャリブレーションAPI", err.Error())
	case cal == nil || len(cal.Weights) == 0:
		r.fail("G.#203キャリブレーションAPI", "weights empty")
	default:
		r.pass("G.#203キャリブレーションAPI", fmt.Sprintf("version=%d weights=%d msg=%s", cal.Version, len(cal.Weights), cal.Message))
	}

	// 確定結果＋スナップショットを十分に積み、自動キャリブレーションが重みを保存できることを確認
	_ = db.Model(&models.ScoreCalibrationWeight{}).Where("is_active = ?", true).Update("is_active", false).Error
	if err := seedDecidedSnapshotsForCalibration(db, company.ID); err != nil {
		r.fail("G.#203自動キャリブレーション", "seed: "+err.Error())
	} else {
		auto, err := scoreVal.AutoCalibrateIfDue(time.Now().UTC().Add(25 * time.Hour))
		switch {
		case err != nil:
			r.fail("G.#203自動キャリブレーション", err.Error())
		case auto == nil || len(auto.Weights) == 0:
			r.fail("G.#203自動キャリブレーション", "result empty（サンプル/変化なし）")
		default:
			techW := 1.0
			for _, w := range auto.Weights {
				if w.Category == "技術志向" {
					techW = w.Weight
				}
			}
			if techW <= 1.0 {
				r.fail("G.#203自動キャリブレーション", fmt.Sprintf("技術志向 weight=%.2f want >1（高スコア→通過の信号）", techW))
			} else {
				r.pass("G.#203自動キャリブレーション", fmt.Sprintf("version=%d 技術志向 weight=%.2f", auto.Version, techW))
			}
		}
	}

	var beforeMatch models.UserCompanyMatch
	_ = db.Where("user_id = ? AND company_id = ?", user.ID, company.ID).First(&beforeMatch).Error
	_ = db.Model(&models.ScoreCalibrationWeight{}).Where("is_active = ?", true).Update("is_active", false).Error
	seedW := models.ScoreCalibrationWeight{
		Category: "技術志向", Weight: 3.0, IsActive: true, Version: 9999,
	}
	if err := db.Create(&seedW).Error; err != nil {
		r.fail("G.#203マッチング適用", "seed weight: "+err.Error())
	} else if err := matchingSvc.CalculateMatching(context.Background(), user.ID, session); err != nil {
		r.fail("G.#203マッチング適用", err.Error())
	} else {
		var afterMatch models.UserCompanyMatch
		_ = db.Where("user_id = ? AND company_id = ?", user.ID, company.ID).First(&afterMatch).Error
		if afterMatch.MatchScore == beforeMatch.MatchScore {
			r.fail("G.#203マッチング適用", fmt.Sprintf("weight=3.0 でも MatchScore 不変 %.2f", beforeMatch.MatchScore))
		} else {
			r.pass("G.#203マッチング適用", fmt.Sprintf("MatchScore %.2f→%.2f (技術志向 weight=3.0)", beforeMatch.MatchScore, afterMatch.MatchScore))
		}
	}

	// H. 不同意ユーザーは集合知に書かない
	// gorm default:true の bool は Create 時に false が落ちることがあるため、明示 Update する。
	no := models.User{
		OrganizationID: 1, Email: "flywheel-extra-noconsent@example.com", Name: "NoConsent",
		Role: "student", AllowCollectiveInsight: true,
	}
	_ = db.Create(&no).Error
	if err := db.Model(&no).Update("allow_collective_insight", false).Error; err != nil {
		return fmt.Errorf("consent update: %w", err)
	}
	var check models.User
	_ = db.First(&check, no.ID).Error
	if check.AllowCollectiveInsight {
		r.fail("H.集合知不同意ガード", "DB上で allow_collective_insight が false にならない")
	} else {
		for _, cat := range valueobject.AllWeightCategories() {
			_ = scoreRepo.SetScore(no.ID, session+"-nc", string(cat), 60)
		}
		beforeLogs := countAllCollective(db, company.ID)
		_ = collective.RecordAction(no.ID, session+"-nc", company.ID, "viewed")
		afterLogs := countAllCollective(db, company.ID)
		if afterLogs != beforeLogs {
			r.fail("H.集合知不同意ガード", fmt.Sprintf("logs %d→%d (consent=%v)", beforeLogs, afterLogs, check.AllowCollectiveInsight))
		} else {
			r.pass("H.集合知不同意ガード", "AllowCollectiveInsight=false は記録しない")
		}
	}

	return nil
}

func seedDecidedSnapshotsForCalibration(db *gorm.DB, companyID uint) error {
	// 技術志向だけ通過側を高くし、他カテゴリは一律。Signal>0 → weight>1 を期待する。
	mkSnap := func(tech int) string {
		m := map[string]int{}
		for _, cat := range valueobject.AllWeightCategories() {
			m[string(cat)] = 50
		}
		m["技術志向"] = tech
		b, _ := json.Marshal(m)
		return string(b)
	}
	now := time.Now()
	for i := 0; i < 10; i++ {
		for _, pass := range []bool{true, false} {
			tech := 30
			status := "rejected"
			if pass {
				tech = 90
				status = "document_passed"
			}
			u := models.User{
				OrganizationID: 1,
				Email:          fmt.Sprintf("flywheel-cal-%t-%d@example.com", pass, i),
				Name:           "CalSeed",
				Role:           "student",
			}
			if err := db.Where("email = ?", u.Email).FirstOrCreate(&u).Error; err != nil {
				return err
			}
			match := models.UserCompanyMatch{
				UserID: u.ID, SessionID: fmt.Sprintf("cal-%t-%d", pass, i),
				CompanyID: companyID, MatchScore: 80, MatchedAxisCount: 10,
			}
			if err := db.Where("user_id = ? AND session_id = ? AND company_id = ?", u.ID, match.SessionID, companyID).
				FirstOrCreate(&match).Error; err != nil {
				return err
			}
			app := models.UserApplicationStatus{
				UserID: u.ID, CompanyID: companyID, MatchID: match.ID,
				Status: status, AppliedAt: &now, ScoreSnapshot: mkSnap(tech),
			}
			var existing models.UserApplicationStatus
			if err := db.Where("user_id = ? AND company_id = ? AND match_id = ?", u.ID, companyID, match.ID).
				First(&existing).Error; err == nil {
				existing.Status = status
				existing.ScoreSnapshot = app.ScoreSnapshot
				if err := db.Save(&existing).Error; err != nil {
					return err
				}
			} else if err := db.Create(&app).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func cleanup(db *gorm.DB) {
	emails := []string{email, "flywheel-extra-orphan@example.com", "flywheel-extra-noconsent@example.com"}
	for i := 0; i < 10; i++ {
		emails = append(emails,
			fmt.Sprintf("flywheel-cal-true-%d@example.com", i),
			fmt.Sprintf("flywheel-cal-false-%d@example.com", i),
		)
	}
	for _, e := range emails {
		var u models.User
		if db.Where("email = ?", e).First(&u).Error == nil {
			db.Where("user_id = ?", u.ID).Delete(&models.UserWeightScore{})
			db.Where("user_id = ?", u.ID).Delete(&models.UserCompanyMatch{})
			db.Where("user_id = ?", u.ID).Delete(&models.UserApplicationStatus{})
			db.Where("user_id = ?", u.ID).Delete(&models.ESReview{})
			db.Delete(&u)
		}
	}
	var companies []models.Company
	db.Unscoped().Where("name = ?", "Flywheel Extra Corp").Find(&companies)
	for _, c := range companies {
		db.Where("company_id = ?", c.ID).Delete(&models.CompanyWeightProfile{})
		db.Where("company_id = ?", c.ID).Delete(&models.CollectiveInsightLog{})
		db.Where("company_id = ?", c.ID).Delete(&models.AnonymizedBehaviorSummary{})
		db.Unscoped().Delete(&c)
	}
	db.Where("version = ?", 9999).Delete(&models.ScoreCalibrationWeight{})
}

func countAction(db *gorm.DB, companyID uint, action string) int64 {
	var n int64
	db.Model(&models.CollectiveInsightLog{}).Where("company_id = ? AND action_type = ?", companyID, action).Count(&n)
	return n
}

func countAllCollective(db *gorm.DB, companyID uint) int64 {
	var n int64
	db.Model(&models.CollectiveInsightLog{}).Where("company_id = ?", companyID).Count(&n)
	return n
}

func scoreOf(s *models.UserWeightScore) int {
	if s == nil {
		return -1
	}
	return s.Score
}

func containsCategory(snapshot, category string) bool {
	return strings.Contains(snapshot, `"`+category+`"`)
}

type check struct {
	name, detail string
	ok           bool
}
type report struct {
	checks []check
	failed int
}

func (r *report) pass(name, detail string) {
	r.checks = append(r.checks, check{name, detail, true})
	log.Printf("PASS  %s — %s", name, detail)
}
func (r *report) fail(name, detail string) {
	r.failed++
	r.checks = append(r.checks, check{name, detail, false})
	log.Printf("FAIL  %s — %s", name, detail)
}
func (r *report) print() {
	fmt.Println("\n=== AI Flywheel Verify EXTRA ===")
	fmt.Printf("time: %s\n", time.Now().Format(time.RFC3339))
	ok := 0
	for _, c := range r.checks {
		m := "FAIL"
		if c.ok {
			m = "PASS"
			ok++
		}
		fmt.Printf("[%s] %s\n       %s\n", m, c.name, c.detail)
	}
	fmt.Printf("\nresult: %d/%d passed\n", ok, len(r.checks))
}
