package skillscore

import (
	"Backend/internal/models"
	"Backend/internal/repositories"
	"math"
)

// languageCategoryMap 言語→スキルカテゴリの分類マスタ
var languageCategoryMap = map[string]models.SkillCategory{
	// Frontend
	"JavaScript":   models.SkillCategoryFrontend,
	"TypeScript":   models.SkillCategoryFrontend,
	"HTML":         models.SkillCategoryFrontend,
	"CSS":          models.SkillCategoryFrontend,
	"SCSS":         models.SkillCategoryFrontend,
	"Sass":         models.SkillCategoryFrontend,
	"Less":         models.SkillCategoryFrontend,
	"Vue":          models.SkillCategoryFrontend,
	"Svelte":       models.SkillCategoryFrontend,
	"Dart":         models.SkillCategoryFrontend,
	"CoffeeScript": models.SkillCategoryFrontend,
	"Elm":          models.SkillCategoryFrontend,

	// Backend
	"Go":      models.SkillCategoryBackend,
	"Python":  models.SkillCategoryBackend,
	"Ruby":    models.SkillCategoryBackend,
	"PHP":     models.SkillCategoryBackend,
	"Java":    models.SkillCategoryBackend,
	"Kotlin":  models.SkillCategoryBackend,
	"C#":      models.SkillCategoryBackend,
	"Rust":    models.SkillCategoryBackend,
	"C++":     models.SkillCategoryBackend,
	"C":       models.SkillCategoryBackend,
	"Scala":   models.SkillCategoryBackend,
	"Swift":   models.SkillCategoryBackend,
	"Elixir":  models.SkillCategoryBackend,
	"Haskell": models.SkillCategoryBackend,
	"Perl":    models.SkillCategoryBackend,
	"Clojure": models.SkillCategoryBackend,
	"Erlang":  models.SkillCategoryBackend,
	"F#":      models.SkillCategoryBackend,
	"Groovy":  models.SkillCategoryBackend,
	"Crystal": models.SkillCategoryBackend,
	"Nim":     models.SkillCategoryBackend,
	"Zig":     models.SkillCategoryBackend,

	// Infrastructure
	"Shell":      models.SkillCategoryInfra,
	"Bash":       models.SkillCategoryInfra,
	"PowerShell": models.SkillCategoryInfra,
	"Dockerfile": models.SkillCategoryInfra,
	"HCL":        models.SkillCategoryInfra,
	"Makefile":   models.SkillCategoryInfra,
	"Nix":        models.SkillCategoryInfra,
	"Puppet":     models.SkillCategoryInfra,
	"Ansible":    models.SkillCategoryInfra,

	// Database
	"SQL":     models.SkillCategoryDB,
	"PLpgSQL": models.SkillCategoryDB,
	"PLSQL":   models.SkillCategoryDB,
	"TSQL":    models.SkillCategoryDB,
}

// ClassifyLanguage 言語名からスキルカテゴリを返す
func ClassifyLanguage(lang string) models.SkillCategory {
	if cat, ok := languageCategoryMap[lang]; ok {
		return cat
	}
	return models.SkillCategoryOther
}

// SkillScoreService スキルスコア算出サービス
type SkillScoreService struct {
	scoreRepo *repositories.SkillScoreRepository
}

func NewSkillScoreService(scoreRepo *repositories.SkillScoreRepository) *SkillScoreService {
	return &SkillScoreService{scoreRepo: scoreRepo}
}

// calculateScores GitHubデータからスキルスコアを算出する（テスト可能な純粋関数）
//
// スコア算出方式:
//   - 言語使用比率 (0-100) × 0.60 → 言語スコア
//   - カテゴリ内リポジトリのstar合計のlog換算 × 5 (上限30) → starボーナス
//   - 年間コントリビューション数のlog換算 × 2 (上限20) → コントリビューションボーナス
//   - 合計を min(100, 合算) でキャップ
func CalculateScores(
	userID uint,
	langStats []models.GitHubLanguageStat,
	repos []models.GitHubRepo,
	totalContributions int,
) []models.SkillScore {
	// カテゴリ別の言語比率合計
	langScores := make(map[models.SkillCategory]float64)
	for _, stat := range langStats {
		cat := ClassifyLanguage(stat.Language)
		langScores[cat] += stat.Percentage
	}

	// カテゴリ別のstar数合計
	starCounts := make(map[models.SkillCategory]int)
	for _, repo := range repos {
		if repo.Language != "" {
			cat := ClassifyLanguage(repo.Language)
			starCounts[cat] += repo.Stars
		}
	}

	// コントリビューションボーナス（全カテゴリ共通）
	contribBonus := math.Min(20, math.Log1p(float64(totalContributions))*2)

	allCategories := []models.SkillCategory{
		models.SkillCategoryFrontend,
		models.SkillCategoryBackend,
		models.SkillCategoryInfra,
		models.SkillCategoryDB,
		models.SkillCategoryOther,
	}

	scores := make([]models.SkillScore, 0, len(allCategories))
	for _, cat := range allCategories {
		langScore := math.Min(100, langScores[cat]) * 0.60
		starBonus := math.Min(30, math.Log1p(float64(starCounts[cat]))*5)
		raw := langScore + starBonus + contribBonus
		score := math.Min(100, raw)

		scores = append(scores, models.SkillScore{
			UserID:   userID,
			Category: cat,
			Score:    math.Round(score*10) / 10,
		})
	}
	return scores
}

// CalculateAndSave GitHubデータからスキルスコアを算出してDBに保存する
func (s *SkillScoreService) CalculateAndSave(
	userID uint,
	langStats []models.GitHubLanguageStat,
	repos []models.GitHubRepo,
	totalContributions int,
) error {
	scores := CalculateScores(userID, langStats, repos, totalContributions)
	return s.scoreRepo.ReplaceScores(scores)
}

// GetScores ユーザーのスキルスコア一覧を取得する
func (s *SkillScoreService) GetScores(userID uint) ([]models.SkillScore, error) {
	return s.scoreRepo.GetScores(userID)
}
