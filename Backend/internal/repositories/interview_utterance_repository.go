package repositories

import (
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type InterviewUtteranceRepository struct {
	db *gorm.DB
}

func NewInterviewUtteranceRepository(db *gorm.DB) *InterviewUtteranceRepository {
	return &InterviewUtteranceRepository{db: db}
}

// Create は発話を保存する。同じ (session_id, client_utterance_id) の再送は no-op にする(#1476)。
//
// クライアントは保存失敗を再試行するが、「DBへは書けたが応答だけ失われた」失敗が混ざるため、
// 追記のままだと再試行で同じ発言がもう一件入る。一意制約に当たったら成功として扱う。
// clause.OnConflict{DoNothing:true} は MySQL では INSERT IGNORE になり、
// 一意制約以外のエラー（型不一致・切り詰め等）まで警告へ落として無言で壊れた行を作るため使わない。
func (r *InterviewUtteranceRepository) Create(utterance *models.InterviewUtterance) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "session_id"}, {Name: "client_utterance_id"}},
		DoUpdates: clause.Assignments(map[string]any{"id": gorm.Expr("id")}),
	}).Create(utterance).Error
}

func (r *InterviewUtteranceRepository) FindBySessionID(sessionID uint) ([]models.InterviewUtterance, error) {
	var utterances []models.InterviewUtterance
	// サーバーは1ターンの user と ai を続けて保存するので、created_at が同じミリ秒になりうる。
	// 同値の順序は SQL では決まらないため、id で並びを固定する（#1969）。
	err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC, id ASC").Find(&utterances).Error
	return utterances, err
}
