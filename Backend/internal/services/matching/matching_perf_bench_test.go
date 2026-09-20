package matching

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"Backend/domain/entity"
	"Backend/internal/models"
)

// 本番想定の企業行を作る。companies は1行あたり約2KB（実測: 762行で1.5MB）。
func benchCompanies(n int) ([]models.Company, map[uint]*models.CompanyWeightProfile) {
	companies := make([]models.Company, n)
	profiles := make(map[uint]*models.CompanyWeightProfile, n)
	desc := strings.Repeat("事業内容の説明文。", 40)   // 約400字
	culture := strings.Repeat("企業文化の説明。", 20) // 約160字
	welfare := strings.Repeat("福利厚生の説明。", 20)
	for i := range n {
		id := uint(i + 1)
		companies[i] = models.Company{
			ID:             id,
			Name:           fmt.Sprintf("株式会社サンプル%d", id),
			Industry:       "情報通信業",
			Location:       "東京都",
			Description:    desc,
			MainBusiness:   desc,
			Culture:        culture,
			WelfareDetails: welfare,
			WorkStyle:      "ハイブリッド",
			TechStack:      `["Go","TypeScript","React","MySQL"]`,
			DataStatus:     "published",
		}
		profiles[id] = &models.CompanyWeightProfile{
			CompanyID:             id,
			TechnicalOrientation:  50 + i%50,
			TeamworkOrientation:   40 + i%40,
			LeadershipOrientation: 30 + i%50,
			CreativityOrientation: 45 + i%30,
			StabilityOrientation:  55 + i%30,
			GrowthOrientation:     60 + i%30,
			WorkLifeBalance:       50 + i%40,
			ChallengeSeeking:      40 + i%50,
			DetailOrientation:     45 + i%40,
			CommunicationSkill:    50 + i%40,
		}
	}
	return companies, profiles
}

func benchUserScores() []entity.UserWeightScore {
	cats := []string{
		"technical_orientation", "teamwork_orientation", "leadership_orientation",
		"creativity_orientation", "stability_orientation", "growth_orientation",
		"work_life_balance", "challenge_seeking", "detail_orientation", "communication_skill",
	}
	out := make([]entity.UserWeightScore, len(cats))
	for i, c := range cats {
		out[i] = entity.UserWeightScore{WeightCategory: c, Score: 50 + i*3}
	}
	return out
}

// BenchmarkCalculateMatching は診断完了時に走るマッチング計算の実測。
// 本番想定の公開企業数で回し、1回あたりの時間と確保メモリを見る。
func BenchmarkCalculateMatching(b *testing.B) {
	for _, n := range []int{90, 842, 2500, 4000} {
		b.Run(fmt.Sprintf("companies=%d", n), func(b *testing.B) {
			companies, profiles := benchCompanies(n)
			svc := NewMatchingService(
				&matchingScoreRepo{scores: benchUserScores()},
				&matchingCompanyRepo{companies: companies, profiles: profiles},
				&matchingMatchRepo{},
				nil,
			)
			// CalculateMatching は1回あたり5行ログを出す。計測の邪魔になるので捨てる。
			log.SetOutput(io.Discard)
			b.Cleanup(func() { log.SetOutput(os.Stderr) })
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := svc.CalculateMatching(context.Background(), 1, "s1"); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkBuildMatchReason は全企業ぶん生成している理由文の単価。
// 表示されるのは上位10件だけだが、現状は全件について生成し DB へ保存している。
func BenchmarkBuildMatchReason(b *testing.B) {
	scores := benchUserScores()
	companies, _ := benchCompanies(1)
	comp := &companies[0]
	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		m := &entity.UserCompanyMatch{
			CompanyID:      1,
			Company:        &entity.Company{ID: 1, Name: comp.Name, Industry: comp.Industry, MainBusiness: comp.MainBusiness, Culture: comp.Culture, WorkStyle: comp.WorkStyle, TechStack: comp.TechStack},
			MatchScore:     float64(i % 100),
			TechnicalMatch: 80, TeamworkMatch: 70, GrowthMatch: 60,
		}
		if s := BuildMatchReason(m, scores); s == "" {
			b.Fatal("empty")
		}
	}
}
