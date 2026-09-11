package main

import (
	"Backend/internal/config"
	"Backend/internal/openai"
	"Backend/internal/repositories"
	"Backend/internal/services/company"
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

	crawlRepo := repositories.NewCrawlRepository(db)
	companyRepo := repositories.NewCompanyRepository(db)
	popularityRepo := repositories.NewCompanyPopularityRepository(db)
	service := company.NewCrawlService(crawlRepo, companyRepo, popularityRepo, aiClient)

	service.RunDueSources()
	log.Println("Crawl runner completed")
}
