package models

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

// SeedLargeCompanyData 大規模企業データのシード（40,000件）+ 企業関係データ
func SeedLargeCompanyData(db *gorm.DB) error {
	// 既存のデータ件数を確認
	var count int64
	db.Model(&Company{}).Count(&count)

	if count >= 40000 {
		// すでに4万社以上ある場合はスキップ
		slog.Info("Company data already exists, skipping large seed", "records", count)
		return nil
	}

	slog.Info("Starting to seed companies with relationships", "target_companies", 40000)
	startTime := time.Now()

	// ランダムシードを初期化
	rand.Seed(time.Now().UnixNano())

	// 業界リスト
	industries := []string{
		"IT・ソフトウェア", "製造業", "金融", "商社", "小売",
		"医療・ヘルスケア", "教育", "建設・不動産", "運輸・物流", "エネルギー",
		"通信", "メディア・広告", "コンサルティング", "人材サービス", "飲食",
	}

	// 市場区分リスト
	marketTypes := []string{"prime", "standard", "growth", "unlisted"}
	marketWeights := []int{10, 20, 15, 55} // 非上場が多い

	// 技術スタックのパターン
	techStacks := []string{
		`["Go", "React", "TypeScript", "AWS", "Docker"]`,
		`["Java", "Spring", "Oracle", "Kubernetes"]`,
		`["Python", "Django", "PostgreSQL", "GCP"]`,
		`["PHP", "Laravel", "MySQL", "Vue.js"]`,
		`["Node.js", "Express", "MongoDB", "React"]`,
		`["C#", ".NET", "Azure", "SQL Server"]`,
		`["Ruby", "Rails", "PostgreSQL", "AWS"]`,
		`["Scala", "Play", "Cassandra", "Spark"]`,
	}

	// ワークスタイル
	workStyles := []string{"フルリモート", "ハイブリッド（週2-3日出社）", "オフィス勤務", "フレックス"}

	// 開発スタイル
	devStyles := []string{"アジャイル", "ウォーターフォール", "スクラム", "リーン開発"}

	// 都道府県リスト
	locations := []string{
		"東京都", "大阪府", "神奈川県", "愛知県", "福岡県",
		"北海道", "宮城県", "埼玉県", "千葉県", "京都府",
		"兵庫県", "広島県", "静岡県", "茨城県", "新潟県",
	}

	// バッチサイズ
	batchSize := 1000
	totalCompanies := 40000

	for i := 0; i < totalCompanies; i += batchSize {
		companies := make([]Company, 0, batchSize)

		end := i + batchSize
		if end > totalCompanies {
			end = totalCompanies
		}

		for j := i; j < end; j++ {
			industry := industries[rand.Intn(len(industries))]
			location := locations[rand.Intn(len(locations))]

			// 市場区分を重み付きで選択
			marketType := selectWeighted(marketTypes, marketWeights)
			_ = marketType // 市場情報は別テーブルで管理

			// 従業員数（10-10000人の範囲）
			employeeCount := 10 + rand.Intn(9990)

			// 設立年（1950-2023）
			foundedYear := 1950 + rand.Intn(74)

			// 平均年齢（25-55歳）
			avgAge := 25.0 + rand.Float64()*30.0

			// 女性比率（10-50%）
			femaleRatio := 10.0 + rand.Float64()*40.0

			company := Company{
				Name:             fmt.Sprintf("%s%s第%d号株式会社", location[:len(location)-3], industry, j+1),
				Description:      generateDescription(industry, employeeCount),
				Industry:         industry,
				EmployeeCount:    employeeCount,
				FoundedYear:      foundedYear,
				Location:         location,
				WebsiteURL:       fmt.Sprintf("https://company-%d.example.com", j+1),
				Culture:          generateCulture(employeeCount),
				WorkStyle:        workStyles[rand.Intn(len(workStyles))],
				TechStack:        techStacks[rand.Intn(len(techStacks))],
				DevelopmentStyle: devStyles[rand.Intn(len(devStyles))],
				MainBusiness:     generateBusiness(industry),
				AverageAge:       avgAge,
				FemaleRatio:      femaleRatio,
				IsActive:         true,
				IsVerified:       rand.Float32() < 0.7, // 70%が認証済み
			}

			companies = append(companies, company)
		}

		// バッチ挿入
		if err := db.CreateInBatches(companies, batchSize).Error; err != nil {
			return fmt.Errorf("failed to insert companies batch %d-%d: %w", i, end, err)
		}

		if (i/batchSize)%10 == 0 {
			slog.Info("Inserted companies", "inserted", end, "total", totalCompanies)
		}
	}

	slog.Info("Successfully inserted companies", "count", totalCompanies, "duration", time.Since(startTime))
	return nil
}

// SeedLargeCompanyMarketInfo 大規模市場情報のシード
func SeedLargeCompanyMarketInfo(db *gorm.DB) error {
	var count int64
	db.Model(&CompanyMarketInfo{}).Count(&count)

	if count >= 100 {
		slog.Info("Market info already exists, skipping", "records", count)
		return nil
	}

	slog.Info("Seeding market info for companies")
	startTime := time.Now()

	marketTypes := []string{"prime", "standard", "growth", "unlisted"}
	marketWeights := []int{10, 20, 15, 55}

	// 全企業を取得（IDのみ）
	var companyIDs []uint
	db.Model(&Company{}).Pluck("id", &companyIDs)

	batchSize := 1000
	for i := 0; i < len(companyIDs); i += batchSize {
		marketInfos := make([]CompanyMarketInfo, 0, batchSize)

		end := i + batchSize
		if end > len(companyIDs) {
			end = len(companyIDs)
		}

		for j := i; j < end; j++ {
			companyID := companyIDs[j]
			marketType := selectWeighted(marketTypes, marketWeights)
			isListed := marketType != "unlisted"

			info := CompanyMarketInfo{
				CompanyID:  companyID,
				MarketType: marketType,
				IsListed:   isListed,
			}

			if isListed {
				// 証券コード（1000-9999）
				info.StockCode = fmt.Sprintf("%04d", 1000+rand.Intn(8999))

				// 時価総額（100億円〜10兆円）
				marketCap := 10000.0 + rand.Float64()*9990000.0 // 単位: 百万円
				info.MarketCap = &marketCap

				// 上場日（2000年〜2023年）
				year := 2000 + rand.Intn(24)
				month := 1 + rand.Intn(12)
				day := 1 + rand.Intn(28)
				listingDate := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
				info.ListingDate = &listingDate
			}

			marketInfos = append(marketInfos, info)
		}

		if err := db.CreateInBatches(marketInfos, batchSize).Error; err != nil {
			return fmt.Errorf("failed to insert market info batch: %w", err)
		}

		if (i/batchSize)%10 == 0 {
			slog.Info("Inserted market info", "inserted", end, "total", len(companyIDs))
		}
	}

	slog.Info("Successfully inserted market info", "duration", time.Since(startTime))
	return nil
}

// SeedLargeCompanyRelations 大規模企業関係のシード（資本関係+多様なビジネス関係）
func SeedLargeCompanyRelations(db *gorm.DB) error {
	var count int64
	db.Model(&CompanyRelation{}).Count(&count)

	if count >= 50000 {
		slog.Info("Relations already exist, skipping", "records", count)
		return nil
	}

	slog.Info("Seeding company relations with enhanced group structures")
	startTime := time.Now()

	// 全企業IDを取得
	var companyIDs []uint
	db.Model(&Company{}).Pluck("id", &companyIDs)

	if len(companyIDs) == 0 {
		return fmt.Errorf("no companies found")
	}

	slog.Info("Generating relations", "company_count", len(companyIDs))

	// === 企業グループの生成 ===
	// 約200のグループを作成（各グループ5-20社）
	numGroups := 200
	avgGroupSize := 10

	// 資本関係: 企業グループ構造（約2,000-3,000件）
	numCapitalRelations := numGroups * avgGroupSize

	// ビジネス関係:
	// - グループ内取引: 約5,000件
	// - グループ間取引: 約20,000件
	// - ランダム取引: 約25,000件
	numBusinessRelations := 50000

	relations := make([]CompanyRelation, 0, numCapitalRelations+numBusinessRelations)

	// === 企業グループの資本関係生成 ===
	slog.Info("Generating corporate groups with capital relations")

	usedCompanies := make(map[uint]bool)
	groups := make([][]uint, 0, numGroups)

	for g := 0; g < numGroups && len(usedCompanies) < len(companyIDs)-100; g++ {
		// グループサイズ: 5-20社
		groupSize := 5 + rand.Intn(16)
		group := make([]uint, 0, groupSize)

		// 親会社を選択
		var parentID uint
		for {
			idx := rand.Intn(len(companyIDs))
			parentID = companyIDs[idx]
			if !usedCompanies[parentID] {
				usedCompanies[parentID] = true
				group = append(group, parentID)
				break
			}
		}

		// 子会社を追加
		for i := 1; i < groupSize; i++ {
			var childID uint
			for {
				idx := rand.Intn(len(companyIDs))
				childID = companyIDs[idx]
				if !usedCompanies[childID] {
					usedCompanies[childID] = true
					group = append(group, childID)
					break
				}
			}

			// 親会社との資本関係を作成
			ratio := 50.0 + rand.Float64()*50.0 // 50-100%
			relationType := "capital_subsidiary"
			desc := "子会社"
			if ratio >= 100 {
				ratio = 100.0
				desc = "完全子会社"
			}

			relations = append(relations, CompanyRelation{
				ParentID:     &parentID,
				ChildID:      &childID,
				RelationType: relationType,
				Ratio:        &ratio,
				Description:  fmt.Sprintf("%s（出資比率%.1f%%）", desc, ratio),
				IsActive:     true,
			})
		}

		groups = append(groups, group)

		if g%20 == 0 {
			slog.Info("Generated corporate groups", "count", g)
		}
	}

	slog.Info("Generated corporate groups and capital relations", "groups", len(groups), "capital_relations", len(relations))

	// === ビジネス関係の生成（多様な関係タイプ） ===
	slog.Info("Generating business relations")

	businessRelationTypes := []struct {
		Type        string
		Description string
		Weight      int
	}{
		{"business_service_provider", "【技術サービス】システム開発・運用保守を提供", 25},
		{"business_service_provider", "【コンサルティング】経営・IT戦略支援を提供", 15},
		{"business_partner", "【業務提携】共同開発プロジェクトの実施", 20},
		{"business_partner", "【技術協力】技術ノウハウの相互提供", 15},
		{"business_supplier", "【製品供給】ハードウェア・ソフトウェアの供給", 10},
		{"business_supplier", "【物流サービス】配送・在庫管理サービス提供", 5},
		{"business_internal", "【グループ内】経営管理・人事サービス提供", 3},
		{"business_investment", "【投資】新規事業への資金・リソース提供", 2},
		{"business_outsource", "【業務委託】開発・運用業務の委託", 5},
	}

	totalWeight := 0
	for _, brt := range businessRelationTypes {
		totalWeight += brt.Weight
	}

	businessRelationsCount := 0

	// 1. グループ内のビジネス関係（各グループ内で5-10件）
	slog.Info("Generating intra-group business relations")
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}

		// グループ内で3-8件のビジネス関係を作成
		numIntraRelations := 3 + rand.Intn(6)
		for i := 0; i < numIntraRelations && i < len(group)-1; i++ {
			fromIdx := rand.Intn(len(group))
			toIdx := rand.Intn(len(group))

			if fromIdx == toIdx {
				continue
			}

			fromID := group[fromIdx]
			toID := group[toIdx]

			// グループ内取引は主に経営管理系
			relation := CompanyRelation{
				FromID:       &fromID,
				ToID:         &toID,
				RelationType: "business_internal",
				Description:  "【グループ内】経営管理・人事サービス提供",
				IsActive:     true,
			}

			relations = append(relations, relation)
			businessRelationsCount++
		}
	}

	slog.Info("Generated intra-group business relations", "count", businessRelationsCount)

	// 2. グループ間のビジネス関係（各グループが5-15社と取引）
	slog.Info("Generating inter-group business relations")
	interGroupCount := 0
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}

		// グループ代表企業（親会社）
		groupRep := group[0]

		// 他のグループと5-15件の取引関係
		numInterRelations := 5 + rand.Intn(11)
		for i := 0; i < numInterRelations && i < len(groups); i++ {
			// ランダムに他のグループを選択
			otherGroupIdx := rand.Intn(len(groups))
			if len(groups[otherGroupIdx]) == 0 {
				continue
			}

			otherGroupRep := groups[otherGroupIdx][0]
			if groupRep == otherGroupRep {
				continue
			}

			// 重み付きでビジネス関係タイプを選択
			randWeight := rand.Intn(totalWeight)
			cumWeight := 0
			var selectedType = businessRelationTypes[0]

			for _, brt := range businessRelationTypes {
				cumWeight += brt.Weight
				if randWeight < cumWeight {
					selectedType = brt
					break
				}
			}

			relation := CompanyRelation{
				FromID:       &groupRep,
				ToID:         &otherGroupRep,
				RelationType: selectedType.Type,
				Description:  selectedType.Description,
				IsActive:     true,
			}

			relations = append(relations, relation)
			interGroupCount++
		}
	}

	slog.Info("Generated inter-group business relations", "count", interGroupCount)
	businessRelationsCount += interGroupCount

	// 3. ランダムなビジネス関係（エコシステム）
	slog.Info("Generating random business ecosystem")
	randomCount := 0
	targetRandomRelations := 25000

	usedBusinessPairs := make(map[string]bool)

	for randomCount < targetRandomRelations {
		fromIdx := rand.Intn(len(companyIDs))
		toIdx := rand.Intn(len(companyIDs))

		if fromIdx == toIdx {
			continue
		}

		fromID := companyIDs[fromIdx]
		toID := companyIDs[toIdx]

		pairKey := fmt.Sprintf("biz_%d_%d", fromID, toID)
		if usedBusinessPairs[pairKey] {
			continue
		}
		usedBusinessPairs[pairKey] = true

		// 重み付きでビジネス関係タイプを選択
		randWeight := rand.Intn(totalWeight)
		cumWeight := 0
		var selectedType = businessRelationTypes[0]

		for _, brt := range businessRelationTypes {
			cumWeight += brt.Weight
			if randWeight < cumWeight {
				selectedType = brt
				break
			}
		}

		relation := CompanyRelation{
			FromID:       &fromID,
			ToID:         &toID,
			RelationType: selectedType.Type,
			Description:  selectedType.Description,
			IsActive:     true,
		}

		relations = append(relations, relation)
		randomCount++

		if randomCount%5000 == 0 {
			slog.Info("Generated random business relations", "count", randomCount)
		}
	}

	businessRelationsCount += randomCount
	slog.Info("Generated total business relations", "count", businessRelationsCount)
	slog.Info("Relations generation summary",
		"total_relations", len(relations),
		"capital_relations", len(relations)-businessRelationsCount,
		"business_relations", businessRelationsCount,
		"corporate_groups", len(groups),
	)

	// バッチ挿入
	batchSize := 1000
	for i := 0; i < len(relations); i += batchSize {
		end := i + batchSize
		if end > len(relations) {
			end = len(relations)
		}

		if err := db.CreateInBatches(relations[i:end], batchSize).Error; err != nil {
			return fmt.Errorf("failed to insert relations batch: %w", err)
		}

		if (i/batchSize)%5 == 0 {
			slog.Info("Inserted relations", "inserted", end, "total", len(relations))
		}
	}

	slog.Info("Successfully inserted relations", "count", len(relations), "duration", time.Since(startTime))
	return nil
}

// SeedLargeCompanyProfiles 大規模企業プロファイルのシード
func SeedLargeCompanyProfiles(db *gorm.DB) error {
	var count int64
	db.Model(&CompanyWeightProfile{}).Count(&count)

	if count >= 40000 {
		slog.Info("Profiles already exist, skipping", "records", count)
		return nil
	}

	slog.Info("Seeding company weight profiles for all companies")
	startTime := time.Now()

	// 全企業IDを取得
	var companyIDs []uint
	db.Model(&Company{}).Pluck("id", &companyIDs)

	if len(companyIDs) == 0 {
		return fmt.Errorf("no companies found")
	}

	batchSize := 1000
	for i := 0; i < len(companyIDs); i += batchSize {
		profiles := make([]CompanyWeightProfile, 0, batchSize)

		end := i + batchSize
		if end > len(companyIDs) {
			end = len(companyIDs)
		}

		for j := i; j < end; j++ {
			profile := CompanyWeightProfile{
				CompanyID: companyIDs[j],
				// 各指標を30-100の範囲でランダム生成（よりリアルな分布）
				TechnicalOrientation:  30 + rand.Intn(71),
				TeamworkOrientation:   40 + rand.Intn(61),
				LeadershipOrientation: 30 + rand.Intn(71),
				CreativityOrientation: 30 + rand.Intn(71),
				StabilityOrientation:  30 + rand.Intn(71),
				GrowthOrientation:     30 + rand.Intn(71),
				WorkLifeBalance:       40 + rand.Intn(61),
				ChallengeSeeking:      30 + rand.Intn(71),
				DetailOrientation:     40 + rand.Intn(61),
				CommunicationSkill:    40 + rand.Intn(61),
			}

			profiles = append(profiles, profile)
		}

		if err := db.CreateInBatches(profiles, batchSize).Error; err != nil {
			return fmt.Errorf("failed to insert profiles batch: %w", err)
		}

		if (i/batchSize)%10 == 0 {
			slog.Info("Inserted company profiles", "inserted", end, "total", len(companyIDs))
		}
	}

	slog.Info("Successfully inserted company profiles", "count", len(companyIDs), "duration", time.Since(startTime))
	return nil
}

// ヘルパー関数
func selectWeighted(options []string, weights []int) string {
	total := 0
	for _, w := range weights {
		total += w
	}

	r := rand.Intn(total)
	cumulative := 0

	for i, w := range weights {
		cumulative += w
		if r < cumulative {
			return options[i]
		}
	}

	return options[0]
}

func generateDescription(industry string, employeeCount int) string {
	templates := map[string][]string{
		"IT・ソフトウェア": {
			"最先端技術で社会課題を解決する企業",
			"クラウドサービスを提供するテクノロジー企業",
			"エンタープライズ向けソリューションを提供",
		},
		"製造業": {
			"高品質な製品を世界に提供する製造企業",
			"革新的な技術で製造業の未来を創造",
			"環境に配慮したものづくりを追求",
		},
		"金融": {
			"お客様の資産形成をサポートする金融機関",
			"革新的な金融サービスを提供",
			"地域に根ざした金融サービス",
		},
	}

	descriptions, ok := templates[industry]
	if !ok {
		descriptions = []string{
			fmt.Sprintf("%sの分野で事業を展開する企業", industry),
			fmt.Sprintf("従業員%d名規模の成長企業", employeeCount),
		}
	}

	return descriptions[rand.Intn(len(descriptions))]
}

func generateCulture(employeeCount int) string {
	if employeeCount < 50 {
		return "少数精鋭のスタートアップ文化。フラットな組織で自由な発想を尊重。"
	} else if employeeCount < 300 {
		return "成長フェーズの企業文化。チャレンジを推奨し、失敗を恐れない環境。"
	} else {
		return "安定した大企業の文化。充実した研修制度とキャリアパス。"
	}
}

func generateBusiness(industry string) string {
	businessTemplates := map[string][]string{
		"IT・ソフトウェア": {
			"Webアプリケーション開発", "システムインテグレーション",
			"SaaSプラットフォーム提供", "AIソリューション開発",
		},
		"製造業": {
			"自動車部品製造", "電子部品製造", "精密機器製造", "産業機械製造",
		},
		"金融": {
			"投資銀行業務", "資産運用サービス", "法人向け融資", "決済サービス",
		},
	}

	businesses, ok := businessTemplates[industry]
	if !ok {
		businesses = []string{fmt.Sprintf("%s関連事業", industry)}
	}

	return businesses[rand.Intn(len(businesses))]
}

func getCapitalDescription(relationType string) string {
	if relationType == "capital_subsidiary" {
		return "子会社"
	}
	return "関連会社"
}
