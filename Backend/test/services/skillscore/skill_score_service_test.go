package skillscore_test

import (
	"math"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/skillscore"

	"github.com/stretchr/testify/assert"
)

// --- ClassifyLanguage ---

func TestClassifyLanguage_Frontend(t *testing.T) {
	langs := []string{"JavaScript", "TypeScript", "HTML", "CSS", "Vue", "Svelte", "Dart"}
	for _, lang := range langs {
		assert.Equal(t, models.SkillCategoryFrontend, skillscore.ClassifyLanguage(lang), "expected Frontend for %s", lang)
	}
}

func TestClassifyLanguage_Backend(t *testing.T) {
	langs := []string{"Go", "Python", "Ruby", "PHP", "Java", "Kotlin", "Rust", "C#"}
	for _, lang := range langs {
		assert.Equal(t, models.SkillCategoryBackend, skillscore.ClassifyLanguage(lang), "expected Backend for %s", lang)
	}
}

func TestClassifyLanguage_Infra(t *testing.T) {
	langs := []string{"Shell", "Dockerfile", "HCL", "Makefile"}
	for _, lang := range langs {
		assert.Equal(t, models.SkillCategoryInfra, skillscore.ClassifyLanguage(lang), "expected Infra for %s", lang)
	}
}

func TestClassifyLanguage_DB(t *testing.T) {
	langs := []string{"SQL", "PLpgSQL", "PLSQL", "TSQL"}
	for _, lang := range langs {
		assert.Equal(t, models.SkillCategoryDB, skillscore.ClassifyLanguage(lang), "expected DB for %s", lang)
	}
}

func TestClassifyLanguage_Unknown(t *testing.T) {
	assert.Equal(t, models.SkillCategoryOther, skillscore.ClassifyLanguage("UnknownLang"))
	assert.Equal(t, models.SkillCategoryOther, skillscore.ClassifyLanguage(""))
	assert.Equal(t, models.SkillCategoryOther, skillscore.ClassifyLanguage("COBOL"))
}

// --- CalculateScores ---

func TestCalculateScores_Empty(t *testing.T) {
	scores := skillscore.CalculateScores(1, nil, nil, 0)
	assert.Len(t, scores, 5, "should return scores for all 5 categories")
	for _, s := range scores {
		assert.Equal(t, float64(0), s.Score)
	}
}

func TestCalculateScores_FrontendOnly(t *testing.T) {
	langStats := []models.GitHubLanguageStat{
		{UserID: 1, Language: "TypeScript", Percentage: 100},
	}
	repos := []models.GitHubRepo{
		{Language: "TypeScript", Stars: 10},
	}
	scores := skillscore.CalculateScores(1, langStats, repos, 0)

	var fe *models.SkillScore
	for i := range scores {
		if scores[i].Category == models.SkillCategoryFrontend {
			fe = &scores[i]
		}
	}
	assert.NotNil(t, fe)
	// langScore = min(100, 100) * 0.60 = 60
	// starBonus = min(30, log1p(10)*5) ≈ min(30, 11.99) ≈ 11.99
	// contribBonus = 0
	assert.Greater(t, fe.Score, 60.0)
	assert.LessOrEqual(t, fe.Score, 100.0)
}

func TestCalculateScores_MultipleCategories(t *testing.T) {
	langStats := []models.GitHubLanguageStat{
		{UserID: 1, Language: "Go", Percentage: 60},
		{UserID: 1, Language: "TypeScript", Percentage: 40},
	}
	repos := []models.GitHubRepo{
		{Language: "Go", Stars: 5},
		{Language: "TypeScript", Stars: 3},
	}
	scores := skillscore.CalculateScores(1, langStats, repos, 100)

	scoreMap := make(map[models.SkillCategory]float64)
	for _, s := range scores {
		scoreMap[s.Category] = s.Score
	}

	// Backendスコアが最高のはず（Go 60%）
	assert.Greater(t, scoreMap[models.SkillCategoryBackend], scoreMap[models.SkillCategoryDB])
	assert.Greater(t, scoreMap[models.SkillCategoryBackend], scoreMap[models.SkillCategoryInfra])
}

func TestCalculateScores_ContributionBonus(t *testing.T) {
	// contributions=0 vs contributions=500 でスコアが上がることを確認
	scores0 := skillscore.CalculateScores(1, nil, nil, 0)
	scores500 := skillscore.CalculateScores(1, nil, nil, 500)

	for i := range scores0 {
		assert.Greater(t, scores500[i].Score, scores0[i].Score,
			"higher contributions should increase score for category %s", scores0[i].Category)
	}
}

func TestCalculateScores_ScoreCappedAt100(t *testing.T) {
	// 言語比率100% + star多数 + contribution多数 → 100を超えないこと
	langStats := []models.GitHubLanguageStat{
		{UserID: 1, Language: "Go", Percentage: 100},
	}
	repos := make([]models.GitHubRepo, 50)
	for i := range repos {
		repos[i] = models.GitHubRepo{Language: "Go", Stars: 1000}
	}
	scores := skillscore.CalculateScores(1, langStats, repos, 10000)

	for _, s := range scores {
		assert.LessOrEqual(t, s.Score, 100.0, "score should not exceed 100 for category %s", s.Category)
	}
}

func TestCalculateScores_RoundingToOneDecimal(t *testing.T) {
	langStats := []models.GitHubLanguageStat{
		{UserID: 1, Language: "Go", Percentage: 33.333},
	}
	scores := skillscore.CalculateScores(1, langStats, nil, 0)

	for _, s := range scores {
		// 小数第1位に丸められていること
		rounded := math.Round(s.Score*10) / 10
		assert.Equal(t, rounded, s.Score, "score should be rounded to 1 decimal place")
	}
}

func TestCalculateScores_UserIDPropagated(t *testing.T) {
	scores := skillscore.CalculateScores(42, nil, nil, 0)
	for _, s := range scores {
		assert.Equal(t, uint(42), s.UserID)
	}
}
