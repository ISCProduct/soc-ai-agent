package models

import "gorm.io/gorm"

// demoCompanySubquery はデモ企業IDを求めるサブクエリ。
// DELETE対象と同じcompaniesテーブルを参照するため派生テーブルで包む。
const demoCompanySubquery = `(SELECT id FROM (SELECT id FROM companies WHERE website_url LIKE '%.example.com%' OR website_url = 'https://example.com' OR name IN ('株式会社テックイノベーション', 'エンタープライズシステムズ株式会社', 'クリエイティブラボ株式会社')) AS demo_companies)`

// CleanupDemoCompanies 過去のシードで投入されたデモ企業と関連データを削除する。
// デモ企業が存在しない場合は何もしない（冪等）。
//
// company_relations は両端がデモ企業の行のみ削除する。
// 実在企業とデモ企業の混在関係は行を残し、デモ側 FK を NULL にしてから企業を削除する
// （片端のみデモの関係を丸ごと消すと、実在企業側の関係データが失われるため）。
func CleanupDemoCompanies(db *gorm.DB) error {
	// 外部キー制約に従い、子テーブルから順に削除する
	stmts := []string{
		`DELETE FROM company_weight_profiles WHERE company_id IN ` + demoCompanySubquery +
			` OR job_position_id IN (SELECT id FROM company_job_positions WHERE company_id IN ` + demoCompanySubquery + `)`,
		`DELETE FROM graduate_employments WHERE company_id IN ` + demoCompanySubquery +
			` OR job_position_id IN (SELECT id FROM company_job_positions WHERE company_id IN ` + demoCompanySubquery + `)`,
		`DELETE FROM user_company_matches WHERE company_id IN ` + demoCompanySubquery +
			` OR job_position_id IN (SELECT id FROM company_job_positions WHERE company_id IN ` + demoCompanySubquery + `)`,
		`DELETE FROM company_job_positions WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM company_benefits WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM company_market_info WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM company_popularity_records WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM company_profile_update_histories WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM company_reviews WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM g_biz_company_profiles WHERE company_id IN ` + demoCompanySubquery,
		`DELETE FROM user_application_statuses WHERE company_id IN ` + demoCompanySubquery,
		// 両端ともデモ企業の関係のみ削除（資本: parent+child / ビジネス: from+to）
		`DELETE FROM company_relations WHERE ` +
			`(parent_id IS NOT NULL AND child_id IS NOT NULL AND parent_id IN ` + demoCompanySubquery + ` AND child_id IN ` + demoCompanySubquery + `) OR ` +
			`(from_id IS NOT NULL AND to_id IS NOT NULL AND from_id IN ` + demoCompanySubquery + ` AND to_id IN ` + demoCompanySubquery + `)`,
		// 混在関係はデモ側 FK を外し、実在企業側の行を残す
		`UPDATE company_relations SET parent_id = NULL WHERE parent_id IN ` + demoCompanySubquery,
		`UPDATE company_relations SET child_id = NULL WHERE child_id IN ` + demoCompanySubquery,
		`UPDATE company_relations SET from_id = NULL WHERE from_id IN ` + demoCompanySubquery,
		`UPDATE company_relations SET to_id = NULL WHERE to_id IN ` + demoCompanySubquery,
		`DELETE FROM companies WHERE id IN ` + demoCompanySubquery,
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, stmt := range stmts {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
