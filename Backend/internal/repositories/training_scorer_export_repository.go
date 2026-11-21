package repositories

import (
	"context"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// TrainingSubject は採点モデルの学習データの書き出し（#2168）の対象の学生。
// 氏名・メール・学校名はマスキングの辞書を作るためだけに取り出す。出力には書かない。
type TrainingSubject struct {
	UserID     uint
	Name       string
	Email      string
	SchoolName string
}

// TrainingSession は書き出す面接1回ぶん。ScoresJSON は LLM の採点（interview_reports.scores_json）。
type TrainingSession struct {
	SessionID       uint
	CompanyName     string
	TemplateVersion string
	ScoresJSON      string
}

// TrainingUtterance は面接の発話1つ（AI の質問と学生の回答の両方）。
type TrainingUtterance struct {
	SessionID uint
	Role      string
	Text      string
}

// TrainingMaskingTerms はマスキングの辞書に入れる、DB にある既知の名称。
type TrainingMaskingTerms struct {
	Schools   []string
	Companies []string
}

// TrainingScorerExportRepository は採点モデルの学習データの書き出し（#2168）で DB を引く。
// 既存の TrainingExportRepository（選考結果を教師ラベルにする別目的の書き出し）とは分けている。
// あちらは同意（allow_training_use）を見ないため、同じリポジトリに置くと条件の付け忘れが起きやすい。
type TrainingScorerExportRepository struct {
	db *gorm.DB
}

func NewTrainingScorerExportRepository(db *gorm.DB) *TrainingScorerExportRepository {
	return &TrainingScorerExportRepository{db: db}
}

// trainingEligibleUsers は「学習に使ってよい学生」の条件を組み立てる（#2168 追記2）。
// 学習利用に同意済み・未退会・学生ロール・非ゲスト。退会の猶予期間中は論理削除で
// allow_training_use が 1 のまま残るため、フラグだけでは判定しない。
// 残りの2つは企業への公開の判定（visibleStudents）に合わせた。同意の画面は学生にしか出さないため。
// 書き出しのすべての経路（一覧・単体の判定・面接の取得）がこの条件を通る。条件はここ以外に書かないこと。
func (r *TrainingScorerExportRepository) trainingEligibleUsers(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("users AS u").
		Where("u.allow_training_use = ?", true).
		Where("u.withdrawn_at IS NULL").
		Where("u.role = ?", models.UserRoleStudent).
		Where("u.is_guest = ?", false)
}

// IsTrainingEligible は、その学生の面接を学習に使ってよいかを返す。
func (r *TrainingScorerExportRepository) IsTrainingEligible(ctx context.Context, userID uint) (bool, error) {
	var n int64
	err := r.trainingEligibleUsers(ctx).Where("u.id = ?", userID).Count(&n).Error
	return n > 0, err
}

// ListTrainingSubjects は学習に使ってよい学生の一覧を、ID の昇順で返す。
func (r *TrainingScorerExportRepository) ListTrainingSubjects(ctx context.Context) ([]TrainingSubject, error) {
	var rows []TrainingSubject
	err := r.trainingEligibleUsers(ctx).
		Select("u.id AS user_id, COALESCE(u.name, '') AS name, u.email AS email, COALESCE(u.school_name, '') AS school_name").
		Order("u.id").
		Scan(&rows).Error
	return rows, err
}

// ListTrainingSessions は、その学生の完了した面接のうち、LLM の採点があるものを返す。
// 学生の条件は trainingEligibleUsers の副問い合わせで付け直す。一覧を取ったあとに
// 同意を取り消した学生の面接を、ここで取り出さないため。
func (r *TrainingScorerExportRepository) ListTrainingSessions(ctx context.Context, userID uint) ([]TrainingSession, error) {
	var rows []TrainingSession
	err := r.db.WithContext(ctx).Table("interview_sessions AS s").
		Select("s.id AS session_id, COALESCE(s.company_name, '') AS company_name, COALESCE(s.template_version, '') AS template_version, ir.scores_json AS scores_json").
		Joins("JOIN interview_reports AS ir ON ir.session_id = s.id").
		Where("s.user_id = ?", userID).
		Where("s.user_id IN (?)", r.trainingEligibleUsers(ctx).Select("u.id")).
		Where("s.status = ? AND s.deleted_at IS NULL", "finished").
		Where("ir.scores_json IS NOT NULL AND ir.scores_json <> ''").
		Order("s.id").
		Scan(&rows).Error
	return rows, err
}

// ListTrainingUtterances は面接の発話を、話した順に返す。AI の質問も含める（学習の入力は「質問と回答」の組）。
func (r *TrainingScorerExportRepository) ListTrainingUtterances(ctx context.Context, sessionID uint) ([]TrainingUtterance, error) {
	var rows []TrainingUtterance
	err := r.db.WithContext(ctx).Table("interview_utterances").
		Select("session_id, role, text").
		Where("session_id = ?", sessionID).
		Order("created_at, id").
		Scan(&rows).Error
	return rows, err
}

// ListTrainingMaskingTerms は、マスキングの辞書に入れる学校名と企業名を返す。
// 学校名は構造化した schools と、学生が自由記述した users.school_name の両方を使う。
// 面接で話すのは自分の学校とは限らない（併願・編入の話など）ため、全員分を入れる。
// 企業名は companies の名称と読み、面接のときに入力した interview_sessions.company_name を使う。
func (r *TrainingScorerExportRepository) ListTrainingMaskingTerms(ctx context.Context) (TrainingMaskingTerms, error) {
	var out TrainingMaskingTerms
	db := r.db.WithContext(ctx)
	if err := db.Raw(`SELECT name FROM schools WHERE name <> ''
		UNION SELECT DISTINCT school_name FROM users WHERE school_name IS NOT NULL AND school_name <> ''`).
		Scan(&out.Schools).Error; err != nil {
		return TrainingMaskingTerms{}, err
	}
	if err := db.Raw(`SELECT name FROM companies WHERE name <> ''
		UNION SELECT name_reading FROM companies WHERE name_reading IS NOT NULL AND name_reading <> ''
		UNION SELECT DISTINCT company_name FROM interview_sessions WHERE company_name IS NOT NULL AND company_name <> ''`).
		Scan(&out.Companies).Error; err != nil {
		return TrainingMaskingTerms{}, err
	}
	return out, nil
}

// ListTrainingEligibleUserIDs は、いま学習に使ってよい学生の利用者 ID を返す。書き出し済みのデータの削除
// （scorerexport.Sweeper.Sweep）が、この一覧に無い学生のファイルを消すのに使う。条件は trainingEligibleUsers。
func (r *TrainingScorerExportRepository) ListTrainingEligibleUserIDs(ctx context.Context) ([]uint, error) {
	var ids []uint
	err := r.trainingEligibleUsers(ctx).Order("u.id").Pluck("u.id", &ids).Error
	return ids, err
}

// StartTrainingSweepRun は、書き出し済みのデータの削除の監査ログを running で作り、その ID を返す。
func (r *TrainingScorerExportRepository) StartTrainingSweepRun(ctx context.Context, destination string, startedAt time.Time) (uint, error) {
	run := models.TrainingScorerExportRun{
		Kind:        models.TrainingScorerRunKindSweep,
		Status:      models.TrainingScorerExportRunning,
		StartedAt:   startedAt,
		Destination: destination,
	}
	if err := r.db.WithContext(ctx).Create(&run).Error; err != nil {
		return 0, err
	}
	return run.ID, nil
}

// FinishTrainingSweepRun は、削除の監査ログに結果（消したファイルの数）を書く。
func (r *TrainingScorerExportRepository) FinishTrainingSweepRun(ctx context.Context, id uint, status string, deletedFiles int, errMsg string, finishedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.TrainingScorerExportRun{}).
		Where("id = ? AND kind = ?", id, models.TrainingScorerRunKindSweep).
		Updates(map[string]any{
			"status":             status,
			"deleted_file_count": deletedFiles,
			"error_message":      errMsg,
			"finished_at":        finishedAt,
		}).Error
}

// StartTrainingExportRun は監査ログを running で作り、その ID を返す。
func (r *TrainingScorerExportRepository) StartTrainingExportRun(ctx context.Context, destination string, startedAt time.Time) (uint, error) {
	run := models.TrainingScorerExportRun{
		Kind:        models.TrainingScorerRunKindExport,
		Status:      models.TrainingScorerExportRunning,
		StartedAt:   startedAt,
		Destination: destination,
	}
	if err := r.db.WithContext(ctx).Create(&run).Error; err != nil {
		return 0, err
	}
	return run.ID, nil
}

// FinishTrainingExportRun は監査ログに結果を書く。status は succeeded か failed。
func (r *TrainingScorerExportRepository) FinishTrainingExportRun(ctx context.Context, id uint, status string, students, sessions int, errMsg string, finishedAt time.Time) error {
	return r.db.WithContext(ctx).Model(&models.TrainingScorerExportRun{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":        status,
			"student_count": students,
			"session_count": sessions,
			"error_message": errMsg,
			"finished_at":   finishedAt,
		}).Error
}
