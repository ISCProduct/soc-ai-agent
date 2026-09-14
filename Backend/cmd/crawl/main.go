package main

import (
	"Backend/internal/config"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/company"
	"Backend/internal/services/costs"
	"log"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	aiClient, err := openai.NewFromEnv("")
	if err != nil {
		log.Fatalf("Failed to initialize OpenAI client: %v", err)
	}

	// バッチは無人で走るため、ガード未注入のままだとフォールバックが
	// 一切効かない（fail-closed）。使用量記録とセットで注入する(#1293)。
	apiCallLogRepo := repositories.NewAPICallLogRepository(db)
	apiCostService := costs.NewAPICostService(apiCallLogRepo)
	aiClient.OnUsage = func(u openai.Usage) { apiCostService.LogUsage(u) }
	aiClient.SetFallbackGuard(costs.NewOpenAIFallbackGuard(apiCallLogRepo))

	crawlRepo := repositories.NewCrawlRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	popularityRepo := repositories.NewCompanyPopularityRepository(db)
	service := company.NewCrawlService(crawlRepo, companyRepo, popularityRepo, aiClient)

	service.RunDueSources()
	log.Println("Crawl runner completed")
}
