package models

import "time"

// GitHubProfile GitHubアカウント連携情報
type GitHubProfile struct {
	ID                 uint   `gorm:"primaryKey"`
	UserID             uint   `gorm:"uniqueIndex;not null"`
	GitHubLogin        string `gorm:"size:255;not null"`
	AccessToken        string `gorm:"size:500;not null"`
	TotalContributions int    // 年間コントリビューション数
	PublicRepos        int    // 公開リポジトリ数
	Followers          int
	Following          int
	SyncedAt           *time.Time // 最終同期日時（レート制限キャッシュ用）
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// GitHubRepo GitHubリポジトリ情報
type GitHubRepo struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"index;not null"`
	Name            string `gorm:"size:255;not null"`
	FullName        string `gorm:"size:500;not null"`
	Description     string `gorm:"type:text"`
	Language        string `gorm:"size:100"` // メイン言語
	Stars           int
	Forks           int
	IsForked        bool
	GitHubUpdatedAt time.Time // GitHubのupdated_at
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GitHubLanguageStat 言語使用比率
type GitHubLanguageStat struct {
	ID         uint    `gorm:"primaryKey"`
	UserID     uint    `gorm:"index;not null"`
	Language   string  `gorm:"size:100;not null"`
	Bytes      int64   // バイト数
	Percentage float64 // 使用比率（%）
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// GitHubRepoSummary リポジトリのAI要約キャッシュ
type GitHubRepoSummary struct {
	ID          uint   `gorm:"primaryKey"`
	UserID      uint   `gorm:"uniqueIndex:idx_user_repo;not null"`
	FullName    string `gorm:"size:500;uniqueIndex:idx_user_repo;not null"` // "owner/repo"
	SummaryText string `gorm:"type:text"`                                   // 3行の総合要約
	TechReason  string `gorm:"type:text"`                                   // 技術選定の理由
	Challenge   string `gorm:"type:text"`                                   // 解決した課題
	Achievement string `gorm:"type:text"`                                   // 成果
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
