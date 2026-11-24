package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// researchKeyMaxRunes は interview_research_snapshots.company_key / position の VARCHAR(191) に合わせた上限。
const researchKeyMaxRunes = 191

type InterviewResearchRepository struct {
	db *gorm.DB
}

func NewInterviewResearchRepository(db *gorm.DB) *InterviewResearchRepository {
	return &InterviewResearchRepository{db: db}
}

// truncateResearchKey は列の幅（文字数）を超える分を切り捨てる。保存と検索で同じ値になるよう両方で使う。
func truncateResearchKey(s string) string {
	runes := []rune(s)
	if len(runes) <= researchKeyMaxRunes {
		return s
	}
	return string(runes[:researchKeyMaxRunes])
}

func (r *InterviewResearchRepository) FindFresh(ctx context.Context, companyKey, position string, now time.Time) ([]string, bool, error) {
	var row models.InterviewResearchSnapshot
	err := r.db.WithContext(ctx).
		Where("company_key = ? AND position = ? AND expires_at > ?", truncateResearchKey(companyKey), truncateResearchKey(position), now).
		Order("id DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var questions []string
	if err := json.Unmarshal([]byte(row.QuestionsJSON), &questions); err != nil {
		return nil, false, err
	}
	return questions, true, nil
}

// Save は (company_key, position) の一意キーで原子的に upsert する。
// 並行して同じ企業×職種を保存しても重複キーエラーにならない。
func (r *InterviewResearchRepository) Save(ctx context.Context, companyKey, position string, questions []string, fetchedAt, expiresAt time.Time) error {
	raw, err := json.Marshal(questions)
	if err != nil {
		return err
	}
	row := models.InterviewResearchSnapshot{
		CompanyKey:    truncateResearchKey(companyKey),
		Position:      truncateResearchKey(position),
		QuestionsJSON: string(raw),
		FetchedAt:     fetchedAt,
		ExpiresAt:     expiresAt,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "company_key"}, {Name: "position"}},
		DoUpdates: clause.AssignmentColumns([]string{"questions_json", "fetched_at", "expires_at", "updated_at"}),
	}).Create(&row).Error
}
